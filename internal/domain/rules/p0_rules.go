package rules

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"syskit/internal/domain/model"
)

const (
	phaseP0 = "P0"
)

var defaultCriticalPorts = []int{80, 443, 8080, 3306, 6379}

const (
	defaultCPUPercent          = 85.0
	defaultMemPercent          = 85.0
	defaultDiskPercent         = 90.0
	defaultFileSizeGB          = 2.0
	defaultSwapPercent         = 50.0
	defaultFileGrowthMBPerHour = 512.0
	lowFreeDiskBytes           = 5 * 1024 * 1024 * 1024
)

// NewP0Rules 返回 P0 必需规则集合。
func NewP0Rules() []Rule {
	return []Rule{
		newP0Rule("PORT-001", "port", checkPort001),
		newP0Rule("PORT-002", "port", checkPort002),
		newP0Rule("PROC-001", "proc", checkProc001),
		newP0Rule("PROC-002", "proc", checkProc002),
		newP0Rule("CPU-001", "cpu", checkCPU001),
		newP0Rule("MEM-001", "mem", checkMEM001),
		newP0Rule("DISK-001", "disk", checkDisk001),
		newP0Rule("DISK-002", "disk", checkDisk002),
		newP0Rule("FILE-001", "file", checkFile001),
		newP0Rule("ENV-001", "env", checkEnv001),
	}
}

type p0Rule struct {
	id     string
	module string
	check  func(in DiagnoseInput) *model.Issue
}

func newP0Rule(id string, module string, check func(in DiagnoseInput) *model.Issue) *p0Rule {
	return &p0Rule{id: id, module: module, check: check}
}

func (r *p0Rule) ID() string {
	return r.id
}

func (r *p0Rule) Phase() string {
	return phaseP0
}

func (r *p0Rule) Module() string {
	return r.module
}

func (r *p0Rule) Check(ctx context.Context, in DiagnoseInput) (*model.Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.check == nil {
		return nil, nil
	}
	return r.check(in), nil
}

func checkPort001(in DiagnoseInput) *model.Issue {
	opts := resolvedOptions(in)
	criticalSet := intSet(opts.Policy.CriticalPorts)
	excludedPorts := intSet(opts.Excludes.Ports)
	excludedProcesses := stringSet(opts.Excludes.Processes)

	offenders := make([]PortSnapshot, 0, len(in.Snapshots.Ports))
	for _, item := range in.Snapshots.Ports {
		if _, ok := criticalSet[item.Port]; !ok {
			continue
		}
		if _, ok := excludedPorts[item.Port]; ok {
			continue
		}
		if containsNormalized(excludedProcesses, item.ProcessName) {
			continue
		}
		offenders = append(offenders, item)
	}
	if len(offenders) == 0 {
		return nil
	}

	sort.Slice(offenders, func(i int, j int) bool {
		if offenders[i].Port != offenders[j].Port {
			return offenders[i].Port < offenders[j].Port
		}
		return offenders[i].PID < offenders[j].PID
	})
	first := offenders[0]
	return &model.Issue{
		RuleID:      "PORT-001",
		Severity:    model.SeverityCritical,
		Summary:     fmt.Sprintf("关键端口 %d 被进程 %s(PID %d) 占用", first.Port, fallbackText(first.ProcessName, "unknown"), first.PID),
		Evidence:    map[string]any{"critical_ports": sortedIntKeys(criticalSet), "matches": portEvidence(offenders)},
		Impact:      "关键服务可能无法启动或发生端口冲突",
		Suggestion:  "先确认进程用途，再执行端口释放",
		FixCommand:  fmt.Sprintf("syskit port kill %d --apply", first.Port),
		AutoFixable: true,
		Confidence:  100,
		Scope:       model.ScopeLocal,
	}
}

func checkPort002(in DiagnoseInput) *model.Issue {
	opts := resolvedOptions(in)
	allowList := stringSet(opts.Policy.AllowPublicListen)

	offenders := make([]PortSnapshot, 0, len(in.Snapshots.Ports))
	for _, item := range in.Snapshots.Ports {
		if !isPublicAddress(item.LocalAddr) {
			continue
		}
		if containsNormalized(allowList, item.ProcessName) {
			continue
		}
		offenders = append(offenders, item)
	}
	if len(offenders) == 0 {
		return nil
	}

	sort.Slice(offenders, func(i int, j int) bool {
		if offenders[i].Port != offenders[j].Port {
			return offenders[i].Port < offenders[j].Port
		}
		return offenders[i].PID < offenders[j].PID
	})
	first := offenders[0]
	return &model.Issue{
		RuleID:      "PORT-002",
		Severity:    model.SeverityHigh,
		Summary:     fmt.Sprintf("进程 %s(PID %d) 监听公网地址 %s", fallbackText(first.ProcessName, "unknown"), first.PID, first.LocalAddr),
		Evidence:    map[string]any{"allow_public_listen": sortedStringKeys(allowList), "matches": portEvidence(offenders)},
		Impact:      "服务暴露到公网，存在安全风险",
		Suggestion:  "将监听地址收敛到内网或回环地址，并按需调整策略白名单",
		FixCommand:  "",
		AutoFixable: false,
		Confidence:  90,
		Scope:       model.ScopeSystem,
	}
}

func checkProc001(in DiagnoseInput) *model.Issue {
	opts := resolvedOptions(in)
	threshold := opts.Thresholds.CPUPercent
	excludedProcesses := stringSet(opts.Excludes.Processes)

	offenders := make([]ProcessSnapshot, 0, len(in.Snapshots.Processes))
	for _, item := range in.Snapshots.Processes {
		if containsNormalized(excludedProcesses, item.Name) {
			continue
		}
		if item.CPUPercent < threshold {
			continue
		}
		offenders = append(offenders, item)
	}
	if len(offenders) == 0 {
		return nil
	}

	sort.Slice(offenders, func(i int, j int) bool {
		if offenders[i].CPUPercent != offenders[j].CPUPercent {
			return offenders[i].CPUPercent > offenders[j].CPUPercent
		}
		return offenders[i].PID < offenders[j].PID
	})
	first := offenders[0]
	return &model.Issue{
		RuleID:      "PROC-001",
		Severity:    model.SeverityHigh,
		Summary:     fmt.Sprintf("进程 %s(PID %d) CPU 使用率 %.1f%% 超过阈值 %.1f%%", fallbackText(first.Name, "unknown"), first.PID, first.CPUPercent, threshold),
		Evidence:    map[string]any{"threshold": threshold, "matches": processEvidence(offenders, 5)},
		Impact:      "持续高 CPU 占用会导致系统响应下降",
		Suggestion:  "先确认进程业务用途，再考虑降载或终止异常进程",
		FixCommand:  fmt.Sprintf("syskit proc kill %d --apply", first.PID),
		AutoFixable: true,
		Confidence:  90,
		Scope:       model.ScopeLocal,
	}
}

func checkProc002(in DiagnoseInput) *model.Issue {
	opts := resolvedOptions(in)
	threshold := opts.Thresholds.MemPercent
	excludedProcesses := stringSet(opts.Excludes.Processes)

	offenders := make([]MemoryProcess, 0, len(in.Snapshots.MemoryTop))
	for _, item := range in.Snapshots.MemoryTop {
		if containsNormalized(excludedProcesses, item.Name) {
			continue
		}
		if item.MemPercent >= threshold || (item.SwapBytes > 0 && item.MemPercent >= threshold*0.7) {
			offenders = append(offenders, item)
		}
	}
	if len(offenders) == 0 {
		return nil
	}

	sort.Slice(offenders, func(i int, j int) bool {
		if offenders[i].MemPercent != offenders[j].MemPercent {
			return offenders[i].MemPercent > offenders[j].MemPercent
		}
		return offenders[i].PID < offenders[j].PID
	})
	first := offenders[0]
	return &model.Issue{
		RuleID:      "PROC-002",
		Severity:    model.SeverityHigh,
		Summary:     fmt.Sprintf("进程 %s(PID %d) 内存占用 %.1f%% 超过阈值 %.1f%%", fallbackText(first.Name, "unknown"), first.PID, first.MemPercent, threshold),
		Evidence:    map[string]any{"threshold": threshold, "matches": memoryProcessEvidence(offenders, 5)},
		Impact:      "异常内存占用可能引发 OOM 或系统抖动",
		Suggestion:  "排查进程内存增长原因，必要时重启或结束进程",
		FixCommand:  fmt.Sprintf("syskit proc kill %d --apply", first.PID),
		AutoFixable: true,
		Confidence:  88,
		Scope:       model.ScopeLocal,
	}
}

func checkCPU001(in DiagnoseInput) *model.Issue {
	overview := in.Snapshots.CPU
	if overview == nil {
		return nil
	}
	opts := resolvedOptions(in)
	threshold := opts.Thresholds.CPUPercent

	highUsage := overview.UsagePercent >= threshold
	highLoad := overview.CPUCores > 0 && overview.Load1 > float64(overview.CPUCores*2)
	if !highUsage && !highLoad {
		return nil
	}

	summary := fmt.Sprintf("系统 CPU 使用率 %.1f%% 超过阈值 %.1f%%", overview.UsagePercent, threshold)
	if !highUsage && highLoad {
		summary = fmt.Sprintf("系统 load1 %.2f 超过核心数阈值 %.2f", overview.Load1, float64(overview.CPUCores*2))
	}

	return &model.Issue{
		RuleID:      "CPU-001",
		Severity:    model.SeverityHigh,
		Summary:     summary,
		Evidence:    map[string]any{"cpu_cores": overview.CPUCores, "usage_percent": overview.UsagePercent, "load1": overview.Load1, "load5": overview.Load5, "load15": overview.Load15, "top_processes": processEvidence(overview.TopProcesses, 5)},
		Impact:      "系统整体吞吐下降，可能影响业务稳定性",
		Suggestion:  "优先定位高 CPU 进程并确认是否属于计划内高负载",
		FixCommand:  "",
		AutoFixable: false,
		Confidence:  85,
		Scope:       model.ScopeSystem,
	}
}

func checkMEM001(in DiagnoseInput) *model.Issue {
	overview := in.Snapshots.Memory
	if overview == nil {
		return nil
	}
	opts := resolvedOptions(in)
	memThreshold := opts.Thresholds.MemPercent
	swapThreshold := opts.Thresholds.SwapPercent

	availablePercent := 100.0
	if overview.TotalBytes > 0 {
		availablePercent = (float64(overview.AvailableBytes) / float64(overview.TotalBytes)) * 100
	}
	lowAvailable := availablePercent <= (100 - memThreshold)
	highUsage := overview.UsagePercent >= memThreshold
	highSwap := overview.SwapUsagePercent >= swapThreshold
	if !lowAvailable && !(highUsage && highSwap) {
		return nil
	}

	summary := fmt.Sprintf("可用内存比例 %.1f%% 低于安全阈值", availablePercent)
	if !lowAvailable {
		summary = fmt.Sprintf("内存使用率 %.1f%% 与 swap 使用率 %.1f%% 同时偏高", overview.UsagePercent, overview.SwapUsagePercent)
	}

	return &model.Issue{
		RuleID:      "MEM-001",
		Severity:    model.SeverityHigh,
		Summary:     summary,
		Evidence:    map[string]any{"total_bytes": overview.TotalBytes, "available_bytes": overview.AvailableBytes, "available_percent": availablePercent, "usage_percent": overview.UsagePercent, "swap_usage_percent": overview.SwapUsagePercent, "threshold": memThreshold, "swap_threshold": swapThreshold},
		Impact:      "可用内存不足会导致系统卡顿并增加 OOM 风险",
		Suggestion:  "清理高占用进程或缓存，并评估内存容量",
		FixCommand:  "",
		AutoFixable: false,
		Confidence:  90,
		Scope:       model.ScopeSystem,
	}
}

func checkDisk001(in DiagnoseInput) *model.Issue {
	opts := resolvedOptions(in)
	threshold := opts.Thresholds.DiskPercent

	offenders := make([]DiskPartition, 0, len(in.Snapshots.Disk))
	for _, item := range in.Snapshots.Disk {
		if item.UsagePercent >= threshold || item.FreeBytes <= lowFreeDiskBytes {
			offenders = append(offenders, item)
		}
	}
	if len(offenders) == 0 {
		return nil
	}

	sort.Slice(offenders, func(i int, j int) bool {
		if offenders[i].UsagePercent != offenders[j].UsagePercent {
			return offenders[i].UsagePercent > offenders[j].UsagePercent
		}
		return offenders[i].MountPoint < offenders[j].MountPoint
	})
	first := offenders[0]
	return &model.Issue{
		RuleID:      "DISK-001",
		Severity:    model.SeverityCritical,
		Summary:     fmt.Sprintf("分区 %s 使用率 %.1f%% 超过阈值 %.1f%%", fallbackText(first.MountPoint, "/"), first.UsagePercent, threshold),
		Evidence:    map[string]any{"threshold": threshold, "matches": diskEvidence(offenders, 5), "low_free_bytes_threshold": lowFreeDiskBytes},
		Impact:      "磁盘空间不足会导致写入失败和服务异常",
		Suggestion:  "优先定位大文件和日志膨胀点，必要时执行清理",
		FixCommand:  "syskit fix cleanup --apply",
		AutoFixable: true,
		Confidence:  95,
		Scope:       model.ScopeSystem,
	}
}

func checkDisk002(in DiagnoseInput) *model.Issue {
	samples := in.Snapshots.DiskGrowth
	if len(samples) == 0 {
		return nil
	}

	offenders := make([]DiskGrowthSample, 0, len(samples))
	for _, sample := range samples {
		baseline := sample.BaselineGBPerDay
		threshold := 10.0
		if baseline > 0 {
			threshold = math.Max(5, baseline*2)
		}
		if sample.GrowthRateGBPerDay > threshold {
			offenders = append(offenders, sample)
		}
	}
	if len(offenders) == 0 {
		return nil
	}

	sort.Slice(offenders, func(i int, j int) bool {
		if offenders[i].GrowthRateGBPerDay != offenders[j].GrowthRateGBPerDay {
			return offenders[i].GrowthRateGBPerDay > offenders[j].GrowthRateGBPerDay
		}
		return offenders[i].MountPoint < offenders[j].MountPoint
	})
	first := offenders[0]
	path := fallbackText(first.MountPoint, "/")
	return &model.Issue{
		RuleID:      "DISK-002",
		Severity:    model.SeverityHigh,
		Summary:     fmt.Sprintf("分区 %s 日增长 %.2fGB 超过历史阈值", path, first.GrowthRateGBPerDay),
		Evidence:    map[string]any{"matches": diskGrowthEvidence(offenders, 5)},
		Impact:      "磁盘增长异常可能在短时间内耗尽可用空间",
		Suggestion:  "排查近期新增文件和日志增长来源",
		FixCommand:  fmt.Sprintf("syskit disk scan %s", path),
		AutoFixable: false,
		Confidence:  82,
		Scope:       model.ScopeSystem,
	}
}

func checkFile001(in DiagnoseInput) *model.Issue {
	opts := resolvedOptions(in)
	sizeThreshold := int64(opts.Thresholds.FileSizeGB * 1024 * 1024 * 1024)
	growthThreshold := opts.Thresholds.FileGrowthMBPerHour

	offenders := make([]FileObservation, 0, len(in.Snapshots.Files))
	for _, item := range in.Snapshots.Files {
		if item.SizeBytes >= sizeThreshold || item.GrowthMBPerHour >= growthThreshold {
			offenders = append(offenders, item)
		}
	}
	if len(offenders) == 0 {
		return nil
	}

	sort.Slice(offenders, func(i int, j int) bool {
		if offenders[i].SizeBytes != offenders[j].SizeBytes {
			return offenders[i].SizeBytes > offenders[j].SizeBytes
		}
		return offenders[i].Path < offenders[j].Path
	})
	first := offenders[0]
	return &model.Issue{
		RuleID:      "FILE-001",
		Severity:    model.SeverityHigh,
		Summary:     fmt.Sprintf("文件 %s 体积或增长速度异常", fallbackText(first.Path, "unknown")),
		Evidence:    map[string]any{"size_threshold_bytes": sizeThreshold, "growth_threshold_mb_per_hour": growthThreshold, "matches": fileEvidence(offenders, 5)},
		Impact:      "大文件或日志膨胀会快速消耗磁盘空间",
		Suggestion:  "检查日志轮转和缓存策略，并清理历史垃圾文件",
		FixCommand:  "syskit fix cleanup --apply",
		AutoFixable: true,
		Confidence:  84,
		Scope:       model.ScopeLocal,
	}
}

func checkEnv001(in DiagnoseInput) *model.Issue {
	entries := in.Snapshots.PathEntries
	if len(entries) == 0 {
		return nil
	}

	seen := make(map[string]string, len(entries))
	duplicates := make([]string, 0, 4)
	duplicateSet := make(map[string]struct{})
	for _, entry := range entries {
		normalized := normalizePathEntry(entry)
		if normalized == "" {
			continue
		}
		if first, exists := seen[normalized]; exists {
			if _, ok := duplicateSet[normalized]; !ok {
				duplicates = append(duplicates, first)
				duplicateSet[normalized] = struct{}{}
			}
			duplicates = append(duplicates, entry)
			continue
		}
		seen[normalized] = entry
	}
	if len(duplicates) == 0 {
		return nil
	}

	return &model.Issue{
		RuleID:      "ENV-001",
		Severity:    model.SeverityMedium,
		Summary:     fmt.Sprintf("PATH 中发现 %d 个重复路径项", len(duplicates)),
		Evidence:    map[string]any{"duplicates": duplicates, "path_length": len(entries)},
		Impact:      "重复 PATH 可能导致命令解析顺序混乱",
		Suggestion:  "移除重复项并保持工具链路径顺序一致",
		FixCommand:  "",
		AutoFixable: false,
		Confidence:  85,
		Scope:       model.ScopeLocal,
	}
}

func resolvedOptions(in DiagnoseInput) DiagnoseOptions {
	opts := in.Options
	thresholds := opts.Thresholds
	if thresholds.CPUPercent <= 0 {
		thresholds.CPUPercent = defaultCPUPercent
	}
	if thresholds.MemPercent <= 0 {
		thresholds.MemPercent = defaultMemPercent
	}
	if thresholds.DiskPercent <= 0 {
		thresholds.DiskPercent = defaultDiskPercent
	}
	if thresholds.FileSizeGB <= 0 {
		thresholds.FileSizeGB = defaultFileSizeGB
	}
	if thresholds.SwapPercent <= 0 {
		thresholds.SwapPercent = defaultSwapPercent
	}
	if thresholds.FileGrowthMBPerHour <= 0 {
		thresholds.FileGrowthMBPerHour = defaultFileGrowthMBPerHour
	}
	opts.Thresholds = thresholds
	if len(opts.Policy.CriticalPorts) == 0 {
		opts.Policy.CriticalPorts = append([]int(nil), defaultCriticalPorts...)
	}
	return opts
}

func isPublicAddress(addr string) bool {
	value := strings.ToLower(strings.TrimSpace(addr))
	if value == "" {
		return false
	}
	return strings.HasPrefix(value, "0.0.0.0") || strings.HasPrefix(value, "[::]") || strings.HasPrefix(value, "::") || strings.HasPrefix(value, "*:") || strings.HasPrefix(value, ":::")
}

func normalizePathEntry(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(filepath.Clean(trimmed))
}

func intSet(values []int) map[int]struct{} {
	set := make(map[int]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func sortedIntKeys(set map[int]struct{}) []int {
	keys := make([]int, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	return set
}

func sortedStringKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsNormalized(set map[string]struct{}, value string) bool {
	if len(set) == 0 {
		return false
	}
	_, ok := set[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

func portEvidence(items []PortSnapshot) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"port":          item.Port,
			"pid":           item.PID,
			"process_name":  item.ProcessName,
			"local_address": item.LocalAddr,
			"command":       item.Command,
			"parent_pid":    item.ParentPID,
		})
	}
	return result
}

func processEvidence(items []ProcessSnapshot, top int) []map[string]any {
	if top <= 0 || len(items) <= top {
		top = len(items)
	}
	result := make([]map[string]any, 0, top)
	for _, item := range items[:top] {
		result = append(result, map[string]any{
			"pid":          item.PID,
			"process_name": item.Name,
			"cpu_percent":  item.CPUPercent,
			"rss_bytes":    item.RSSBytes,
			"vms_bytes":    item.VMSBytes,
			"command":      item.Command,
		})
	}
	return result
}

func memoryProcessEvidence(items []MemoryProcess, top int) []map[string]any {
	if top <= 0 || len(items) <= top {
		top = len(items)
	}
	result := make([]map[string]any, 0, top)
	for _, item := range items[:top] {
		result = append(result, map[string]any{
			"pid":          item.PID,
			"process_name": item.Name,
			"mem_percent":  item.MemPercent,
			"rss_bytes":    item.RSSBytes,
			"swap_bytes":   item.SwapBytes,
			"command":      item.Command,
		})
	}
	return result
}

func diskEvidence(items []DiskPartition, top int) []map[string]any {
	if top <= 0 || len(items) <= top {
		top = len(items)
	}
	result := make([]map[string]any, 0, top)
	for _, item := range items[:top] {
		result = append(result, map[string]any{
			"mount_point":   item.MountPoint,
			"usage_percent": item.UsagePercent,
			"free_bytes":    item.FreeBytes,
		})
	}
	return result
}

func diskGrowthEvidence(items []DiskGrowthSample, top int) []map[string]any {
	if top <= 0 || len(items) <= top {
		top = len(items)
	}
	result := make([]map[string]any, 0, top)
	for _, item := range items[:top] {
		result = append(result, map[string]any{
			"mount_point":            item.MountPoint,
			"growth_rate_gb_per_day": item.GrowthRateGBPerDay,
			"baseline_gb_per_day":    item.BaselineGBPerDay,
			"window_days":            item.WindowDays,
		})
	}
	return result
}

func fileEvidence(items []FileObservation, top int) []map[string]any {
	if top <= 0 || len(items) <= top {
		top = len(items)
	}
	result := make([]map[string]any, 0, top)
	for _, item := range items[:top] {
		result = append(result, map[string]any{
			"path":               item.Path,
			"size_bytes":         item.SizeBytes,
			"growth_mb_per_hour": item.GrowthMBPerHour,
			"last_modified":      item.LastModifiedTime,
		})
	}
	return result
}

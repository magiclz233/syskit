package port

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	proccollector "syskit/internal/collectors/proc"
	"syskit/internal/errs"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type processInfo struct {
	Name    string
	User    string
	Command string
	Parent  int32
}

// QueryPorts 查询指定端口的占用信息。
func QueryPorts(ctx context.Context, ports []int, detail bool) (*QueryResult, error) {
	if len(ports) == 0 {
		return nil, errs.InvalidArgument("至少指定一个端口")
	}

	entries, warnings, err := collectPortEntries(ctx, detail)
	if err != nil {
		return nil, err
	}

	portSet := make(map[int]struct{}, len(ports))
	for _, port := range ports {
		if port <= 0 || port > 65535 {
			return nil, errs.InvalidArgument(fmt.Sprintf("端口超出范围(1-65535): %d", port))
		}
		portSet[port] = struct{}{}
	}

	filtered := make([]PortEntry, 0, len(entries))
	foundSet := make(map[int]struct{}, len(ports))
	for _, entry := range entries {
		if _, ok := portSet[entry.Port]; !ok {
			continue
		}
		filtered = append(filtered, entry)
		foundSet[entry.Port] = struct{}{}
	}

	foundPorts := make([]int, 0, len(foundSet))
	for value := range foundSet {
		foundPorts = append(foundPorts, value)
	}
	sort.Ints(foundPorts)

	missingPorts := make([]int, 0, len(ports))
	for _, value := range ports {
		if _, ok := foundSet[value]; !ok {
			missingPorts = append(missingPorts, value)
		}
	}

	return &QueryResult{
		RequestedPorts: append([]int(nil), ports...),
		FoundPorts:     foundPorts,
		MissingPorts:   missingPorts,
		Entries:        filtered,
		Warnings:       warnings,
	}, nil
}

// ListPorts 列出监听端口信息。
func ListPorts(ctx context.Context, opts ListOptions, detail bool) (*ListResult, error) {
	entries, warnings, err := collectPortEntries(ctx, detail)
	if err != nil {
		return nil, err
	}

	filtered := make([]PortEntry, 0, len(entries))
	listenFilter := strings.ToLower(strings.TrimSpace(opts.Listen))
	for _, item := range entries {
		if opts.Protocol != "" && item.Protocol != opts.Protocol {
			continue
		}
		if listenFilter != "" && !strings.Contains(strings.ToLower(item.LocalAddr), listenFilter) {
			continue
		}
		filtered = append(filtered, item)
	}

	switch opts.By {
	case "pid":
		sort.Slice(filtered, func(i int, j int) bool {
			if filtered[i].PID != filtered[j].PID {
				return filtered[i].PID < filtered[j].PID
			}
			if filtered[i].Port != filtered[j].Port {
				return filtered[i].Port < filtered[j].Port
			}
			return filtered[i].LocalAddr < filtered[j].LocalAddr
		})
	default:
		sort.Slice(filtered, func(i int, j int) bool {
			if filtered[i].Port != filtered[j].Port {
				return filtered[i].Port < filtered[j].Port
			}
			if filtered[i].Protocol != filtered[j].Protocol {
				return filtered[i].Protocol < filtered[j].Protocol
			}
			return filtered[i].PID < filtered[j].PID
		})
	}

	return &ListResult{
		By:       opts.By,
		Protocol: string(opts.Protocol),
		Listen:   opts.Listen,
		Total:    len(filtered),
		Entries:  filtered,
		Warnings: warnings,
	}, nil
}

// BuildKillPlan 按 `discover/plan` 阶段生成 `port kill` 执行计划。
func BuildKillPlan(ctx context.Context, opts KillOptions) (*KillPlan, error) {
	if opts.Port <= 0 || opts.Port > 65535 {
		return nil, errs.InvalidArgument(fmt.Sprintf("端口超出范围(1-65535): %d", opts.Port))
	}

	query, err := QueryPorts(ctx, []int{opts.Port}, true)
	if err != nil {
		return nil, err
	}
	if len(query.Entries) == 0 {
		return nil, errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, fmt.Sprintf("未找到占用端口 %d 的监听进程", opts.Port))
	}

	targetByPID := make(map[int32]*KillTarget)
	for _, entry := range query.Entries {
		if entry.PID <= 0 {
			continue
		}
		target, ok := targetByPID[entry.PID]
		if !ok {
			target = &KillTarget{
				PID:         entry.PID,
				ProcessName: entry.ProcessName,
			}
			targetByPID[entry.PID] = target
		}
		if !slices.Contains(target.Protocols, entry.Protocol) {
			target.Protocols = append(target.Protocols, entry.Protocol)
		}
	}

	targets := make([]KillTarget, 0, len(targetByPID))
	for _, target := range targetByPID {
		sort.Slice(target.Protocols, func(i int, j int) bool {
			return target.Protocols[i] < target.Protocols[j]
		})
		targets = append(targets, *target)
	}
	sort.Slice(targets, func(i int, j int) bool {
		return targets[i].PID < targets[j].PID
	})

	if len(targets) == 0 {
		return nil, errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, fmt.Sprintf("端口 %d 存在监听记录但未识别到可终止进程", opts.Port))
	}

	return &KillPlan{
		Port:     opts.Port,
		Force:    opts.Force,
		KillTree: opts.KillTree,
		Targets:  targets,
		Steps: []string{
			"discover: 定位端口占用进程",
			"plan: 生成 dry-run 计划",
			"confirm: 检查 --apply 与 --yes",
			"apply: 终止目标进程",
			"verify: 校验端口是否释放",
			"audit: 审计日志（P0-022 接入后落盘）",
		},
		Warnings:   query.Warnings,
		Connection: query.Entries,
	}, nil
}

// ExecuteKillPlan 执行 `port kill` 计划并校验端口释放结果。
func ExecuteKillPlan(ctx context.Context, plan *KillPlan) (*KillResult, error) {
	if plan == nil {
		return nil, errs.InvalidArgument("kill 计划不能为空")
	}
	if len(plan.Targets) == 0 {
		return nil, errs.InvalidArgument("kill 计划没有可执行目标")
	}

	result := &KillResult{
		Plan:          plan,
		Applied:       true,
		ProcessResult: make([]KillProcessResult, 0, len(plan.Targets)),
	}

	for _, target := range plan.Targets {
		if err := ctx.Err(); err != nil {
			return nil, mapTimeoutError(err)
		}

		procPlan, err := proccollector.BuildKillPlan(ctx, target.PID, plan.KillTree, plan.Force)
		if err != nil {
			code := errs.ErrorCode(err)
			if code == errs.CodeTimeout {
				return nil, err
			}
			if code == errs.CodePermissionDenied {
				return nil, err
			}
			status := "failed"
			if code == errs.CodeNotFound {
				status = "already_exited"
			}
			result.ProcessResult = append(result.ProcessResult, KillProcessResult{
				PID:         target.PID,
				ProcessName: target.ProcessName,
				Status:      status,
				Message:     errs.Message(err),
			})
			continue
		}

		procResult, err := proccollector.ExecuteKillPlan(ctx, procPlan)
		if err != nil {
			code := errs.ErrorCode(err)
			if code == errs.CodeTimeout {
				return nil, err
			}
			if code == errs.CodePermissionDenied {
				return nil, err
			}
			result.ProcessResult = append(result.ProcessResult, KillProcessResult{
				PID:         target.PID,
				ProcessName: target.ProcessName,
				Status:      "failed",
				Message:     errs.Message(err),
			})
			continue
		}

		status := "killed"
		message := ""
		if failed := proccollector.CountKillFailures(procResult); failed > 0 {
			status = "failed"
			message = fmt.Sprintf("存在 %d 个子进程终止失败", failed)
		}
		result.ProcessResult = append(result.ProcessResult, KillProcessResult{
			PID:         target.PID,
			ProcessName: target.ProcessName,
			Status:      status,
			Message:     message,
		})
		if len(procResult.Warnings) > 0 {
			result.Warnings = appendUniqueWarnings(result.Warnings, procResult.Warnings...)
		}
	}

	verify, err := QueryPorts(ctx, []int{plan.Port}, true)
	if err != nil {
		return nil, err
	}
	result.Remaining = verify.Entries
	result.Released = len(verify.Entries) == 0
	result.Warnings = appendUniqueWarnings(result.Warnings, verify.Warnings...)
	if !result.Released {
		result.Warnings = appendUniqueWarnings(result.Warnings, fmt.Sprintf("端口 %d 仍有占用", plan.Port))
	}
	return result, nil
}

func collectPortEntries(ctx context.Context, detail bool) ([]PortEntry, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	connections, err := gnet.ConnectionsWithContext(ctx, "inet")
	if err != nil {
		return nil, nil, mapCollectionError("读取端口连接信息失败", err)
	}

	entries := make([]PortEntry, 0, len(connections))
	warnings := make([]string, 0, 4)
	warnSet := make(map[string]struct{})
	processCache := make(map[int32]processInfo)
	seen := make(map[string]struct{})

	for _, conn := range connections {
		if err := ctx.Err(); err != nil {
			return nil, nil, mapTimeoutError(err)
		}
		if !isListener(conn) {
			continue
		}

		port := int(conn.Laddr.Port)
		if port <= 0 || port > 65535 {
			continue
		}
		protocol := resolveProtocol(conn.Type)
		if protocol == "" {
			continue
		}

		entry := PortEntry{
			Port:      port,
			Protocol:  protocol,
			LocalAddr: formatLocalAddr(conn),
			Status:    normalizeStatus(conn.Status),
			PID:       conn.Pid,
		}

		if detail && conn.Pid > 0 {
			info := resolveProcessInfo(ctx, conn.Pid, processCache, warnSet)
			entry.ProcessName = info.Name
			entry.User = info.User
			entry.Command = info.Command
			entry.ParentPID = info.Parent
		}

		key := fmt.Sprintf("%d|%s|%s|%d|%s", entry.Port, entry.Protocol, entry.LocalAddr, entry.PID, entry.Status)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, entry)
	}

	warnings = append(warnings, warningSlice(warnSet)...)
	sort.Slice(entries, func(i int, j int) bool {
		if entries[i].Port != entries[j].Port {
			return entries[i].Port < entries[j].Port
		}
		if entries[i].Protocol != entries[j].Protocol {
			return entries[i].Protocol < entries[j].Protocol
		}
		if entries[i].PID != entries[j].PID {
			return entries[i].PID < entries[j].PID
		}
		return entries[i].LocalAddr < entries[j].LocalAddr
	})
	return entries, warnings, nil
}

func resolveProcessInfo(ctx context.Context, pid int32, cache map[int32]processInfo, warningSet map[string]struct{}) processInfo {
	if cached, ok := cache[pid]; ok {
		return cached
	}

	info := processInfo{}
	procRef, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		addWarning(warningSet, "proc-lookup", fmt.Sprintf("读取 PID=%d 进程信息失败: %v", pid, err))
		cache[pid] = info
		return info
	}

	if name, err := procRef.NameWithContext(ctx); err == nil {
		info.Name = strings.TrimSpace(name)
	}
	if user, err := procRef.UsernameWithContext(ctx); err == nil {
		info.User = strings.TrimSpace(user)
	}
	if command, err := procRef.CmdlineWithContext(ctx); err == nil {
		info.Command = strings.TrimSpace(command)
	}
	if parent, err := procRef.PpidWithContext(ctx); err == nil {
		info.Parent = parent
	}

	cache[pid] = info
	return info
}

func isListener(conn gnet.ConnectionStat) bool {
	protocol := resolveProtocol(conn.Type)
	if protocol == "" {
		return false
	}
	if protocol == ProtocolUDP {
		return conn.Laddr.Port > 0
	}
	status := strings.ToUpper(strings.TrimSpace(conn.Status))
	return status == "LISTEN" || status == "NONE" || status == ""
}

func resolveProtocol(sockType uint32) Protocol {
	switch sockType {
	case 1:
		return ProtocolTCP
	case 2:
		return ProtocolUDP
	default:
		return ""
	}
}

func formatLocalAddr(conn gnet.ConnectionStat) string {
	ip := strings.TrimSpace(conn.Laddr.IP)
	if ip == "" {
		ip = "*"
	}
	return fmt.Sprintf("%s:%d", ip, conn.Laddr.Port)
}

func normalizeStatus(raw string) string {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		return "listen"
	}
	return status
}

func mapCollectionError(message string, err error) error {
	if err == nil {
		return nil
	}
	if timeout := mapTimeoutError(err); timeout != nil {
		return timeout
	}
	if permission := mapPermissionError(message, err); permission != nil {
		return permission
	}
	return errs.ExecutionFailed(message, err)
}

func mapTimeoutError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "命令执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
	}
	return nil
}

func mapPermissionError(message string, err error) error {
	if err == nil {
		return nil
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "permission denied") || strings.Contains(text, "access denied") || strings.Contains(text, "operation not permitted") {
		return errs.PermissionDenied(message, "请提升权限后重试")
	}
	return nil
}

func warningSlice(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	items := make([]string, 0, len(set))
	for value := range set {
		items = append(items, value)
	}
	slices.Sort(items)
	return items
}

func addWarning(set map[string]struct{}, key string, message string) {
	if set == nil {
		return
	}
	set[key+": "+message] = struct{}{}
}

func appendUniqueWarnings(items []string, values ...string) []string {
	for _, value := range values {
		exists := false
		for _, item := range items {
			if item == value {
				exists = true
				break
			}
		}
		if !exists {
			items = append(items, value)
		}
	}
	return items
}

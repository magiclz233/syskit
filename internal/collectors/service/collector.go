package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syskit/internal/errs"
)

var (
	// runtimeName 和 commandRunner 保持可替换，便于单元测试覆盖平台差异分支。
	runtimeName   = runtime.GOOS
	commandRunner = defaultCommandRunner
)

var allowedStates = map[string]struct{}{
	"running": {},
	"stopped": {},
	"failed":  {},
	"pending": {},
	"unknown": {},
}

var allowedStartup = map[string]struct{}{
	"auto":     {},
	"manual":   {},
	"disabled": {},
	"unknown":  {},
}

// ListServices 采集并过滤系统服务列表。
// 当平台命令缺失或执行失败时，函数会降级为空结果并返回 warning，避免只读场景被硬阻断。
func ListServices(ctx context.Context, opts ListOptions) (*ListResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	stateSet, stateFilter, err := parseFilterSet(opts.State, allowedStates, "--state")
	if err != nil {
		return nil, err
	}
	startupSet, startupFilter, err := parseFilterSet(opts.Startup, allowedStartup, "--startup")
	if err != nil {
		return nil, err
	}
	nameFilter := strings.ToLower(strings.TrimSpace(opts.Name))

	services, warnings, collectErr := collectServiceEntries(ctx)
	if collectErr != nil {
		if errs.ErrorCode(collectErr) == errs.CodeTimeout {
			return nil, collectErr
		}
		warnings = append(warnings, "服务采集降级: "+errs.Message(collectErr))
		services = nil
	}

	filtered := filterServices(services, stateSet, startupSet, nameFilter)
	return &ListResult{
		Platform:      runtimeName,
		StateFilter:   stateFilter,
		StartupFilter: startupFilter,
		NameFilter:    strings.TrimSpace(opts.Name),
		Total:         len(filtered),
		Services:      filtered,
		Warnings:      dedupeSortedWarnings(warnings),
	}, nil
}

// CheckService 检查指定服务的健康状态。
// 默认按服务名精确匹配；`--all` 会改为模糊匹配并返回所有命中项。
func CheckService(ctx context.Context, name string, opts CheckOptions) (*CheckResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	target := strings.TrimSpace(name)
	if target == "" {
		return nil, errs.InvalidArgument("服务名不能为空")
	}

	services, warnings, collectErr := collectServiceEntries(ctx)
	if collectErr != nil {
		if errs.ErrorCode(collectErr) == errs.CodeTimeout {
			return nil, collectErr
		}
		warnings = append(warnings, "服务采集降级: "+errs.Message(collectErr))
		services = nil
	}

	matches := filterCheckTargets(services, target, opts.All)
	if opts.Detail && len(matches) > 0 {
		detailWarnings := make([]string, 0, len(matches))
		for idx := range matches {
			itemWarnings := enrichServiceDetail(ctx, &matches[idx])
			detailWarnings = append(detailWarnings, itemWarnings...)
		}
		warnings = append(warnings, detailWarnings...)
	}

	running := 0
	for _, item := range matches {
		if item.State == "running" {
			running++
		}
	}
	found := len(matches) > 0
	healthy := found && running == len(matches)

	summary := "未找到匹配服务"
	switch {
	case !found:
		summary = "未找到匹配服务"
	case healthy:
		summary = "服务状态健康（全部 running）"
	case running == 0:
		summary = "服务未运行"
	default:
		summary = fmt.Sprintf("共有 %d/%d 个服务处于 running", running, len(matches))
	}

	return &CheckResult{
		Platform: runtimeName,
		Name:     target,
		All:      opts.All,
		Detail:   opts.Detail,
		Found:    found,
		Healthy:  healthy,
		Matched:  len(matches),
		Running:  running,
		Summary:  summary,
		Services: matches,
		Warnings: dedupeSortedWarnings(warnings),
	}, nil
}

// ParseAction 解析并校验服务动作类型。
func ParseAction(raw string) (Action, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch Action(value) {
	case ActionStart, ActionStop, ActionRestart, ActionEnable, ActionDisable:
		return Action(value), nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("不支持的服务动作: %s", raw))
	}
}

// BuildActionPlan 生成服务写操作计划。
// 这里会先读取当前服务状态，保证 dry-run 输出和真实执行口径一致。
func BuildActionPlan(ctx context.Context, action Action, name string) (*ActionPlan, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	target := strings.TrimSpace(name)
	if target == "" {
		return nil, errs.InvalidArgument("服务名不能为空")
	}

	action, err := ParseAction(string(action))
	if err != nil {
		return nil, err
	}

	check, err := CheckService(ctx, target, CheckOptions{All: false, Detail: false})
	if err != nil {
		return nil, err
	}

	plan := &ActionPlan{
		Action:   action,
		Name:     target,
		Platform: runtimeName,
		Found:    check.Found,
		Steps:    make([]string, 0, 4),
		Warnings: append([]string{}, check.Warnings...),
	}
	if check.Found && len(check.Services) > 0 {
		plan.Current = check.Services[0]
	}

	plan.Steps = append(plan.Steps, fmt.Sprintf("检查服务当前状态: %s", check.Summary))
	switch action {
	case ActionStart:
		plan.Steps = append(plan.Steps, "执行启动命令")
	case ActionStop:
		plan.Steps = append(plan.Steps, "执行停止命令")
	case ActionRestart:
		plan.Steps = append(plan.Steps, "执行重启命令")
	case ActionEnable:
		plan.Steps = append(plan.Steps, "执行开机自启启用命令")
	case ActionDisable:
		plan.Steps = append(plan.Steps, "执行开机自启禁用命令")
	}
	plan.Steps = append(plan.Steps, "重新读取服务状态并验证结果")
	if !plan.Found {
		plan.Warnings = append(plan.Warnings, "未命中服务，真实执行预计会失败")
	}
	plan.Warnings = dedupeSortedWarnings(plan.Warnings)
	return plan, nil
}

// ExecuteAction 执行服务写操作。
func ExecuteAction(ctx context.Context, plan *ActionPlan) (*ActionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if plan == nil {
		return nil, errs.InvalidArgument("服务动作计划不能为空")
	}
	if !plan.Found {
		return nil, errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, "未找到目标服务: "+plan.Name)
	}

	before := plan.Current
	warnings := append([]string{}, plan.Warnings...)

	if err := runServiceAction(ctx, plan.Action, plan.Name); err != nil {
		return nil, err
	}

	checkAfter, checkErr := CheckService(ctx, plan.Name, CheckOptions{All: false, Detail: false})
	after := ServiceEntry{}
	if checkErr != nil {
		warnings = append(warnings, "执行后状态校验失败: "+errs.Message(checkErr))
	} else {
		warnings = append(warnings, checkAfter.Warnings...)
		if checkAfter.Found && len(checkAfter.Services) > 0 {
			after = checkAfter.Services[0]
		}
	}

	success := verifyActionOutcome(plan.Action, before, after, checkErr)
	summary := actionSummary(plan.Action, success, before, after)

	return &ActionResult{
		Action:   plan.Action,
		Name:     plan.Name,
		Platform: plan.Platform,
		Applied:  true,
		Success:  success,
		Summary:  summary,
		Before:   before,
		After:    after,
		Warnings: dedupeSortedWarnings(warnings),
	}, nil
}

func collectServiceEntries(ctx context.Context) ([]ServiceEntry, []string, error) {
	switch runtimeName {
	case "windows":
		return collectWindowsServices(ctx)
	case "linux":
		return collectLinuxServices(ctx)
	case "darwin":
		return collectDarwinServices(ctx)
	default:
		return nil, []string{"当前平台尚未接入服务采集，已降级为空结果"}, nil
	}
}

func runServiceAction(ctx context.Context, action Action, name string) error {
	target := strings.TrimSpace(name)
	if target == "" {
		return errs.InvalidArgument("服务名不能为空")
	}
	switch runtimeName {
	case "windows":
		return runWindowsServiceAction(ctx, action, target)
	case "linux":
		return runLinuxServiceAction(ctx, action, target)
	case "darwin":
		return runDarwinServiceAction(ctx, action, target)
	default:
		return errs.NewWithSuggestion(
			errs.ExitExecutionFailed,
			errs.CodePlatformUnsupported,
			"当前平台不支持 service 写操作",
			"请在受支持的平台执行该命令",
		)
	}
}

func runWindowsServiceAction(ctx context.Context, action Action, name string) error {
	script := ""
	switch action {
	case ActionStart:
		script = fmt.Sprintf("$ErrorActionPreference='Stop'; Start-Service -Name '%s'", escapeSingleQuote(name))
	case ActionStop:
		script = fmt.Sprintf("$ErrorActionPreference='Stop'; Stop-Service -Name '%s'", escapeSingleQuote(name))
	case ActionRestart:
		script = fmt.Sprintf("$ErrorActionPreference='Stop'; Restart-Service -Name '%s' -Force", escapeSingleQuote(name))
	case ActionEnable:
		script = fmt.Sprintf("$ErrorActionPreference='Stop'; Set-Service -Name '%s' -StartupType Automatic", escapeSingleQuote(name))
	case ActionDisable:
		script = fmt.Sprintf("$ErrorActionPreference='Stop'; Set-Service -Name '%s' -StartupType Disabled", escapeSingleQuote(name))
	default:
		return errs.InvalidArgument("不支持的服务动作")
	}
	if _, err := commandRunner(ctx, "powershell", "-NoProfile", "-Command", script); err != nil {
		return mapCommandError(ctx, "执行 Windows 服务操作失败", "powershell", err)
	}
	return nil
}

func runLinuxServiceAction(ctx context.Context, action Action, name string) error {
	args := []string{}
	switch action {
	case ActionStart:
		args = []string{"start", name}
	case ActionStop:
		args = []string{"stop", name}
	case ActionRestart:
		args = []string{"restart", name}
	case ActionEnable:
		args = []string{"enable", name}
	case ActionDisable:
		args = []string{"disable", name}
	default:
		return errs.InvalidArgument("不支持的服务动作")
	}
	if _, err := commandRunner(ctx, "systemctl", args...); err != nil {
		return mapCommandError(ctx, "执行 Linux 服务操作失败", "systemctl", err)
	}
	return nil
}

func runDarwinServiceAction(ctx context.Context, action Action, name string) error {
	switch action {
	case ActionStart:
		if _, err := commandRunner(ctx, "launchctl", "start", name); err != nil {
			return mapCommandError(ctx, "执行 macOS 服务启动失败", "launchctl", err)
		}
		return nil
	case ActionStop:
		if _, err := commandRunner(ctx, "launchctl", "stop", name); err != nil {
			return mapCommandError(ctx, "执行 macOS 服务停止失败", "launchctl", err)
		}
		return nil
	case ActionRestart:
		_, _ = commandRunner(ctx, "launchctl", "stop", name)
		if _, err := commandRunner(ctx, "launchctl", "start", name); err != nil {
			return mapCommandError(ctx, "执行 macOS 服务重启失败", "launchctl", err)
		}
		return nil
	case ActionEnable, ActionDisable:
		return errs.NewWithSuggestion(
			errs.ExitExecutionFailed,
			errs.CodePlatformUnsupported,
			"macOS 当前未接入 service enable/disable",
			"请使用 launchctl/load/unload 在系统层处理自启配置",
		)
	default:
		return errs.InvalidArgument("不支持的服务动作")
	}
}

func collectWindowsServices(ctx context.Context) ([]ServiceEntry, []string, error) {
	// 通过 CIM 一次性读取 Name/State/StartMode/PID，避免逐服务调用带来的高额开销。
	script := "$ErrorActionPreference='Stop'; Get-CimInstance Win32_Service | Select-Object Name,DisplayName,State,StartMode,ProcessId,Description | ConvertTo-Json -Compress"
	output, err := commandRunner(ctx, "powershell", "-NoProfile", "-Command", script)
	if err != nil {
		return nil, nil, mapCommandError(ctx, "读取 Windows 服务列表失败", "powershell", err)
	}

	type windowsServiceRecord struct {
		Name        string `json:"Name"`
		DisplayName string `json:"DisplayName"`
		State       string `json:"State"`
		StartMode   string `json:"StartMode"`
		ProcessID   int32  `json:"ProcessId"`
		Description string `json:"Description"`
	}

	records := make([]windowsServiceRecord, 0, 64)
	if jsonErr := json.Unmarshal(output, &records); jsonErr != nil {
		var single windowsServiceRecord
		if singleErr := json.Unmarshal(output, &single); singleErr != nil {
			return nil, nil, errs.ExecutionFailed("解析 Windows 服务输出失败", jsonErr)
		}
		if strings.TrimSpace(single.Name) != "" {
			records = append(records, single)
		}
	}

	entries := make([]ServiceEntry, 0, len(records))
	for _, item := range records {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		entries = append(entries, ServiceEntry{
			Name:        name,
			DisplayName: strings.TrimSpace(item.DisplayName),
			State:       normalizeWindowsState(item.State),
			Startup:     normalizeWindowsStartup(item.StartMode),
			PID:         item.ProcessID,
			Description: strings.TrimSpace(item.Description),
			Platform:    runtimeName,
		})
	}
	sortServices(entries)
	return entries, nil, nil
}

func collectLinuxServices(ctx context.Context) ([]ServiceEntry, []string, error) {
	unitOutput, err := commandRunner(
		ctx,
		"systemctl",
		"list-units",
		"--type=service",
		"--all",
		"--no-legend",
		"--no-pager",
		"--plain",
	)
	if err != nil {
		return nil, nil, mapCommandError(ctx, "读取 Linux 服务列表失败", "systemctl", err)
	}

	entries := parseSystemdListUnits(unitOutput)
	warnings := make([]string, 0, 1)

	unitFileOutput, unitFileErr := commandRunner(
		ctx,
		"systemctl",
		"list-unit-files",
		"--type=service",
		"--no-legend",
		"--no-pager",
		"--plain",
	)
	if unitFileErr != nil {
		warnings = append(warnings, "读取服务开机自启状态失败，startup 已按 unknown 填充")
	} else {
		startupMap := parseSystemdUnitFiles(unitFileOutput)
		for idx := range entries {
			if startup, ok := startupMap[entries[idx].Name]; ok {
				entries[idx].Startup = startup
			}
		}
	}

	sortServices(entries)
	return entries, warnings, nil
}

func collectDarwinServices(ctx context.Context) ([]ServiceEntry, []string, error) {
	output, err := commandRunner(ctx, "launchctl", "list")
	if err != nil {
		return nil, nil, mapCommandError(ctx, "读取 macOS 服务列表失败", "launchctl", err)
	}

	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	entries := make([]ServiceEntry, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "pid") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		pid, _ := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 32)
		statusCode, _ := strconv.Atoi(strings.TrimSpace(fields[1]))
		label := strings.TrimSpace(fields[2])
		if label == "" {
			continue
		}

		state := "unknown"
		switch {
		case pid > 0:
			state = "running"
		case statusCode == 0:
			state = "stopped"
		default:
			state = "failed"
		}

		entries = append(entries, ServiceEntry{
			Name:        label,
			DisplayName: label,
			State:       state,
			Startup:     "unknown",
			PID:         int32(pid),
			Platform:    runtimeName,
		})
	}
	sortServices(entries)
	return entries, nil, nil
}

// enrichServiceDetail 在 `service check --detail` 场景下补充平台可得的细节信息。
// 当细节采集失败时只记录 warning，保持 check 命令可用。
func enrichServiceDetail(ctx context.Context, service *ServiceEntry) []string {
	if service == nil || strings.TrimSpace(service.Name) == "" {
		return nil
	}
	switch runtimeName {
	case "linux":
		return enrichLinuxServiceDetail(ctx, service)
	case "darwin":
		return []string{"macOS 平台暂未接入 service check --detail 额外字段"}
	default:
		return nil
	}
}

func enrichLinuxServiceDetail(ctx context.Context, service *ServiceEntry) []string {
	output, err := commandRunner(
		ctx,
		"systemctl",
		"show",
		service.Name,
		"--property=Description,MainPID,UnitFileState,ActiveState,SubState",
		"--no-pager",
	)
	if err != nil {
		return []string{fmt.Sprintf("读取 %s 详情失败，已降级为基础字段", service.Name)}
	}

	values := parseKeyValueLines(output)
	if desc := strings.TrimSpace(values["Description"]); desc != "" {
		service.Description = desc
	}
	if pidRaw := strings.TrimSpace(values["MainPID"]); pidRaw != "" {
		if pid, parseErr := strconv.ParseInt(pidRaw, 10, 32); parseErr == nil && pid > 0 {
			service.PID = int32(pid)
		}
	}
	if startup := normalizeSystemdStartup(values["UnitFileState"]); startup != "" {
		service.Startup = startup
	}
	if active := strings.TrimSpace(values["ActiveState"]); active != "" {
		service.State = normalizeSystemdState(active, values["SubState"])
	}
	return nil
}

func verifyActionOutcome(action Action, before ServiceEntry, after ServiceEntry, checkErr error) bool {
	if checkErr != nil {
		return false
	}
	switch action {
	case ActionStart, ActionRestart:
		return after.State == "running"
	case ActionStop:
		return after.State != "running"
	case ActionEnable:
		return after.Startup == "auto"
	case ActionDisable:
		return after.Startup == "disabled"
	default:
		return false
	}
}

func actionSummary(action Action, success bool, before ServiceEntry, after ServiceEntry) string {
	if !success {
		return fmt.Sprintf("服务动作 %s 已执行，但结果未通过校验", action)
	}
	switch action {
	case ActionStart:
		return fmt.Sprintf("服务已启动（%s -> %s）", before.State, after.State)
	case ActionStop:
		return fmt.Sprintf("服务已停止（%s -> %s）", before.State, after.State)
	case ActionRestart:
		return fmt.Sprintf("服务已重启（%s -> %s）", before.State, after.State)
	case ActionEnable:
		return fmt.Sprintf("服务已启用自启（%s -> %s）", before.Startup, after.Startup)
	case ActionDisable:
		return fmt.Sprintf("服务已禁用自启（%s -> %s）", before.Startup, after.Startup)
	default:
		return "服务动作执行完成"
	}
}

func filterServices(
	services []ServiceEntry,
	stateSet map[string]struct{},
	startupSet map[string]struct{},
	nameFilter string,
) []ServiceEntry {
	if len(services) == 0 {
		return []ServiceEntry{}
	}

	filtered := make([]ServiceEntry, 0, len(services))
	for _, item := range services {
		if len(stateSet) > 0 {
			if _, ok := stateSet[item.State]; !ok {
				continue
			}
		}
		if len(startupSet) > 0 {
			if _, ok := startupSet[item.Startup]; !ok {
				continue
			}
		}
		if nameFilter != "" {
			name := strings.ToLower(item.Name)
			display := strings.ToLower(item.DisplayName)
			if !strings.Contains(name, nameFilter) && !strings.Contains(display, nameFilter) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	sortServices(filtered)
	return filtered
}

func filterCheckTargets(services []ServiceEntry, target string, all bool) []ServiceEntry {
	if len(services) == 0 {
		return []ServiceEntry{}
	}
	target = strings.ToLower(strings.TrimSpace(target))
	matches := make([]ServiceEntry, 0, 4)
	for _, item := range services {
		name := strings.ToLower(strings.TrimSpace(item.Name))
		display := strings.ToLower(strings.TrimSpace(item.DisplayName))
		if all {
			if strings.Contains(name, target) || strings.Contains(display, target) {
				matches = append(matches, item)
			}
			continue
		}

		if name == target || display == target {
			matches = append(matches, item)
			continue
		}
		// Linux 常见输入是 `ssh`，系统服务名可能是 `ssh.service`，这里做一层协议友好兜底。
		if name == target+".service" || strings.TrimSuffix(name, ".service") == target {
			matches = append(matches, item)
		}
	}
	sortServices(matches)
	return matches
}

func parseSystemdListUnits(output []byte) []ServiceEntry {
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	entries := make([]ServiceEntry, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		name := strings.TrimSpace(fields[0])
		activeState := strings.TrimSpace(fields[2])
		subState := strings.TrimSpace(fields[3])
		description := ""
		if len(fields) > 4 {
			description = strings.TrimSpace(strings.Join(fields[4:], " "))
		}

		entries = append(entries, ServiceEntry{
			Name:        name,
			DisplayName: name,
			State:       normalizeSystemdState(activeState, subState),
			Startup:     "unknown",
			Description: description,
			Platform:    runtimeName,
		})
	}
	return entries
}

func parseSystemdUnitFiles(output []byte) map[string]string {
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	result := make(map[string]string, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		state := strings.TrimSpace(fields[1])
		if name == "" {
			continue
		}
		result[name] = normalizeSystemdStartup(state)
	}
	return result
}

func normalizeWindowsState(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "running":
		return "running"
	case "stopped":
		return "stopped"
	case "start pending", "stop pending", "paused", "pause pending", "continue pending":
		return "pending"
	case "failed":
		return "failed"
	default:
		return "unknown"
	}
}

func normalizeSystemdState(activeState string, subState string) string {
	active := strings.ToLower(strings.TrimSpace(activeState))
	sub := strings.ToLower(strings.TrimSpace(subState))
	switch active {
	case "active":
		if sub == "running" || sub == "listening" || sub == "exited" {
			return "running"
		}
		return "pending"
	case "inactive":
		return "stopped"
	case "failed":
		return "failed"
	case "activating", "deactivating", "reloading":
		return "pending"
	default:
		return "unknown"
	}
}

func normalizeWindowsStartup(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "auto":
		return "auto"
	case "manual":
		return "manual"
	case "disabled":
		return "disabled"
	default:
		return "unknown"
	}
}

func normalizeSystemdStartup(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "enabled", "enabled-runtime", "linked", "linked-runtime", "alias":
		return "auto"
	case "disabled", "masked":
		return "disabled"
	case "static", "indirect", "generated", "transient":
		return "manual"
	default:
		return "unknown"
	}
}

func parseFilterSet(raw string, allowed map[string]struct{}, flagName string) (map[string]struct{}, []string, error) {
	items := splitCSV(raw)
	if len(items) == 0 {
		return nil, nil, nil
	}

	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.ToLower(strings.TrimSpace(item))
		if _, ok := allowed[value]; !ok {
			return nil, nil, errs.InvalidArgument(fmt.Sprintf("%s 包含不支持取值: %s", flagName, item))
		}
		set[value] = struct{}{}
	}

	filter := make([]string, 0, len(set))
	for item := range set {
		filter = append(filter, item)
	}
	sort.Strings(filter)
	return set, filter, nil
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func escapeSingleQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func parseKeyValueLines(output []byte) map[string]string {
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	result := make(map[string]string, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		result[key] = strings.TrimSpace(value)
	}
	return result
}

func dedupeSortedWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(warnings))
	for _, item := range warnings {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		set[item] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	result := make([]string, 0, len(set))
	for item := range set {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func sortServices(items []ServiceEntry) {
	sort.Slice(items, func(i int, j int) bool {
		left := strings.ToLower(strings.TrimSpace(items[i].Name))
		right := strings.ToLower(strings.TrimSpace(items[j].Name))
		if left == right {
			return items[i].State < items[j].State
		}
		return left < right
	})
}

func defaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, err
	}
	return output, nil
}

func mapCommandError(ctx context.Context, message string, command string, err error) error {
	if err == nil {
		return nil
	}
	if timeoutErr := mapContextError(ctx, message); timeoutErr != nil {
		return timeoutErr
	}
	if isCommandNotFound(err) {
		return errs.NewWithSuggestion(
			errs.ExitExecutionFailed,
			errs.CodeDependencyMissing,
			fmt.Sprintf("未找到服务管理命令: %s", command),
			"请确认系统已安装对应服务管理工具，或在支持平台重试",
		)
	}

	text := strings.ToLower(err.Error())
	if strings.Contains(text, "permission denied") ||
		strings.Contains(text, "access denied") ||
		strings.Contains(text, "operation not permitted") {
		return errs.PermissionDenied(message, "请提升权限后重试")
	}
	return errs.ExecutionFailed(message, err)
}

func mapContextError(ctx context.Context, message string) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "命令执行超时", "请调大 --timeout 后重试")
		case errors.Is(err, context.Canceled):
			return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
		default:
			return errs.ExecutionFailed(message, err)
		}
	}
	return nil
}

func isCommandNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr, exec.ErrNotFound) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "executable file not found") ||
		strings.Contains(text, "is not recognized as an internal or external command")
}

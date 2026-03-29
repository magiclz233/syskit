package startup

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syskit/internal/errs"
)

var runtimeName = runtime.GOOS
var resolveStartupDirs = startupDirsForPlatform

// ListItems 采集并过滤启动项。
func ListItems(ctx context.Context, opts ListOptions) (*ListResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	items, warnings, err := collectItems(ctx)
	if err != nil {
		return nil, err
	}

	userFilter := strings.ToLower(strings.TrimSpace(opts.User))
	filtered := make([]Item, 0, len(items))
	for _, item := range items {
		if opts.OnlyRisk && !item.Risk {
			continue
		}
		if userFilter != "" && !strings.Contains(strings.ToLower(item.User), userFilter) {
			continue
		}
		filtered = append(filtered, item)
	}

	return &ListResult{
		Platform: runtimeName,
		OnlyRisk: opts.OnlyRisk,
		User:     strings.TrimSpace(opts.User),
		Total:    len(filtered),
		Items:    filtered,
		Warnings: dedupeWarnings(warnings),
	}, nil
}

// ParseAction 解析启动项动作。
func ParseAction(raw string) (Action, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch Action(value) {
	case ActionEnable, ActionDisable:
		return Action(value), nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("不支持的启动项动作: %s", raw))
	}
}

// BuildActionPlan 生成启动项写操作计划。
func BuildActionPlan(ctx context.Context, action Action, id string) (*ActionPlan, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	action, err := ParseAction(string(action))
	if err != nil {
		return nil, err
	}

	targetID := strings.TrimSpace(id)
	if targetID == "" {
		return nil, errs.InvalidArgument("启动项 ID 不能为空")
	}

	listResult, err := ListItems(ctx, ListOptions{})
	if err != nil {
		return nil, err
	}
	item, found := findByID(listResult.Items, targetID)

	plan := &ActionPlan{
		Action:   action,
		ID:       targetID,
		Platform: runtimeName,
		Found:    found,
		Steps: []string{
			"读取当前启动项状态",
		},
		Warnings: append([]string{}, listResult.Warnings...),
	}

	if found {
		plan.Current = item
	} else {
		plan.Warnings = append(plan.Warnings, "未找到目标启动项，真实执行预计会失败")
	}

	switch action {
	case ActionEnable:
		plan.Steps = append(plan.Steps, "启用启动项（恢复到可执行位置）")
	case ActionDisable:
		plan.Steps = append(plan.Steps, "禁用启动项（重命名为 .disabled）")
	}
	plan.Steps = append(plan.Steps, "重新读取状态并校验执行结果")
	plan.Warnings = dedupeWarnings(plan.Warnings)
	return plan, nil
}

// ExecuteAction 执行启动项写操作。
func ExecuteAction(ctx context.Context, plan *ActionPlan) (*ActionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if plan == nil {
		return nil, errs.InvalidArgument("启动项计划不能为空")
	}
	if !plan.Found {
		return nil, errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, "未找到启动项: "+plan.ID)
	}

	before := plan.Current
	after := before
	warnings := append([]string{}, plan.Warnings...)

	switch plan.Action {
	case ActionDisable:
		if !before.Enabled {
			return &ActionResult{
				Action:   plan.Action,
				ID:       plan.ID,
				Platform: plan.Platform,
				Applied:  true,
				Success:  true,
				Summary:  "启动项已是禁用状态，无需重复执行",
				Before:   before,
				After:    after,
				Warnings: dedupeWarnings(warnings),
			}, nil
		}
		newPath := before.SourcePath + ".disabled"
		if err := os.Rename(before.SourcePath, newPath); err != nil {
			return nil, mapActionError("禁用启动项失败", err)
		}
		after.Enabled = false
		after.SourcePath = newPath
		after.ID = buildID(newPath)
	case ActionEnable:
		if before.Enabled {
			return &ActionResult{
				Action:   plan.Action,
				ID:       plan.ID,
				Platform: plan.Platform,
				Applied:  true,
				Success:  true,
				Summary:  "启动项已是启用状态，无需重复执行",
				Before:   before,
				After:    after,
				Warnings: dedupeWarnings(warnings),
			}, nil
		}
		if !strings.HasSuffix(before.SourcePath, ".disabled") {
			return nil, errs.NewWithSuggestion(
				errs.ExitExecutionFailed,
				errs.CodeExecutionFailed,
				"当前启动项无法自动启用：缺少 .disabled 后缀",
				"请手动恢复原始文件名后重试",
			)
		}
		newPath := strings.TrimSuffix(before.SourcePath, ".disabled")
		if err := os.Rename(before.SourcePath, newPath); err != nil {
			return nil, mapActionError("启用启动项失败", err)
		}
		after.Enabled = true
		after.SourcePath = newPath
		after.ID = buildID(newPath)
	default:
		return nil, errs.InvalidArgument("不支持的启动项动作")
	}

	after.Risk, after.RiskReason = evaluateRisk(after)
	return &ActionResult{
		Action:   plan.Action,
		ID:       plan.ID,
		Platform: plan.Platform,
		Applied:  true,
		Success:  true,
		Summary:  fmt.Sprintf("启动项动作执行成功（%s）", plan.Action),
		Before:   before,
		After:    after,
		Warnings: dedupeWarnings(warnings),
	}, nil
}

func collectItems(ctx context.Context) ([]Item, []string, error) {
	dirs := resolveStartupDirs(runtimeName)
	if len(dirs) == 0 {
		return nil, []string{"当前平台未配置启动项目录，已降级为空结果"}, nil
	}

	items := make([]Item, 0, 32)
	warnings := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		select {
		case <-ctx.Done():
			return nil, nil, mapActionError("启动项采集被取消", ctx.Err())
		default:
		}

		entries, err := os.ReadDir(dir.path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				warnings = append(warnings, "目录不存在，已跳过: "+dir.path)
				continue
			}
			if isPermissionError(err) {
				warnings = append(warnings, "目录无权限访问，已跳过: "+dir.path)
				continue
			}
			return nil, nil, errs.ExecutionFailed("读取启动项目录失败: "+dir.path, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.TrimSpace(entry.Name())
			if name == "" {
				continue
			}
			// 非常规文件（如锁文件）不参与启动项管理，避免误操作。
			if !isStartupFile(name) {
				continue
			}

			path := filepath.Join(dir.path, name)
			item := Item{
				ID:         buildID(path),
				Name:       normalizeItemName(name),
				Command:    inferCommand(path),
				Location:   dir.location,
				User:       dir.user,
				Enabled:    !strings.HasSuffix(strings.ToLower(name), ".disabled"),
				Platform:   runtimeName,
				SourcePath: path,
			}
			item.Risk, item.RiskReason = evaluateRisk(item)
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i int, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].SourcePath < items[j].SourcePath
	})
	return items, dedupeWarnings(warnings), nil
}

type startupDir struct {
	path     string
	location string
	user     string
}

func startupDirsForPlatform(goos string) []startupDir {
	userName := currentUserName()
	home, _ := os.UserHomeDir()
	switch goos {
	case "windows":
		appData := strings.TrimSpace(os.Getenv("APPDATA"))
		programData := strings.TrimSpace(os.Getenv("ProgramData"))
		dirs := []startupDir{
			{
				path:     filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup"),
				location: "user_startup",
				user:     userName,
			},
			{
				path:     filepath.Join(programData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup"),
				location: "system_startup",
				user:     "system",
			},
		}
		return cleanStartupDirs(dirs)
	case "darwin":
		return cleanStartupDirs([]startupDir{
			{
				path:     filepath.Join(home, "Library", "LaunchAgents"),
				location: "launch_agents",
				user:     userName,
			},
			{
				path:     "/Library/LaunchAgents",
				location: "launch_agents",
				user:     "system",
			},
		})
	default:
		return cleanStartupDirs([]startupDir{
			{
				path:     filepath.Join(home, ".config", "autostart"),
				location: "autostart",
				user:     userName,
			},
			{
				path:     "/etc/xdg/autostart",
				location: "autostart",
				user:     "system",
			},
		})
	}
}

func cleanStartupDirs(items []startupDir) []startupDir {
	seen := make(map[string]struct{}, len(items))
	result := make([]startupDir, 0, len(items))
	for _, item := range items {
		path := strings.TrimSpace(item.path)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		item.path = path
		result = append(result, item)
	}
	return result
}

func findByID(items []Item, id string) (Item, bool) {
	target := strings.TrimSpace(id)
	for _, item := range items {
		if item.ID == target {
			return item, true
		}
	}
	return Item{}, false
}

func evaluateRisk(item Item) (bool, string) {
	pathText := strings.ToLower(item.SourcePath)
	commandText := strings.ToLower(item.Command)

	if strings.Contains(pathText, `\temp\`) || strings.Contains(pathText, "/tmp/") {
		return true, "启动项位于临时目录"
	}
	if strings.Contains(pathText, "appdata") && strings.Contains(pathText, "temp") {
		return true, "启动项位于用户临时缓存目录"
	}
	if strings.Contains(commandText, " -encodedcommand") {
		return true, "命令使用了编码脚本参数"
	}
	for _, keyword := range []string{"powershell", "cmd.exe /c", "curl ", "wget ", "bitsadmin"} {
		if strings.Contains(commandText, keyword) {
			return true, "启动项命令包含高风险执行器"
		}
	}
	return false, ""
}

func buildID(path string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(path))))
	return fmt.Sprintf("stp-%08x", h.Sum32())
}

func isStartupFile(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasSuffix(lower, ".desktop"),
		strings.HasSuffix(lower, ".plist"),
		strings.HasSuffix(lower, ".lnk"),
		strings.HasSuffix(lower, ".url"),
		strings.HasSuffix(lower, ".bat"),
		strings.HasSuffix(lower, ".cmd"),
		strings.HasSuffix(lower, ".ps1"),
		strings.HasSuffix(lower, ".service"),
		strings.HasSuffix(lower, ".disabled"):
		return true
	default:
		return false
	}
}

func normalizeItemName(name string) string {
	value := strings.TrimSpace(name)
	lower := strings.ToLower(value)
	if strings.HasSuffix(lower, ".disabled") {
		value = value[:len(value)-len(".disabled")]
		lower = strings.ToLower(value)
	}
	for _, suffix := range []string{".desktop", ".plist", ".lnk", ".url", ".bat", ".cmd", ".ps1", ".service"} {
		if strings.HasSuffix(lower, suffix) {
			value = value[:len(value)-len(suffix)]
			break
		}
	}
	return strings.TrimSpace(value)
}

func inferCommand(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".desktop") {
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Exec=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "Exec="))
			}
		}
	}
	return ""
}

func currentUserName() string {
	for _, key := range []string{"USERNAME", "USER"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return "unknown"
}

func mapActionError(message string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "启动项操作超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "启动项操作已取消")
	}
	if isPermissionError(err) {
		return errs.PermissionDenied(message, "请提升权限后重试")
	}
	return errs.ExecutionFailed(message, err)
}

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "permission denied") ||
		strings.Contains(text, "access denied") ||
		strings.Contains(text, "operation not permitted")
}

func dedupeWarnings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
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

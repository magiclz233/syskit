package fixrun

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	cleanupcollector "syskit/internal/collectors/cleanup"
	"syskit/internal/config"
	"syskit/internal/errs"
	"time"
)

// ParseOnFail 解析失败策略。
func ParseOnFail(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "stop", nil
	}
	switch value {
	case "stop", "continue":
		return value, nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--on-fail 仅支持 stop/continue，当前为: %s", raw))
	}
}

// BuildPlan 生成剧本执行计划。
func BuildPlan(script string, apply bool, onFail string) (*Plan, error) {
	onFail, err := ParseOnFail(onFail)
	if err != nil {
		return nil, err
	}
	script = strings.TrimSpace(script)
	if script == "" {
		return nil, errs.InvalidArgument("script 不能为空")
	}

	parts := splitScripts(script)
	steps := make([]StepPlan, 0, len(parts))
	for _, part := range parts {
		steps = append(steps, StepPlan{
			Name:    part,
			Builtin: isBuiltin(part),
			Action:  actionText(part),
		})
	}
	return &Plan{
		Script: script,
		Apply:  apply,
		OnFail: onFail,
		Steps:  steps,
	}, nil
}

// Execute 执行剧本计划。
func Execute(ctx context.Context, plan *Plan, cfg *config.Config) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if plan == nil {
		return nil, errs.InvalidArgument("fix run 计划不能为空")
	}
	if cfg == nil {
		return nil, errs.InvalidArgument("配置不能为空")
	}

	result := &Result{
		Applied:   plan.Apply,
		Success:   true,
		OnFail:    plan.OnFail,
		StepCount: len(plan.Steps),
		Steps:     make([]StepResult, 0, len(plan.Steps)),
	}

	for _, step := range plan.Steps {
		if err := ctx.Err(); err != nil {
			return nil, mapContextError(err)
		}
		item := runStep(ctx, step, plan.Apply, cfg)
		result.Steps = append(result.Steps, item)
		if item.Success {
			result.Succeeded++
			continue
		}
		result.Failed++
		result.Success = false
		if plan.OnFail == "stop" {
			break
		}
	}

	switch {
	case result.Success:
		result.Summary = fmt.Sprintf("剧本执行完成（步骤 %d，全部成功）", result.StepCount)
	case result.Failed > 0 && result.Succeeded > 0:
		result.Summary = fmt.Sprintf("剧本执行完成（成功 %d，失败 %d）", result.Succeeded, result.Failed)
	default:
		result.Summary = "剧本执行失败"
	}
	return result, nil
}

func runStep(ctx context.Context, step StepPlan, apply bool, cfg *config.Config) StepResult {
	startedAt := time.Now().UTC()
	item := StepResult{
		Name:      step.Name,
		Builtin:   step.Builtin,
		Applied:   apply,
		Success:   true,
		StartedAt: startedAt,
	}
	defer func() {
		item.EndedAt = time.Now().UTC()
		item.DurationMs = item.EndedAt.Sub(item.StartedAt).Milliseconds()
	}()

	if !apply {
		item.Summary = "dry-run: 已生成步骤计划"
		return item
	}
	if step.Builtin {
		if err := runBuiltin(ctx, step.Name, cfg); err != nil {
			item.Success = false
			item.Summary = "执行失败"
			item.Error = errs.Message(err)
			return item
		}
		item.Summary = "执行成功"
		return item
	}

	output, err := runCustomScript(ctx, step.Name)
	if err != nil {
		item.Success = false
		item.Summary = "执行失败"
		item.Output = compactOutput(string(output))
		item.Error = errs.Message(err)
		return item
	}
	item.Summary = "执行成功"
	item.Output = compactOutput(string(output))
	return item
}

func runBuiltin(ctx context.Context, name string, cfg *config.Config) error {
	now := time.Now()
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "cleanup-temp":
		return runCleanupTarget(ctx, cfg, cleanupcollector.TargetTemp, now)
	case "cleanup-logs":
		return runCleanupTarget(ctx, cfg, cleanupcollector.TargetLogs, now)
	case "cleanup-cache":
		return runCleanupTarget(ctx, cfg, cleanupcollector.TargetCache, now)
	default:
		return errs.InvalidArgument("不支持的内置剧本: " + name)
	}
}

func runCleanupTarget(ctx context.Context, cfg *config.Config, target cleanupcollector.Target, now time.Time) error {
	plan, err := cleanupcollector.BuildPlan(ctx, cleanupcollector.PlanOptions{
		Targets:        []cleanupcollector.Target{target},
		OlderThan:      7 * 24 * time.Hour,
		StorageDataDir: cfg.Storage.DataDir,
		LoggingFile:    cfg.Logging.File,
		Now:            now,
	})
	if err != nil {
		return err
	}
	_, err = cleanupcollector.ApplyPlan(ctx, plan)
	return err
}

func runCustomScript(ctx context.Context, script string) ([]byte, error) {
	args := strings.Fields(strings.TrimSpace(script))
	if len(args) == 0 {
		return nil, errs.InvalidArgument("自定义脚本不能为空")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isPermissionErr(err) {
			return output, errs.PermissionDenied("脚本执行权限不足", "请提升权限后重试")
		}
		return output, errs.ExecutionFailed("执行自定义剧本失败", err)
	}
	return output, nil
}

func splitScripts(raw string) []string {
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

func isBuiltin(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "cleanup-temp", "cleanup-logs", "cleanup-cache":
		return true
	default:
		return false
	}
}

func actionText(name string) string {
	if isBuiltin(name) {
		return "执行内置清理剧本"
	}
	return "执行自定义脚本"
}

func compactOutput(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= 1000 {
		return text
	}
	return text[:1000] + "...(truncated)"
}

func mapContextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "fix run 执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "fix run 已取消")
	}
	return errs.ExecutionFailed("fix run 执行失败", err)
}

func isPermissionErr(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "permission denied") ||
		strings.Contains(text, "access denied") ||
		strings.Contains(text, "operation not permitted")
}

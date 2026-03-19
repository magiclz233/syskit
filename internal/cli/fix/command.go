// Package fix 负责修复和清理命令组。
package fix

import (
	"context"
	"fmt"
	"strings"
	"syskit/internal/audit"
	"syskit/internal/cliutil"
	cleanupcollector "syskit/internal/collectors/cleanup"
	"syskit/internal/config"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type cleanupOptions struct {
	target    string
	olderThan string
}

// cleanupOutputData 是 `fix cleanup` 的统一输出结构。
type cleanupOutputData struct {
	Mode   string                        `json:"mode"`
	Apply  bool                          `json:"apply"`
	Plan   *cleanupcollector.Plan        `json:"plan"`
	Result *cleanupcollector.ApplyResult `json:"result,omitempty"`
}

// NewCommand 创建 `fix` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "修复与清理命令",
		Long: "fix 提供当前版本允许的受控修复入口。" +
			"\n\nP0 当前仅开放 `fix cleanup`，其他修复剧本入口保留为占位命令。",
		Example: "  syskit fix cleanup\n" +
			"  syskit fix cleanup --target temp,logs\n" +
			"  syskit fix cleanup --apply",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newCleanupCommand())
	return cmd
}

func newCleanupCommand() *cobra.Command {
	opts := &cleanupOptions{
		target:    "temp,logs,cache",
		olderThan: "7d",
	}

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "执行 temp/logs/cache 清理",
		Long: "fix cleanup 会先扫描 temp/logs/cache 候选文件并生成清理计划，默认只做 dry-run。" +
			"\n\n传入 `--apply` 后会真实删除文件，并记录审计日志。",
		Example: "  syskit fix cleanup\n" +
			"  syskit fix cleanup --target temp --older-than 72h\n" +
			"  syskit fix cleanup --apply --older-than 7d",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCleanup(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.target, "target", "temp,logs,cache", "清理目标: temp,logs,cache（逗号分隔）")
	flags.StringVar(&opts.olderThan, "older-than", "7d", "仅清理超过该时长的文件（示例: 72h, 7d, 2w）")
	return cmd
}

func runCleanup(cmd *cobra.Command, opts *cleanupOptions) error {
	startedAt := time.Now()

	targets, err := cleanupcollector.ParseTargets(opts.target)
	if err != nil {
		return err
	}
	olderThan, err := cleanupcollector.ParseOlderThan(opts.olderThan)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return err
	}

	plan, err := cleanupcollector.BuildPlan(ctx, cleanupcollector.PlanOptions{
		Targets:        targets,
		OlderThan:      olderThan,
		StorageDataDir: cfg.Storage.DataDir,
		LoggingFile:    cfg.Logging.File,
		Now:            time.Now(),
	})
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	if !apply {
		data := &cleanupOutputData{
			Mode:  "dry-run",
			Apply: false,
			Plan:  plan,
		}
		msg := fmt.Sprintf("清理计划已生成（候选 %d 项）", plan.CandidateCount)
		result := output.NewSuccessResult(msg, data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newCleanupPresenter(data))
	}

	execResult, err := cleanupcollector.ApplyPlan(ctx, plan)
	if err != nil {
		auditErr := writeAuditEvent(cmd, ctx, audit.Event{
			Command:    cmd.CommandPath(),
			Action:     "fix.cleanup",
			Target:     strings.Join(targetNames(targets), ","),
			Before:     plan,
			Result:     "failed",
			ErrorMsg:   errs.Message(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Metadata: map[string]any{
				"apply":      true,
				"older_than": opts.olderThan,
			},
		})
		if auditErr != nil {
			return errs.ExecutionFailed(
				fmt.Sprintf("fix cleanup 执行失败且审计写入失败: %s", errs.Message(err)),
				auditErr,
			)
		}
		return err
	}
	data := &cleanupOutputData{
		Mode:   "apply",
		Apply:  true,
		Plan:   plan,
		Result: execResult,
	}

	msg := fmt.Sprintf("清理执行完成（删除 %d 项）", execResult.DeletedCount)
	if execResult.RemainingCount > 0 || len(execResult.Failed) > 0 {
		msg = fmt.Sprintf("清理执行完成（删除 %d 项，失败 %d 项，剩余 %d 项）", execResult.DeletedCount, len(execResult.Failed), execResult.RemainingCount)
	}
	if err := writeAuditEvent(cmd, ctx, audit.Event{
		Command:    cmd.CommandPath(),
		Action:     "fix.cleanup",
		Target:     strings.Join(targetNames(targets), ","),
		Before:     plan,
		After:      execResult,
		Result:     "success",
		DurationMs: time.Since(startedAt).Milliseconds(),
		Metadata: map[string]any{
			"apply":           true,
			"older_than":      opts.olderThan,
			"deleted_count":   execResult.DeletedCount,
			"remaining_count": execResult.RemainingCount,
		},
	}); err != nil {
		return err
	}
	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newCleanupPresenter(data))
}

func loadRuntimeConfig(cmd *cobra.Command) (*config.Config, error) {
	loadResult, err := config.Load(config.LoadOptions{
		ExplicitPath: strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "config")),
	})
	if err != nil {
		return nil, err
	}
	return loadResult.Config, nil
}

func writeAuditEvent(cmd *cobra.Command, ctx context.Context, event audit.Event) error {
	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return err
	}
	logger, err := audit.NewLogger(cfg.Storage.DataDir)
	if err != nil {
		return err
	}
	return logger.Log(ctx, event)
}

func targetNames(targets []cleanupcollector.Target) []string {
	result := make([]string, 0, len(targets))
	for _, target := range targets {
		result = append(result, string(target))
	}
	return result
}

// Package fix 负责修复和清理命令组。
package fix

import (
	"fmt"
	"strings"
	"syskit/internal/cliutil"
	cleanupcollector "syskit/internal/collectors/cleanup"
	"syskit/internal/config"
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
		Args:  cobra.NoArgs,
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
		Args:  cobra.NoArgs,
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

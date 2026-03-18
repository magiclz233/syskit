// Package proc 负责进程查询和管理命令组。
package proc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syskit/internal/audit"
	"syskit/internal/cliutil"
	proccollector "syskit/internal/collectors/proc"
	"syskit/internal/config"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type topOptions struct {
	by    string
	topN  int
	user  string
	name  string
	watch bool
}

type treeOptions struct {
	detail bool
	full   bool
}

type infoOptions struct {
	env bool
}

type killOptions struct {
	force bool
	tree  bool
}

// killOutputData 表示 `proc kill` 的统一输出数据。
type killOutputData struct {
	Mode   string                    `json:"mode"`
	Apply  bool                      `json:"apply"`
	Plan   *proccollector.KillPlan   `json:"plan"`
	Result *proccollector.KillResult `json:"result,omitempty"`
}

// NewCommand 创建 `proc` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proc",
		Short: "进程查询与管理",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newTopCommand(),
		newTreeCommand(),
		newInfoCommand(),
		newKillCommand(),
	)

	return cmd
}

func newTopCommand() *cobra.Command {
	opts := &topOptions{
		by:   string(proccollector.SortByCPU),
		topN: 20,
	}

	cmd := &cobra.Command{
		Use:   "top",
		Short: "查看进程资源排行",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTop(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.by, "by", string(proccollector.SortByCPU), "排序维度: cpu/mem/io/fd")
	flags.IntVar(&opts.topN, "top", 20, "显示 Top N 进程")
	flags.StringVar(&opts.user, "user", "", "按用户名过滤（模糊匹配）")
	flags.StringVar(&opts.name, "name", "", "按进程名或命令行过滤（模糊匹配）")
	flags.BoolVar(&opts.watch, "watch", false, "持续观察（P0 当前为单次采样）")
	return cmd
}

func newTreeCommand() *cobra.Command {
	opts := &treeOptions{}
	cmd := &cobra.Command{
		Use:   "tree [pid]",
		Short: "查看进程树",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTree(cmd, args, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.detail, "detail", false, "显示用户、命令行和资源字段")
	flags.BoolVar(&opts.full, "full", false, "展示完整层级（默认限制深度）")
	return cmd
}

func newInfoCommand() *cobra.Command {
	opts := &infoOptions{}
	cmd := &cobra.Command{
		Use:   "info <pid>",
		Short: "查看单进程详情",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo(cmd, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.env, "env", false, "包含环境变量输出")
	return cmd
}

func newKillCommand() *cobra.Command {
	opts := &killOptions{}
	cmd := &cobra.Command{
		Use:   "kill <pid>",
		Short: "结束指定进程",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKill(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.force, "force", false, "强制结束进程")
	flags.BoolVar(&opts.tree, "tree", false, "连同子进程一起结束")
	return cmd
}

func runTop(cmd *cobra.Command, opts *topOptions) error {
	startedAt := time.Now()
	by, err := proccollector.ParseSortBy(opts.by)
	if err != nil {
		return err
	}
	if opts.topN <= 0 {
		return errs.InvalidArgument("--top 必须大于 0")
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := proccollector.CollectTop(ctx, proccollector.TopOptions{
		By:    by,
		TopN:  opts.topN,
		User:  opts.user,
		Name:  opts.name,
		Watch: opts.watch,
	})
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("进程排行采集完成", resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newTopPresenter(resultData))
}

func runTree(cmd *cobra.Command, args []string, opts *treeOptions) error {
	startedAt := time.Now()
	rootPID, err := optionalPID(args)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := proccollector.CollectTree(ctx, proccollector.TreeOptions{
		RootPID: rootPID,
		Detail:  opts.detail,
		Full:    opts.full,
	})
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("进程树采集完成", resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newTreePresenter(resultData))
}

func runInfo(cmd *cobra.Command, pidRaw string, opts *infoOptions) error {
	startedAt := time.Now()
	pid, err := parsePID(pidRaw)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := proccollector.CollectInfo(ctx, pid, opts.env)
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("进程详情采集完成", resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newInfoPresenter(resultData))
}

func runKill(cmd *cobra.Command, pidRaw string, opts *killOptions) error {
	startedAt := time.Now()
	pid, err := parsePID(pidRaw)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	plan, err := proccollector.BuildKillPlan(ctx, pid, opts.tree, opts.force)
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	yes := cliutil.ResolveBoolFlag(cmd, "yes")
	if apply && !yes {
		return errs.InvalidArgument("真实执行 proc kill 需要同时传入 --yes")
	}

	if !apply {
		data := &killOutputData{
			Mode:  "dry-run",
			Apply: false,
			Plan:  plan,
		}
		result := output.NewSuccessResult("已生成进程终止计划（dry-run）", data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newKillPresenter(data))
	}

	execResult, err := proccollector.ExecuteKillPlan(ctx, plan)
	if err != nil {
		auditErr := writeAuditEvent(cmd, ctx, audit.Event{
			Command:    cmd.CommandPath(),
			Action:     "proc.kill",
			Target:     fmt.Sprintf("pid:%d", pid),
			Before:     plan,
			Result:     "failed",
			ErrorMsg:   errs.Message(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Metadata: map[string]any{
				"apply": true,
				"force": opts.force,
				"tree":  opts.tree,
			},
		})
		if auditErr != nil {
			return errs.ExecutionFailed(
				fmt.Sprintf("proc kill 执行失败且审计写入失败: %s", errs.Message(err)),
				auditErr,
			)
		}
		return err
	}

	data := &killOutputData{
		Mode:   "apply",
		Apply:  true,
		Plan:   plan,
		Result: execResult,
	}

	msg := "进程终止执行完成"
	if failed := proccollector.CountKillFailures(execResult); failed > 0 {
		msg = fmt.Sprintf("进程终止执行完成（%d 个目标失败）", failed)
	}
	if err := writeAuditEvent(cmd, ctx, audit.Event{
		Command:    cmd.CommandPath(),
		Action:     "proc.kill",
		Target:     fmt.Sprintf("pid:%d", pid),
		Before:     plan,
		After:      execResult,
		Result:     "success",
		DurationMs: time.Since(startedAt).Milliseconds(),
		Metadata: map[string]any{
			"apply":    true,
			"force":    opts.force,
			"tree":     opts.tree,
			"verified": execResult.Verified,
		},
	}); err != nil {
		return err
	}

	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newKillPresenter(data))
}

func parsePID(raw string) (int32, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, errs.InvalidArgument("PID 不能为空")
	}
	value, err := strconv.ParseInt(text, 10, 32)
	if err != nil || value <= 0 {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 PID: %s", raw))
	}
	return int32(value), nil
}

func optionalPID(args []string) (*int32, error) {
	if len(args) == 0 {
		return nil, nil
	}
	pid, err := parsePID(args[0])
	if err != nil {
		return nil, err
	}
	return &pid, nil
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

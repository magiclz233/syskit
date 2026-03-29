// Package startup 负责启动项管理命令组。
package startup

import (
	"context"
	"fmt"
	"strings"
	"syskit/internal/audit"
	"syskit/internal/cliutil"
	startupcollector "syskit/internal/collectors/startup"
	"syskit/internal/config"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type listOptions struct {
	onlyRisk bool
	user     string
}

type actionOutputData struct {
	Mode   string                         `json:"mode"`
	Apply  bool                           `json:"apply"`
	Plan   *startupcollector.ActionPlan   `json:"plan"`
	Result *startupcollector.ActionResult `json:"result,omitempty"`
}

// NewCommand 创建 `startup` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "startup",
		Short: "启动项管理命令",
		Long: "startup 提供启动项扫描和启用/禁用能力。" +
			"\n\nenable/disable 属于危险写操作，默认仅输出 dry-run 计划，真实执行必须显式传入 `--apply --yes`，并记录审计日志。",
		Example: "  syskit startup list\n" +
			"  syskit startup list --only-risk --format json\n" +
			"  syskit startup disable stp-12345678\n" +
			"  syskit startup enable stp-12345678 --apply --yes",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newListCommand(),
		newActionCommand(startupcollector.ActionEnable),
		newActionCommand(startupcollector.ActionDisable),
	)
	return cmd
}

func newListCommand() *cobra.Command {
	opts := &listOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出启动项",
		Long: "startup list 会读取平台启动目录并输出启动项清单。" +
			"\n\n--only-risk 仅显示可疑启动项；--user 可按用户名过滤。",
		Example: "  syskit startup list\n" +
			"  syskit startup list --only-risk\n" +
			"  syskit startup list --user alice --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.onlyRisk, "only-risk", false, "仅显示可疑启动项")
	flags.StringVar(&opts.user, "user", "", "按用户过滤（模糊匹配）")
	return cmd
}

func newActionCommand(action startupcollector.Action) *cobra.Command {
	short := "操作启动项"
	switch action {
	case startupcollector.ActionEnable:
		short = "启用指定启动项"
	case startupcollector.ActionDisable:
		short = "禁用指定启动项"
	}
	cmd := &cobra.Command{
		Use:   string(action) + " <id>",
		Short: short,
		Long: "startup " + string(action) + " 属于危险写操作，默认仅输出 dry-run 计划。" +
			"\n\n真实执行必须显式传入 `--apply --yes`，执行后会写入审计日志。",
		Example: "  syskit startup " + string(action) + " stp-12345678\n" +
			"  syskit startup " + string(action) + " stp-12345678 --apply --yes",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, action, args[0])
		},
	}
	return cmd
}

func runList(cmd *cobra.Command, opts *listOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := startupcollector.ListItems(ctx, startupcollector.ListOptions{
		OnlyRisk: opts.onlyRisk,
		User:     opts.user,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("启动项采集完成，共 %d 条", resultData.Total)
	if resultData.Total == 0 {
		msg = "启动项采集完成，未命中记录"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newListPresenter(resultData))
}

func runAction(cmd *cobra.Command, action startupcollector.Action, id string) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	plan, err := startupcollector.BuildActionPlan(ctx, action, id)
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	yes := cliutil.ResolveBoolFlag(cmd, "yes")
	if apply && !yes {
		return errs.InvalidArgument("真实执行 startup 写操作需要同时传入 --yes")
	}
	if !apply {
		data := &actionOutputData{
			Mode:  "dry-run",
			Apply: false,
			Plan:  plan,
		}
		result := output.NewSuccessResult("启动项计划已生成（dry-run）", data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newActionPresenter(data))
	}

	execResult, err := startupcollector.ExecuteAction(ctx, plan)
	if err != nil {
		auditResult := auditResultFromError(err)
		auditErr := writeAuditEvent(cmd, ctx, audit.Event{
			Command:    cmd.CommandPath(),
			Action:     "startup." + string(action),
			Target:     id,
			Before:     plan,
			Result:     auditResult,
			ErrorMsg:   errs.Message(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Metadata: map[string]any{
				"apply":      true,
				"error_code": errs.ErrorCode(err),
			},
		})
		if auditErr != nil {
			return errs.ExecutionFailed(
				fmt.Sprintf("startup %s 执行失败且审计写入失败: %s", action, errs.Message(err)),
				auditErr,
			)
		}
		return err
	}

	data := &actionOutputData{
		Mode:   "apply",
		Apply:  true,
		Plan:   plan,
		Result: execResult,
	}
	auditResult := "success"
	if !execResult.Success {
		auditResult = "partial"
	}
	if err := writeAuditEvent(cmd, ctx, audit.Event{
		Command:    cmd.CommandPath(),
		Action:     "startup." + string(action),
		Target:     id,
		Before:     plan,
		After:      execResult,
		Result:     auditResult,
		DurationMs: time.Since(startedAt).Milliseconds(),
		Metadata: map[string]any{
			"apply":   true,
			"success": execResult.Success,
		},
	}); err != nil {
		return err
	}

	msg := fmt.Sprintf("启动项动作执行完成（action=%s）", action)
	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newActionPresenter(data))
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

func auditResultFromError(err error) string {
	if errs.Code(err) == errs.ExitPermissionDenied {
		return "skipped"
	}
	return "failed"
}

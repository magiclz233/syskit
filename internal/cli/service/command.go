// Package service 负责服务管理命令组。
package service

import (
	"context"
	"fmt"
	"strings"
	"syskit/internal/audit"
	"syskit/internal/cliutil"
	servicecollector "syskit/internal/collectors/service"
	"syskit/internal/config"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type listOptions struct {
	state   string
	startup string
	name    string
}

type checkOptions struct {
	all    bool
	detail bool
}

type actionOutputData struct {
	Mode   string                         `json:"mode"`
	Apply  bool                           `json:"apply"`
	Plan   *servicecollector.ActionPlan   `json:"plan"`
	Result *servicecollector.ActionResult `json:"result,omitempty"`
}

// NewCommand 创建 `service` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "服务管理命令",
		Long: "service 提供系统服务列表、健康检查和写操作管理能力。" +
			"\n\nstart/stop/restart/enable/disable 默认仅输出 dry-run 计划，真实执行必须显式传入 `--apply --yes`，并写入审计日志。",
		Example: "  syskit service list\n" +
			"  syskit service list --state running --startup auto\n" +
			"  syskit service check ssh\n" +
			"  syskit service check docker --all --detail --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newListCommand(),
		newCheckCommand(),
		newActionCommand(servicecollector.ActionStart),
		newActionCommand(servicecollector.ActionStop),
		newActionCommand(servicecollector.ActionRestart),
		newActionCommand(servicecollector.ActionEnable),
		newActionCommand(servicecollector.ActionDisable),
	)
	return cmd
}

func newListCommand() *cobra.Command {
	opts := &listOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出系统服务",
		Long: "service list 用于查看系统服务清单，可按状态、开机自启类型和名称过滤。" +
			"\n\n当当前平台命令能力受限时，命令会降级为空结果并在 warnings 中说明原因。",
		Example: "  syskit service list\n" +
			"  syskit service list --state running\n" +
			"  syskit service list --startup auto --name docker --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.state, "state", "", "服务状态过滤（逗号分隔）: running/stopped/failed/pending/unknown")
	flags.StringVar(&opts.startup, "startup", "", "启动类型过滤（逗号分隔）: auto/manual/disabled/unknown")
	flags.StringVar(&opts.name, "name", "", "按服务名或显示名模糊过滤")
	return cmd
}

func newCheckCommand() *cobra.Command {
	opts := &checkOptions{}
	cmd := &cobra.Command{
		Use:   "check <name>",
		Short: "检查服务健康状态",
		Long: "service check 会检查指定服务当前状态，默认精确匹配服务名。" +
			"\n\n传入 --all 后改为模糊匹配并返回所有命中项；--detail 会尽量补充平台可得的额外字段。",
		Example: "  syskit service check ssh\n" +
			"  syskit service check docker --all\n" +
			"  syskit service check nginx --detail --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.all, "all", false, "按关键字匹配所有服务，而不是只做精确匹配")
	flags.BoolVar(&opts.detail, "detail", false, "补充平台可用的服务详情字段")
	return cmd
}

func runList(cmd *cobra.Command, opts *listOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := servicecollector.ListServices(ctx, servicecollector.ListOptions{
		State:   opts.state,
		Startup: opts.startup,
		Name:    opts.name,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("服务列表采集完成，共 %d 条", resultData.Total)
	if resultData.Total == 0 {
		msg = "服务列表采集完成，未命中记录"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newListPresenter(resultData))
}

func runCheck(cmd *cobra.Command, name string, opts *checkOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := servicecollector.CheckService(ctx, name, servicecollector.CheckOptions{
		All:    opts.all,
		Detail: opts.detail,
	})
	if err != nil {
		return err
	}

	msg := "服务检查完成"
	if !resultData.Found {
		msg = "服务检查完成，未命中匹配项"
	} else if resultData.Healthy {
		msg = "服务检查完成，状态健康"
	}

	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newCheckPresenter(resultData))
}

func newActionCommand(action servicecollector.Action) *cobra.Command {
	use := string(action) + " <name>"
	short := "执行服务动作"
	long := "service " + string(action) + " 属于写操作，默认仅输出 dry-run 计划。" +
		"\n\n真实执行必须显式传入 `--apply --yes`，执行后会写入审计日志。"
	example := "  syskit service " + string(action) + " nginx\n" +
		"  syskit service " + string(action) + " nginx --apply --yes"
	switch action {
	case servicecollector.ActionStart:
		short = "启动指定服务"
		long = "service start 用于启动指定服务，默认 dry-run，仅输出执行计划。" +
			"\n\n真实执行必须传入 `--apply --yes`，执行结果会写入审计日志。"
	case servicecollector.ActionStop:
		short = "停止指定服务"
		long = "service stop 用于停止指定服务，默认 dry-run，仅输出执行计划。" +
			"\n\n真实执行必须传入 `--apply --yes`，执行结果会写入审计日志。"
	case servicecollector.ActionRestart:
		short = "重启指定服务"
		long = "service restart 用于重启指定服务，默认 dry-run，仅输出执行计划。" +
			"\n\n真实执行必须传入 `--apply --yes`，执行结果会写入审计日志。"
	case servicecollector.ActionEnable:
		short = "启用服务开机自启"
		long = "service enable 用于启用服务开机自启，默认 dry-run，仅输出执行计划。" +
			"\n\n真实执行必须传入 `--apply --yes`，执行结果会写入审计日志。"
	case servicecollector.ActionDisable:
		short = "禁用服务开机自启"
		long = "service disable 用于禁用服务开机自启，默认 dry-run，仅输出执行计划。" +
			"\n\n真实执行必须传入 `--apply --yes`，执行结果会写入审计日志。"
	}

	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, action, args[0])
		},
	}
	return cmd
}

func runAction(cmd *cobra.Command, action servicecollector.Action, name string) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	plan, err := servicecollector.BuildActionPlan(ctx, action, name)
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	yes := cliutil.ResolveBoolFlag(cmd, "yes")
	if apply && !yes {
		return errs.InvalidArgument("真实执行 service 写操作需要同时传入 --yes")
	}

	if !apply {
		data := &actionOutputData{
			Mode:  "dry-run",
			Apply: false,
			Plan:  plan,
		}
		msg := fmt.Sprintf("服务动作计划已生成（action=%s）", action)
		result := output.NewSuccessResult(msg, data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newActionPresenter(data))
	}

	execResult, err := servicecollector.ExecuteAction(ctx, plan)
	if err != nil {
		auditResult := auditResultFromError(err)
		auditErr := writeAuditEvent(cmd, ctx, audit.Event{
			Command:    cmd.CommandPath(),
			Action:     "service." + string(action),
			Target:     name,
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
				fmt.Sprintf("service %s 执行失败且审计写入失败: %s", action, errs.Message(err)),
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
	msg := fmt.Sprintf("服务动作执行完成（action=%s）", action)
	auditResult := "success"
	if !execResult.Success {
		auditResult = "partial"
	}
	if err := writeAuditEvent(cmd, ctx, audit.Event{
		Command:    cmd.CommandPath(),
		Action:     "service." + string(action),
		Target:     name,
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

// Package service 负责服务管理命令组。
package service

import (
	"fmt"
	"syskit/internal/cliutil"
	servicecollector "syskit/internal/collectors/service"
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

// NewCommand 创建 `service` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "服务管理命令",
		Long: "service 提供系统服务列表与健康检查能力，后续会补齐启停和自启管理写操作。" +
			"\n\n当前已交付 list/check；start/stop/restart/enable/disable 仍保留占位提示。",
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
		cliutil.NewPendingCommand("start <name>", "启动指定服务"),
		cliutil.NewPendingCommand("stop <name>", "停止指定服务"),
		cliutil.NewPendingCommand("restart <name>", "重启指定服务"),
		cliutil.NewPendingCommand("enable <name>", "启用服务开机自启"),
		cliutil.NewPendingCommand("disable <name>", "禁用服务开机自启"),
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

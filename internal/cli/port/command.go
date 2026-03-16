// Package port 负责端口查询和端口释放命令。
package port

import (
	"fmt"
	"strconv"
	"strings"
	"syskit/internal/cliutil"
	portcollector "syskit/internal/collectors/port"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type queryOptions struct {
	detail bool
}

type listOptions struct {
	by       string
	protocol string
	listen   string
	detail   bool
}

type killOptions struct {
	force    bool
	killTree bool
}

// killOutputData 是 `port kill` 的统一输出结构。
type killOutputData struct {
	Mode   string                    `json:"mode"`
	Apply  bool                      `json:"apply"`
	Plan   *portcollector.KillPlan   `json:"plan"`
	Result *portcollector.KillResult `json:"result,omitempty"`
}

// NewCommand 创建 `port` 顶层命令。
func NewCommand() *cobra.Command {
	queryOpts := &queryOptions{}

	cmd := &cobra.Command{
		Use:   "port <port[,port]|range>",
		Short: "端口查询与释放",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runQuery(cmd, args[0], queryOpts)
		},
	}

	cmd.Flags().BoolVar(&queryOpts.detail, "detail", false, "显示用户、命令行等详细字段")
	cmd.AddCommand(
		newListCommand(),
		newKillCommand(),
	)
	return cmd
}

func newListCommand() *cobra.Command {
	opts := &listOptions{
		by: "port",
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "查看监听端口列表",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.by, "by", "port", "排序维度: port/pid")
	flags.StringVar(&opts.protocol, "protocol", "", "协议过滤: tcp/udp")
	flags.StringVar(&opts.listen, "listen", "", "监听地址过滤（模糊匹配）")
	flags.BoolVar(&opts.detail, "detail", false, "显示用户、命令行等详细字段")
	return cmd
}

func newKillCommand() *cobra.Command {
	opts := &killOptions{}
	cmd := &cobra.Command{
		Use:   "kill <port>",
		Short: "释放指定端口",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKill(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.force, "force", false, "强制终止占用进程")
	flags.BoolVar(&opts.killTree, "kill-tree", false, "连同子进程一起终止")
	return cmd
}

func runQuery(cmd *cobra.Command, expression string, opts *queryOptions) error {
	startedAt := time.Now()
	ports, err := portcollector.ParsePortExpression(expression)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := portcollector.QueryPorts(ctx, ports, opts.detail)
	if err != nil {
		return err
	}

	msg := "端口查询完成"
	if len(resultData.Entries) == 0 {
		msg = "未发现端口占用"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newQueryPresenter(resultData, opts.detail))
}

func runList(cmd *cobra.Command, opts *listOptions) error {
	startedAt := time.Now()
	by, err := portcollector.ParseSortBy(opts.by)
	if err != nil {
		return err
	}
	protocol, err := portcollector.ParseProtocol(opts.protocol)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := portcollector.ListPorts(ctx, portcollector.ListOptions{
		By:       by,
		Protocol: protocol,
		Listen:   opts.listen,
	}, opts.detail)
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("监听端口列表采集完成", resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newListPresenter(resultData, opts.detail))
}

func runKill(cmd *cobra.Command, portRaw string, opts *killOptions) error {
	startedAt := time.Now()
	port, err := parseSinglePort(portRaw)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	plan, err := portcollector.BuildKillPlan(ctx, portcollector.KillOptions{
		Port:     port,
		Force:    opts.force,
		KillTree: opts.killTree,
	})
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	yes := cliutil.ResolveBoolFlag(cmd, "yes")
	if apply && !yes {
		return errs.InvalidArgument("真实执行 port kill 需要同时传入 --yes")
	}

	if !apply {
		data := &killOutputData{
			Mode:  "dry-run",
			Apply: false,
			Plan:  plan,
		}
		result := output.NewSuccessResult("已生成端口释放计划（dry-run）", data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newKillPresenter(data))
	}

	execResult, err := portcollector.ExecuteKillPlan(ctx, plan)
	if err != nil {
		return err
	}
	data := &killOutputData{
		Mode:   "apply",
		Apply:  true,
		Plan:   plan,
		Result: execResult,
	}

	msg := fmt.Sprintf("端口 %d 释放执行完成", port)
	if !execResult.Released {
		msg = fmt.Sprintf("端口 %d 释放执行完成，但仍存在占用", port)
	}

	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newKillPresenter(data))
}

func parseSinglePort(raw string) (int, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, errs.InvalidArgument("端口不能为空")
	}
	value, err := strconv.Atoi(text)
	if err != nil || value <= 0 || value > 65535 {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效端口: %s", raw))
	}
	return value, nil
}

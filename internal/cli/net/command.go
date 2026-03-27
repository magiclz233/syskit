// Package net 负责网络连接审计与监听列表命令。
package net

import (
	"fmt"
	"time"

	"syskit/internal/cliutil"
	netcollector "syskit/internal/collectors/net"
	portcollector "syskit/internal/collectors/port"
	"syskit/internal/errs"
	"syskit/internal/output"

	"github.com/spf13/cobra"
)

type connOptions struct {
	pid    int
	state  string
	proto  string
	remote string
}

type listenOptions struct {
	proto string
	addr  string
}

// NewCommand 创建 `net` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "net",
		Short: "网络连接审计命令",
		Long: "net 提供网络连接审计和监听端口检查能力，用于补充 `port` 模块的端口视角。" +
			"\n\n当前已交付 `net conn`、`net listen`，`net speed` 继续保留占位。",
		Example: "  syskit net conn\n" +
			"  syskit net conn --proto tcp --state established\n" +
			"  syskit net listen --proto tcp --addr 127.0.0.1",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newConnCommand(),
		newListenCommand(),
		cliutil.NewPendingCommand("speed", "执行带宽测速"),
	)
	return cmd
}

func newConnCommand() *cobra.Command {
	opts := &connOptions{}
	cmd := &cobra.Command{
		Use:   "conn",
		Short: "审计当前网络连接",
		Long: "net conn 用于输出当前网络连接，支持按 PID、状态、协议和远端地址过滤。" +
			"\n\n该命令为只读操作，适用于快速排查异常连接和可疑外联。",
		Example: "  syskit net conn\n" +
			"  syskit net conn --pid 1234 --proto tcp\n" +
			"  syskit net conn --state established,time_wait --remote 10.0.0.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConn(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.pid, "pid", 0, "仅查看指定 PID 的连接")
	flags.StringVar(&opts.state, "state", "", "连接状态过滤，可逗号分隔")
	flags.StringVar(&opts.proto, "proto", "", "协议过滤: tcp/udp")
	flags.StringVar(&opts.remote, "remote", "", "远端地址过滤（模糊匹配）")
	return cmd
}

func newListenCommand() *cobra.Command {
	opts := &listenOptions{}
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "查看监听端口和地址",
		Long: "net listen 用于输出监听中的网络端口，支持按协议和本地监听地址过滤。" +
			"\n\n该命令与 `port list` 互补，面向连接视角输出网络监听条目。",
		Example: "  syskit net listen\n" +
			"  syskit net listen --proto tcp\n" +
			"  syskit net listen --addr 0.0.0.0",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListen(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.proto, "proto", "", "协议过滤: tcp/udp")
	flags.StringVar(&opts.addr, "addr", "", "监听地址过滤（模糊匹配）")
	return cmd
}

func runConn(cmd *cobra.Command, opts *connOptions) error {
	if opts.pid < 0 {
		return errs.InvalidArgument("--pid 不能小于 0")
	}
	protocol, err := portcollector.ParseProtocol(opts.proto)
	if err != nil {
		return err
	}
	states, err := netcollector.ParseStateFilter(opts.state)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := netcollector.CollectConnections(ctx, netcollector.ConnOptions{
		PID:      int32(opts.pid),
		States:   states,
		Protocol: protocol,
		Remote:   opts.remote,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("网络连接采集完成，共 %d 条", resultData.Total)
	if resultData.Total == 0 {
		msg = "网络连接采集完成，未命中记录"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newConnPresenter(resultData))
}

func runListen(cmd *cobra.Command, opts *listenOptions) error {
	protocol, err := portcollector.ParseProtocol(opts.proto)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := netcollector.CollectListen(ctx, netcollector.ListenOptions{
		Protocol: protocol,
		Addr:     opts.addr,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("监听列表采集完成，共 %d 条", resultData.Total)
	if resultData.Total == 0 {
		msg = "监听列表采集完成，未命中记录"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newListenPresenter(resultData))
}

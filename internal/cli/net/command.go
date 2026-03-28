// Package net 负责网络连接审计与监听列表命令。
package net

import (
	"fmt"
	"strings"
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

type speedOptions struct {
	server string
	mode   string
}

// NewCommand 创建 `net` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "net",
		Short: "网络连接审计命令",
		Long: "net 提供网络连接审计、监听端口检查和带宽测速能力，用于补充 `port` 模块的端口视角。" +
			"\n\n当前已交付 `net conn`、`net listen`、`net speed`。",
		Example: "  syskit net conn\n" +
			"  syskit net conn --proto tcp --state established\n" +
			"  syskit net listen --proto tcp --addr 127.0.0.1\n" +
			"  syskit net speed --mode full",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newConnCommand(),
		newListenCommand(),
		newSpeedCommand(),
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

func newSpeedCommand() *cobra.Command {
	opts := &speedOptions{mode: "full"}
	cmd := &cobra.Command{
		Use:   "speed",
		Short: "执行带宽测速",
		Long: "net speed 基于 HTTP 探测执行延迟、下载、上传测速。" +
			"\n\n`--server` 默认使用 Cloudflare 速度测试服务；可通过自建兼容端点覆盖。",
		Example: "  syskit net speed\n" +
			"  syskit net speed --mode download\n" +
			"  syskit net speed --server https://speed.cloudflare.com --mode full --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSpeed(cmd, opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.server, "server", "", "测速服务地址（默认自动选择）")
	flags.StringVar(&opts.mode, "mode", "full", "测速模式: full/download/upload")
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

func runSpeed(cmd *cobra.Command, opts *speedOptions) error {
	mode, err := netcollector.ParseSpeedMode(opts.mode)
	if err != nil {
		return err
	}
	timeout, err := resolveSpeedTimeout(cmd)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	var progress func(netcollector.SpeedProgressEvent)
	if cliutil.ResolveBoolFlag(cmd, "verbose") {
		progress = func(event netcollector.SpeedProgressEvent) {
			if strings.TrimSpace(event.Message) == "" {
				return
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[net speed] %s: %s\n", event.Stage, event.Message)
		}
	}

	resultData, err := netcollector.CollectSpeed(ctx, netcollector.SpeedOptions{
		Server:   strings.TrimSpace(opts.server),
		Mode:     mode,
		Timeout:  timeout,
		Progress: progress,
	})
	if err != nil {
		return err
	}

	msg := buildSpeedMessage(resultData)
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newSpeedPresenter(resultData))
}

func resolveSpeedTimeout(cmd *cobra.Command) (time.Duration, error) {
	raw := strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "timeout"))
	if raw == "" || raw == "0" || raw == "0s" {
		return 15 * time.Second, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --timeout: %s", raw))
	}
	if parsed <= 0 {
		return 15 * time.Second, nil
	}
	return parsed, nil
}

func buildSpeedMessage(result *netcollector.SpeedResult) string {
	if result == nil {
		return "带宽测速完成"
	}
	switch result.Mode {
	case string(netcollector.SpeedModeDownload):
		if result.Download != nil {
			return fmt.Sprintf("下载测速完成，速度 %.2f Mbps", result.Download.Mbps)
		}
	case string(netcollector.SpeedModeUpload):
		if result.Upload != nil {
			return fmt.Sprintf("上传测速完成，速度 %.2f Mbps", result.Upload.Mbps)
		}
	default:
		down := 0.0
		up := 0.0
		ping := 0.0
		if result.Download != nil {
			down = result.Download.Mbps
		}
		if result.Upload != nil {
			up = result.Upload.Mbps
		}
		if result.Ping != nil {
			ping = result.Ping.AvgMs
		}
		if result.Assessment != nil && strings.TrimSpace(result.Assessment.Summary) != "" {
			return fmt.Sprintf(
				"带宽测速完成，download=%.2f Mbps upload=%.2f Mbps ping=%.2f ms，结论：%s",
				down,
				up,
				ping,
				result.Assessment.Summary,
			)
		}
		return fmt.Sprintf("带宽测速完成，download=%.2f Mbps upload=%.2f Mbps ping=%.2f ms", down, up, ping)
	}
	return "带宽测速完成"
}

// Package traceroute 负责路由跟踪命令。
package traceroute

import (
	"fmt"
	"strings"
	"time"

	"syskit/internal/cliutil"
	networkprobe "syskit/internal/collectors/networkprobe"
	"syskit/internal/errs"
	"syskit/internal/output"

	"github.com/spf13/cobra"
)

type traceOptions struct {
	maxHops int
	proto   string
}

// NewCommand 创建 `traceroute` 命令。
func NewCommand() *cobra.Command {
	opts := &traceOptions{
		maxHops: 30,
		proto:   "icmp",
	}
	cmd := &cobra.Command{
		Use:     "traceroute <target>",
		Aliases: []string{"tracert"},
		Short:   "执行路由跟踪",
		Long: "traceroute 会逐跳探测到目标主机的网络路径，输出每一跳的地址与时延。" +
			"\n\n`--proto` 支持 icmp/tcp，Windows 下 tcp 会自动降级为 icmp 并给出提示。",
		Example: "  syskit traceroute 8.8.8.8\n" +
			"  syskit traceroute example.com --max-hops 20\n" +
			"  syskit traceroute 10.0.0.8 --proto tcp --timeout 3s --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTraceroute(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.maxHops, "max-hops", 30, "最大跳数")
	flags.StringVar(&opts.proto, "proto", "icmp", "探测协议: icmp/tcp")
	return cmd
}

func runTraceroute(cmd *cobra.Command, target string, opts *traceOptions) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errs.InvalidArgument("target 不能为空")
	}
	if opts.maxHops <= 0 {
		return errs.InvalidArgument("--max-hops 必须大于 0")
	}
	protocol, err := networkprobe.ParseTraceProtocol(opts.proto)
	if err != nil {
		return err
	}
	timeout, err := resolveTracerouteTimeout(cmd)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := networkprobe.Traceroute(ctx, networkprobe.TracerouteOptions{
		Target:   target,
		MaxHops:  opts.maxHops,
		Timeout:  timeout,
		Protocol: protocol,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("路由跟踪完成，共 %d 跳", resultData.HopCount)
	if resultData.Reached {
		msg = fmt.Sprintf("路由跟踪完成，已到达目标（%d 跳）", resultData.HopCount)
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newTraceroutePresenter(resultData))
}

func resolveTracerouteTimeout(cmd *cobra.Command) (time.Duration, error) {
	raw := strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "timeout"))
	if raw == "" || raw == "0" || raw == "0s" {
		return 2 * time.Second, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --timeout: %s", raw))
	}
	if parsed <= 0 {
		return 2 * time.Second, nil
	}
	return parsed, nil
}

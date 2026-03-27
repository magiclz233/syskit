// Package ping 负责主机连通性 Ping 命令。
package ping

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

type pingOptions struct {
	count    int
	interval time.Duration
	size     int
}

// NewCommand 创建 `ping` 命令。
func NewCommand() *cobra.Command {
	opts := &pingOptions{
		count:    4,
		interval: time.Second,
		size:     32,
	}

	cmd := &cobra.Command{
		Use:   "ping <target>",
		Short: "执行主机连通性 Ping 测试",
		Long: "ping 会对目标主机执行多次系统 Ping 探测，输出连通性、丢包率和时延统计。" +
			"\n\n`--timeout` 复用全局超时参数，并作为单次探测超时；默认 2s。",
		Example: "  syskit ping 127.0.0.1\n" +
			"  syskit ping example.com --count 6 --interval 500ms\n" +
			"  syskit ping 10.0.0.8 --size 64 --timeout 3s --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPing(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.count, "count", 4, "探测次数")
	flags.DurationVar(&opts.interval, "interval", time.Second, "探测间隔")
	flags.IntVar(&opts.size, "size", 32, "探测包大小（字节）")
	return cmd
}

func runPing(cmd *cobra.Command, target string, opts *pingOptions) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errs.InvalidArgument("target 不能为空")
	}
	if opts.count <= 0 {
		return errs.InvalidArgument("--count 必须大于 0")
	}
	if opts.interval < 0 {
		return errs.InvalidArgument("--interval 不能小于 0")
	}
	if opts.size <= 0 || opts.size > 65500 {
		return errs.InvalidArgument("--size 仅支持 1-65500 字节")
	}
	timeout, err := resolvePingTimeout(cmd)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := networkprobe.Ping(ctx, networkprobe.PingOptions{
		Target:   target,
		Count:    opts.count,
		Interval: opts.interval,
		Timeout:  timeout,
		Size:     opts.size,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Ping 测试完成（成功 %d/%d）", resultData.SuccessCount, resultData.Count)
	if resultData.SuccessCount == 0 {
		msg = "Ping 测试完成，目标不可达"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newPingPresenter(resultData))
}

func resolvePingTimeout(cmd *cobra.Command) (time.Duration, error) {
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

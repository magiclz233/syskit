// Package cpu 负责 CPU 相关命令。
package cpu

import (
	"fmt"
	"strings"
	"syskit/internal/cliutil"
	cpucollector "syskit/internal/collectors/cpu"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type options struct {
	detail bool
}

type burstOptions struct {
	interval  string
	duration  string
	threshold float64
}

// NewCommand 创建 `cpu` 顶层命令。
func NewCommand() *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{
		Use:   "cpu",
		Short: "CPU 总览与分析",
		Long: "cpu 用于输出系统 CPU 总览、负载信息以及高 CPU 进程概览。" +
			"\n\n该命令为只读诊断命令，适合先快速确认当前机器是否存在 CPU 资源争用。",
		Example: "  syskit cpu\n" +
			"  syskit cpu --detail\n" +
			"  syskit cpu --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOverview(cmd, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.detail, "detail", false, "显示每核心使用率等详细字段")
	cmd.AddCommand(newBurstCommand())
	return cmd
}

func runOverview(cmd *cobra.Command, opts *options) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	overview, err := cpucollector.CollectOverview(ctx, cpucollector.CollectOptions{
		Detail: opts.detail,
		TopN:   5,
	})
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("CPU 总览采集完成", overview, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newPresenter(overview, opts.detail))
}

func newBurstCommand() *cobra.Command {
	opts := &burstOptions{
		interval:  "500ms",
		duration:  "10s",
		threshold: 50,
	}

	cmd := &cobra.Command{
		Use:   "burst",
		Short: "捕捉突发高 CPU 进程",
		Long: "cpu burst 会按指定间隔连续采样进程 CPU 使用率，捕捉超过阈值的突发进程并输出证据。" +
			"\n\n`--duration 0` 表示持续采样，直到命令超时或手动终止。",
		Example: "  syskit cpu burst\n" +
			"  syskit cpu burst --interval 200ms --duration 8s --threshold 70\n" +
			"  syskit cpu burst --duration 0 --threshold 80 --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBurst(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.interval, "interval", "500ms", "采样间隔，支持 200ms/1s；默认 500ms")
	flags.StringVar(&opts.duration, "duration", "10s", "采样时长，0 表示持续采样")
	flags.Float64Var(&opts.threshold, "threshold", 50, "CPU 命中阈值（百分比）")
	return cmd
}

func runBurst(cmd *cobra.Command, opts *burstOptions) error {
	interval, err := parseBurstInterval(opts.interval)
	if err != nil {
		return err
	}
	duration, err := parseBurstDuration(opts.duration)
	if err != nil {
		return err
	}
	if opts.threshold <= 0 {
		return errs.InvalidArgument("--threshold 必须大于 0")
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := cpucollector.CollectBurst(ctx, cpucollector.BurstOptions{
		Interval:         interval,
		Duration:         duration,
		ThresholdPercent: opts.threshold,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("CPU 突发采样完成，捕捉到 %d 个进程", len(resultData.Processes))
	if len(resultData.Processes) == 0 {
		msg = "CPU 突发采样完成，未命中阈值进程"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newBurstPresenter(resultData))
}

func parseBurstInterval(raw string) (time.Duration, error) {
	parsed, err := parseFlexibleDuration(raw)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --interval: %s", raw))
	}
	if parsed <= 0 {
		return 0, errs.InvalidArgument("--interval 必须大于 0")
	}
	return parsed, nil
}

func parseBurstDuration(raw string) (time.Duration, error) {
	parsed, err := parseFlexibleDuration(raw)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --duration: %s", raw))
	}
	if parsed < 0 {
		return 0, errs.InvalidArgument("--duration 不能小于 0")
	}
	return parsed, nil
}

func parseFlexibleDuration(raw string) (time.Duration, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, errs.InvalidArgument("duration 不能为空")
	}
	if text == "0" {
		return 0, nil
	}

	parsed, err := time.ParseDuration(text)
	if err == nil {
		return parsed, nil
	}

	if isDigits(text) {
		seconds, secErr := time.ParseDuration(text + "s")
		if secErr == nil {
			return seconds, nil
		}
	}

	return 0, err
}

func isDigits(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, ch := range text {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

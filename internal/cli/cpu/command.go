// Package cpu 负责 CPU 相关命令。
package cpu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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

type watchOptions struct {
	top           int
	interval      string
	thresholdCPU  float64
	thresholdLoad float64
	alert         string
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
	cmd.AddCommand(
		newBurstCommand(),
		newWatchCommand(),
	)
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

func newWatchCommand() *cobra.Command {
	opts := &watchOptions{
		top:           10,
		interval:      "1s",
		thresholdCPU:  80,
		thresholdLoad: 0,
	}

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "持续监控 CPU 使用情况",
		Long: "cpu watch 会按固定间隔持续采样系统 CPU 和高负载进程，并在命中阈值时聚合告警。" +
			"\n\n命令默认持续运行，可通过 Ctrl+C 或全局 `--timeout` 安全中断并输出汇总结果。",
		Example: "  syskit cpu watch\n" +
			"  syskit cpu watch --interval 2s --top 15 --threshold-cpu 85\n" +
			"  syskit cpu watch --threshold-load 6 --timeout 30s --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.top, "top", 10, "每次采样保留的高 CPU 进程数量")
	flags.StringVar(&opts.interval, "interval", "1s", "采样间隔，支持 500ms/1s/2s")
	flags.Float64Var(&opts.thresholdCPU, "threshold-cpu", 80, "进程 CPU 告警阈值（百分比）")
	flags.Float64Var(&opts.thresholdLoad, "threshold-load", 0, "系统 load1 告警阈值，0 表示自动按 CPU 核心数推导")
	flags.StringVar(&opts.alert, "alert", "", "告警 webhook 地址（命中阈值时推送汇总）")
	return cmd
}

func runWatch(cmd *cobra.Command, opts *watchOptions) error {
	interval, err := parseBurstInterval(opts.interval)
	if err != nil {
		return err
	}
	if opts.top <= 0 {
		return errs.InvalidArgument("--top 必须大于 0")
	}
	if opts.thresholdCPU <= 0 {
		return errs.InvalidArgument("--threshold-cpu 必须大于 0")
	}
	if opts.thresholdLoad < 0 {
		return errs.InvalidArgument("--threshold-load 不能小于 0")
	}

	startedAt := time.Now()
	baseCtx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	// 使用 signal context 让 Ctrl+C 能触发“有结果退出”，而不是直接硬中断。
	ctx, stopNotify := signal.NotifyContext(baseCtx, os.Interrupt)
	defer stopNotify()

	resultData, err := cpucollector.CollectWatch(ctx, cpucollector.WatchOptions{
		TopN:          opts.top,
		Interval:      interval,
		ThresholdCPU:  opts.thresholdCPU,
		ThresholdLoad: opts.thresholdLoad,
	})
	if err != nil {
		return err
	}

	alertURL := strings.TrimSpace(opts.alert)
	if alertURL != "" && len(resultData.Alerts) > 0 {
		if pushErr := pushWatchAlert(context.Background(), alertURL, resultData); pushErr != nil {
			resultData.Warnings = append(resultData.Warnings, fmt.Sprintf("告警推送失败: %s", pushErr.Error()))
		}
	}

	msg := fmt.Sprintf("CPU 持续监控结束（样本 %d，告警 %d）", resultData.SampleCount, len(resultData.Alerts))
	switch resultData.StoppedReason {
	case "timeout":
		msg = fmt.Sprintf("CPU 持续监控在超时后结束（样本 %d，告警 %d）", resultData.SampleCount, len(resultData.Alerts))
	case "canceled":
		msg = fmt.Sprintf("CPU 持续监控被中断（样本 %d，告警 %d）", resultData.SampleCount, len(resultData.Alerts))
	}

	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newWatchPresenter(resultData))
}

func pushWatchAlert(ctx context.Context, webhookURL string, result *cpucollector.WatchResult) error {
	if result == nil {
		return nil
	}
	body := map[string]any{
		"type":           "cpu_watch_alert",
		"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		"sample_count":   result.SampleCount,
		"stopped_reason": result.StoppedReason,
		"alerts":         result.Alerts,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("webhook 返回状态码 %d", resp.StatusCode)
	}
	return nil
}

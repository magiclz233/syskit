// Package mem 负责内存相关命令。
package mem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syskit/internal/cliutil"
	memcollector "syskit/internal/collectors/mem"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type overviewOptions struct {
	detail bool
}

type topOptions struct {
	topN int
	by   string
	user string
	name string
}

type leakOptions struct {
	duration string
	interval string
}

type watchOptions struct {
	top           int
	interval      string
	thresholdMem  float64
	thresholdSwap float64
	alert         string
}

// NewCommand 创建 `mem` 顶层命令。
func NewCommand() *cobra.Command {
	overviewOpts := &overviewOptions{}

	cmd := &cobra.Command{
		Use:   "mem",
		Short: "内存总览与分析",
		Long: "mem 用于输出系统内存总览、可用内存、Swap 使用情况和高内存进程概览。" +
			"\n\n需要进一步按进程排序时，可继续使用 `mem top`。",
		Example: "  syskit mem\n" +
			"  syskit mem --detail\n" +
			"  syskit mem --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOverview(cmd, overviewOpts)
		},
	}

	cmd.Flags().BoolVar(&overviewOpts.detail, "detail", false, "显示缓存、缓冲区等详细字段")
	cmd.AddCommand(
		newTopCommand(),
		newLeakCommand(),
		newWatchCommand(),
	)
	return cmd
}

func newTopCommand() *cobra.Command {
	opts := &topOptions{
		topN: 20,
		by:   string(memcollector.SortByRSS),
	}

	cmd := &cobra.Command{
		Use:   "top",
		Short: "查看进程内存排行",
		Long: "mem top 用于按 RSS、VMS 或 Swap 维度输出进程内存排行。" +
			"\n\n可以结合 --user 和 --name 缩小排查范围。",
		Example: "  syskit mem top\n" +
			"  syskit mem top --top 10 --by rss\n" +
			"  syskit mem top --user postgres --name java",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTop(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.topN, "top", 20, "显示 Top N 进程")
	flags.StringVar(&opts.by, "by", string(memcollector.SortByRSS), "排序维度: rss/vms/swap")
	flags.StringVar(&opts.user, "user", "", "按用户名过滤（模糊匹配）")
	flags.StringVar(&opts.name, "name", "", "按进程名或命令行过滤（模糊匹配）")
	return cmd
}

func runOverview(cmd *cobra.Command, opts *overviewOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	overview, err := memcollector.CollectOverview(ctx, opts.detail, 5)
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("内存总览采集完成", overview, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newOverviewPresenter(overview, opts.detail))
}

func runTop(cmd *cobra.Command, opts *topOptions) error {
	startedAt := time.Now()
	by, err := memcollector.ParseSortBy(opts.by)
	if err != nil {
		return err
	}
	if opts.topN <= 0 {
		return errs.InvalidArgument("--top 必须大于 0")
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	topResult, err := memcollector.CollectTop(ctx, memcollector.TopOptions{
		By:   by,
		TopN: opts.topN,
		User: opts.user,
		Name: opts.name,
	})
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("内存排行采集完成", topResult, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newTopPresenter(topResult))
}

func newLeakCommand() *cobra.Command {
	opts := &leakOptions{
		duration: "30m",
		interval: "10s",
	}

	cmd := &cobra.Command{
		Use:   "leak <pid>",
		Short: "监控指定进程内存泄漏趋势",
		Long: "mem leak 会按固定间隔采样指定进程 RSS/VMS/Swap，并基于增长趋势给出泄漏风险分级。" +
			"\n\n命令默认监控 30 分钟；可通过 Ctrl+C 提前中断并输出当前汇总。",
		Example: "  syskit mem leak 1234\n" +
			"  syskit mem leak 1234 --duration 5m --interval 2s\n" +
			"  syskit mem leak 1234 --duration 2m --interval 1s --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLeak(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.duration, "duration", "30m", "监控总时长")
	flags.StringVar(&opts.interval, "interval", "10s", "采样间隔")
	return cmd
}

func runLeak(cmd *cobra.Command, pidRaw string, opts *leakOptions) error {
	pid, err := parseLeakPID(pidRaw)
	if err != nil {
		return err
	}
	duration, err := parseLeakDuration(opts.duration)
	if err != nil {
		return err
	}
	interval, err := parseLeakInterval(opts.interval)
	if err != nil {
		return err
	}
	if duration < interval {
		return errs.InvalidArgument("--duration 不能小于 --interval")
	}

	startedAt := time.Now()
	baseCtx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()
	ctx, stopNotify := signal.NotifyContext(baseCtx, os.Interrupt)
	defer stopNotify()

	resultData, err := memcollector.CollectLeak(ctx, memcollector.LeakOptions{
		PID:      pid,
		Duration: duration,
		Interval: interval,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf(
		"内存泄漏趋势监控完成（风险=%s，样本=%d）",
		resultData.LeakRisk,
		resultData.SampleCount,
	)
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newLeakPresenter(resultData))
}

func parseLeakPID(raw string) (int32, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, errs.InvalidArgument("PID 不能为空")
	}
	value, err := strconv.ParseInt(text, 10, 32)
	if err != nil || value <= 0 {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 PID: %s", raw))
	}
	return int32(value), nil
}

func parseLeakDuration(raw string) (time.Duration, error) {
	return parseLeakFlexibleDuration(raw, "--duration")
}

func parseLeakInterval(raw string) (time.Duration, error) {
	return parseLeakFlexibleDuration(raw, "--interval")
}

func parseLeakFlexibleDuration(raw string, flagName string) (time.Duration, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, errs.InvalidArgument(flagName + " 不能为空")
	}
	parsed, err := time.ParseDuration(text)
	if err != nil && isDigits(text) {
		parsed, err = time.ParseDuration(text + "s")
	}
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 %s: %s", flagName, raw))
	}
	if parsed <= 0 {
		return 0, errs.InvalidArgument(flagName + " 必须大于 0")
	}
	return parsed, nil
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
		interval:      "5s",
		thresholdMem:  90,
		thresholdSwap: 50,
	}

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "持续监控系统内存与高内存进程",
		Long: "mem watch 会按固定间隔持续采样系统内存和高内存进程，并聚合阈值告警。" +
			"\n\n命令默认持续运行，可通过 Ctrl+C 或全局 `--timeout` 安全中断并输出汇总。",
		Example: "  syskit mem watch\n" +
			"  syskit mem watch --interval 2s --top 20 --threshold-mem 85\n" +
			"  syskit mem watch --threshold-swap 40 --timeout 30s --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.top, "top", 10, "每次采样保留的高内存进程数量")
	flags.StringVar(&opts.interval, "interval", "5s", "采样间隔")
	flags.Float64Var(&opts.thresholdMem, "threshold-mem", 90, "系统内存使用率告警阈值（百分比）")
	flags.Float64Var(&opts.thresholdSwap, "threshold-swap", 50, "Swap 使用率告警阈值（百分比）")
	flags.StringVar(&opts.alert, "alert", "", "告警 webhook 地址（命中阈值时推送汇总）")
	return cmd
}

func runWatch(cmd *cobra.Command, opts *watchOptions) error {
	interval, err := parseLeakInterval(opts.interval)
	if err != nil {
		return err
	}
	if opts.top <= 0 {
		return errs.InvalidArgument("--top 必须大于 0")
	}
	if opts.thresholdMem <= 0 {
		return errs.InvalidArgument("--threshold-mem 必须大于 0")
	}
	if opts.thresholdSwap <= 0 {
		return errs.InvalidArgument("--threshold-swap 必须大于 0")
	}

	startedAt := time.Now()
	baseCtx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()
	ctx, stopNotify := signal.NotifyContext(baseCtx, os.Interrupt)
	defer stopNotify()

	resultData, err := memcollector.CollectWatch(ctx, memcollector.WatchOptions{
		TopN:          opts.top,
		Interval:      interval,
		ThresholdMem:  opts.thresholdMem,
		ThresholdSwap: opts.thresholdSwap,
	})
	if err != nil {
		return err
	}

	alertURL := strings.TrimSpace(opts.alert)
	if alertURL != "" && len(resultData.Alerts) > 0 {
		if pushErr := pushMemWatchAlert(context.Background(), alertURL, resultData); pushErr != nil {
			resultData.Warnings = append(resultData.Warnings, fmt.Sprintf("告警推送失败: %s", pushErr.Error()))
		}
	}

	msg := fmt.Sprintf("内存持续监控结束（样本 %d，告警 %d）", resultData.SampleCount, len(resultData.Alerts))
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newWatchPresenter(resultData))
}

func pushMemWatchAlert(ctx context.Context, webhookURL string, result *memcollector.WatchResult) error {
	if result == nil {
		return nil
	}
	body := map[string]any{
		"type":           "mem_watch_alert",
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

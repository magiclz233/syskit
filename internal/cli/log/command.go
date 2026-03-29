// Package logcmd 负责日志体检与检索命令组。
package logcmd

import (
	"fmt"
	"strings"
	"syskit/internal/cliutil"
	logcollector "syskit/internal/collectors/log"
	"syskit/internal/config"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type overviewOptions struct {
	since  string
	level  string
	top    int
	detail bool
}

type searchOptions struct {
	since      string
	file       string
	ignoreCase bool
	context    int
}

type watchOptions struct {
	file           string
	thresholdSize  string
	thresholdError float64
	interval       string
}

// NewCommand 创建 `log` 顶层命令。
func NewCommand() *cobra.Command {
	opts := &overviewOptions{
		since: "24h",
		level: "all",
		top:   20,
	}

	cmd := &cobra.Command{
		Use:   "log",
		Short: "日志体检命令",
		Long: "log 会对指定日志文件做快速体检，输出级别统计、错误率和高频消息摘要。" +
			"\n\n该命令为只读操作；search/watch 用于关键字检索和增量监控。",
		Example: "  syskit log\n" +
			"  syskit log --since 6h --level error --detail\n" +
			"  syskit log search timeout --context 2\n" +
			"  syskit log watch --interval 2s --threshold-error 30",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOverview(cmd, opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.since, "since", "24h", "时间范围（示例: 30m, 6h, 7d）")
	flags.StringVar(&opts.level, "level", "all", "日志级别过滤: all/error/warn/info/debug")
	flags.IntVar(&opts.top, "top", 20, "高频消息和样本 Top N")
	flags.BoolVar(&opts.detail, "detail", false, "输出日志样本")

	cmd.AddCommand(
		newSearchCommand(),
		newWatchCommand(),
	)
	return cmd
}

func newSearchCommand() *cobra.Command {
	opts := &searchOptions{
		since:      "24h",
		ignoreCase: true,
		context:    1,
	}
	cmd := &cobra.Command{
		Use:   "search <keyword>",
		Short: "搜索日志关键字",
		Long: "log search 会按关键字检索日志，并支持输出前后文。" +
			"\n\n默认忽略大小写；可通过 --file 指定单文件或 glob 模式。",
		Example: "  syskit log search timeout\n" +
			"  syskit log search ERROR --ignore-case=false --context 2\n" +
			"  syskit log search panic --file /var/log/*.log --since 2h --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, args[0], opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.since, "since", "24h", "时间范围（示例: 30m, 6h, 7d）")
	flags.StringVar(&opts.file, "file", "", "日志文件路径或 glob（逗号分隔）")
	flags.BoolVar(&opts.ignoreCase, "ignore-case", true, "是否忽略大小写")
	flags.IntVar(&opts.context, "context", 1, "前后文行数")
	return cmd
}

func newWatchCommand() *cobra.Command {
	opts := &watchOptions{
		thresholdSize:  "10MB",
		thresholdError: 30,
		interval:       "3s",
	}
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "监控日志增长和错误率",
		Long: "log watch 会周期性采集日志增量和错误率，直到触发超时或用户中断。" +
			"\n\n该命令默认只观测增量内容，适合和 --timeout 一起用于巡检任务。",
		Example: "  syskit log watch --timeout 20s\n" +
			"  syskit log watch --file ./app.log --interval 2s --threshold-size 1MB\n" +
			"  syskit log watch --threshold-error 20 --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch(cmd, opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.file, "file", "", "日志文件路径或 glob（逗号分隔）")
	flags.StringVar(&opts.thresholdSize, "threshold-size", "10MB", "单次采样增长阈值（支持 B/KB/MB/GB）")
	flags.Float64Var(&opts.thresholdError, "threshold-error", 30, "错误率阈值（0-100）")
	flags.StringVar(&opts.interval, "interval", "3s", "采样间隔")
	return cmd
}

func runOverview(cmd *cobra.Command, opts *overviewOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	since, err := parseSince(opts.since)
	if err != nil {
		return err
	}
	files, err := resolveLogFiles(cmd, "")
	if err != nil {
		return err
	}

	resultData, err := logcollector.Analyze(ctx, logcollector.OverviewOptions{
		Files:  files,
		Since:  since,
		Level:  opts.level,
		Top:    opts.top,
		Detail: opts.detail,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("日志体检完成（matches=%d）", resultData.MatchedLines)
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newOverviewPresenter(resultData))
}

func runSearch(cmd *cobra.Command, keyword string, opts *searchOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	since, err := parseSince(opts.since)
	if err != nil {
		return err
	}
	files, err := resolveLogFiles(cmd, opts.file)
	if err != nil {
		return err
	}
	resultData, err := logcollector.Search(ctx, logcollector.SearchOptions{
		Files:      files,
		Keyword:    keyword,
		Since:      since,
		IgnoreCase: opts.ignoreCase,
		Context:    opts.context,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("日志检索完成，共 %d 条命中", resultData.TotalMatches)
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newSearchPresenter(resultData))
}

func runWatch(cmd *cobra.Command, opts *watchOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	files, err := resolveLogFiles(cmd, opts.file)
	if err != nil {
		return err
	}
	thresholdSize, err := parseSize(opts.thresholdSize)
	if err != nil {
		return err
	}
	interval, err := parsePositiveDuration(opts.interval, "--interval")
	if err != nil {
		return err
	}
	if opts.thresholdError < 0 || opts.thresholdError > 100 {
		return errs.InvalidArgument("--threshold-error 必须在 0 到 100 之间")
	}

	resultData, err := logcollector.Watch(ctx, logcollector.WatchOptions{
		Files:          files,
		ThresholdSize:  thresholdSize,
		ThresholdError: opts.thresholdError,
		Interval:       interval,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("日志监控结束（samples=%d alerts=%d）", resultData.SampleCount, len(resultData.Alerts))
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newWatchPresenter(resultData))
}

func resolveLogFiles(cmd *cobra.Command, raw string) ([]string, error) {
	if strings.TrimSpace(raw) != "" {
		return splitLogFiles(raw), nil
	}
	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Logging.File) == "" {
		return nil, errs.InvalidArgument("未配置 logging.file，且未通过 --file 指定日志文件")
	}
	return []string{cfg.Logging.File}, nil
}

func splitLogFiles(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func parseSince(raw string) (time.Duration, error) {
	text := strings.ToLower(strings.TrimSpace(raw))
	if text == "" {
		return 0, nil
	}
	if duration, err := time.ParseDuration(text); err == nil {
		if duration <= 0 {
			return 0, errs.InvalidArgument("--since 必须大于 0")
		}
		return duration, nil
	}
	if strings.HasSuffix(text, "d") {
		value := strings.TrimSuffix(text, "d")
		days, err := parsePositiveInt(value)
		if err != nil {
			return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --since: %s", raw))
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --since: %s（示例: 30m, 6h, 7d）", raw))
}

func parsePositiveDuration(raw string, flagName string) (time.Duration, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, errs.InvalidArgument(flagName + " 不能为空")
	}
	duration, err := time.ParseDuration(text)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 %s: %s", flagName, raw))
	}
	if duration <= 0 {
		return 0, errs.InvalidArgument(flagName + " 必须大于 0")
	}
	return duration, nil
}

func parseSize(raw string) (int64, error) {
	text := strings.ToUpper(strings.TrimSpace(raw))
	if text == "" {
		return 0, nil
	}
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(text, "KB"):
		multiplier = 1024
		text = strings.TrimSuffix(text, "KB")
	case strings.HasSuffix(text, "MB"):
		multiplier = 1024 * 1024
		text = strings.TrimSuffix(text, "MB")
	case strings.HasSuffix(text, "GB"):
		multiplier = 1024 * 1024 * 1024
		text = strings.TrimSuffix(text, "GB")
	case strings.HasSuffix(text, "B"):
		text = strings.TrimSuffix(text, "B")
	}
	value, err := parsePositiveInt(text)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --threshold-size: %s", raw))
	}
	return int64(value) * multiplier, nil
}

func parsePositiveInt(raw string) (int, error) {
	value := 0
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errs.InvalidArgument("数值不能为空")
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, errs.InvalidArgument("无效数字")
		}
		value = value*10 + int(ch-'0')
	}
	if value <= 0 {
		return 0, errs.InvalidArgument("数值必须大于 0")
	}
	return value, nil
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

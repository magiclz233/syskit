// Package monitor 负责持续监控与定时巡检命令。
package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syskit/internal/cli/doctor"
	"syskit/internal/cliutil"
	cpucollector "syskit/internal/collectors/cpu"
	diskcollector "syskit/internal/collectors/disk"
	memcollector "syskit/internal/collectors/mem"
	netcollector "syskit/internal/collectors/net"
	proccollector "syskit/internal/collectors/proc"
	"syskit/internal/config"
	"syskit/internal/errs"
	"syskit/internal/output"
	"syskit/internal/storage"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultMonitorTopN = 5
)

type allOptions struct {
	interval           string
	maxSamples         int
	alert              string
	inspectionInterval string
	inspectionMode     string
	inspectionFailOn   string
}

type normalizedOptions struct {
	interval           time.Duration
	maxSamples         int
	alertURL           string
	alertThreshold     int
	inspectionInterval time.Duration
	inspectionMode     string
	inspectionFailOn   string
}

type monitorOutput struct {
	IntervalMs           int64               `json:"interval_ms"`
	MaxSamples           int                 `json:"max_samples"`
	AlertThreshold       int                 `json:"alert_threshold"`
	InspectionIntervalMs int64               `json:"inspection_interval_ms,omitempty"`
	InspectionMode       string              `json:"inspection_mode,omitempty"`
	InspectionFailOn     string              `json:"inspection_fail_on,omitempty"`
	StartedAt            time.Time           `json:"started_at"`
	EndedAt              time.Time           `json:"ended_at"`
	StoppedReason        string              `json:"stopped_reason"`
	SampleCount          int                 `json:"sample_count"`
	MonitorFile          string              `json:"monitor_file"`
	LatestSample         *monitorSample      `json:"latest_sample,omitempty"`
	Peaks                monitorPeaks        `json:"peaks"`
	Alerts               []monitorAlert      `json:"alerts"`
	Inspections          []monitorInspection `json:"inspections,omitempty"`
	Warnings             []string            `json:"warnings,omitempty"`
}

type monitorPeaks struct {
	CPUPercent        float64 `json:"cpu_percent"`
	Load1             float64 `json:"load1"`
	MemPercent        float64 `json:"mem_percent"`
	SwapPercent       float64 `json:"swap_percent"`
	DiskPercent       float64 `json:"disk_percent"`
	ConnectionCount   int     `json:"connection_count"`
	ListenCount       int     `json:"listen_count"`
	ProcessCount      int     `json:"process_count"`
	DiskPeakMount     string  `json:"disk_peak_mount,omitempty"`
	DiskPeakCollected bool    `json:"disk_peak_collected"`
}

type monitorSample struct {
	Timestamp       time.Time            `json:"timestamp"`
	CPUPercent      float64              `json:"cpu_percent"`
	Load1           float64              `json:"load1"`
	MemPercent      float64              `json:"mem_percent"`
	SwapPercent     float64              `json:"swap_percent"`
	DiskPercent     float64              `json:"disk_percent"`
	DiskPeakMount   string               `json:"disk_peak_mount,omitempty"`
	ConnectionCount int                  `json:"connection_count"`
	ListenCount     int                  `json:"listen_count"`
	ProcessCount    int                  `json:"process_count"`
	TriggeredAlerts []triggeredAlert     `json:"triggered_alerts,omitempty"`
	Inspection      *monitorInspection   `json:"inspection,omitempty"`
	Warnings        []string             `json:"warnings,omitempty"`
	TopCPU          []processSampleBrief `json:"top_cpu,omitempty"`
}

type processSampleBrief struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	Command    string  `json:"command,omitempty"`
	CPUPercent float64 `json:"cpu_percent"`
	RSSBytes   uint64  `json:"rss_bytes"`
}

type triggeredAlert struct {
	Key         string    `json:"key"`
	Type        string    `json:"type"`
	Summary     string    `json:"summary"`
	Threshold   float64   `json:"threshold"`
	Value       float64   `json:"value"`
	Consecutive int       `json:"consecutive"`
	Timestamp   time.Time `json:"timestamp"`
}

type monitorAlert struct {
	Key                 string    `json:"key"`
	Type                string    `json:"type"`
	Summary             string    `json:"summary"`
	Threshold           float64   `json:"threshold"`
	PeakValue           float64   `json:"peak_value"`
	Occurrences         int       `json:"occurrences"`
	ConsecutiveRequired int       `json:"consecutive_required"`
	FirstSeenAt         time.Time `json:"first_seen_at"`
	LastSeenAt          time.Time `json:"last_seen_at"`
}

type monitorInspection struct {
	Timestamp     time.Time `json:"timestamp"`
	Mode          string    `json:"mode"`
	FailOn        string    `json:"fail_on"`
	HealthScore   int       `json:"health_score,omitempty"`
	HealthLevel   string    `json:"health_level,omitempty"`
	IssueCount    int       `json:"issue_count"`
	SkippedCount  int       `json:"skipped_count"`
	FailOnMatched bool      `json:"fail_on_matched"`
	ExitCode      int       `json:"exit_code"`
	Error         string    `json:"error,omitempty"`
	Warnings      []string  `json:"warnings,omitempty"`
}

type thresholdSnapshot struct {
	CPUPercent      float64
	MemPercent      float64
	DiskPercent     float64
	ConnectionCount int
	ProcessCount    int
}

type alertAccumulator struct {
	key                 string
	typ                 string
	threshold           float64
	peakValue           float64
	occurrences         int
	consecutiveRequired int
	firstSeenAt         time.Time
	lastSeenAt          time.Time
}

type inspectSchedule struct {
	interval time.Duration
	mode     string
	failOn   string
	lastRun  time.Time
}

// NewCommand 创建 `monitor` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "持续监控与巡检命令",
		Long: "monitor 提供持续资源监控、阈值告警与定时巡检入口。" +
			"\n\n当前已交付 monitor all，可把监控样本写入 data_dir/monitor 并在周期内执行 doctor all 巡检。",
		Example: "  syskit monitor all --timeout 30s\n" +
			"  syskit monitor all --interval 2s --max-samples 20 --format json\n" +
			"  syskit monitor all --inspection-interval 1m --inspection-mode deep",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAllCommand())
	return cmd
}

func newAllCommand() *cobra.Command {
	opts := &allOptions{
		interval:           "",
		maxSamples:         0,
		alert:              "",
		inspectionInterval: "",
		inspectionMode:     "quick",
		inspectionFailOn:   "",
	}

	cmd := &cobra.Command{
		Use:   "all",
		Short: "持续监控全系统并可定时触发巡检",
		Long: "monitor all 会按固定间隔采样 CPU、内存、磁盘、网络和进程概要，样本会持续写入 monitor 目录。" +
			"\n\n当告警连续命中达到 monitor.alert_threshold 时会触发告警事件；可选启用定时巡检（doctor all）并进行 webhook 推送。",
		Example: "  syskit monitor all --timeout 30s\n" +
			"  syskit monitor all --interval 2s --max-samples 30 --alert http://127.0.0.1:8080/webhook\n" +
			"  syskit monitor all --inspection-interval 1m --inspection-mode deep --inspection-fail-on high",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAll(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.interval, "interval", "", "采样间隔，默认取 config.monitor.interval_sec")
	flags.IntVar(&opts.maxSamples, "max-samples", 0, "本次监控最大样本数，默认取 config.monitor.max_samples")
	flags.StringVar(&opts.alert, "alert", "", "告警 webhook 地址（触发告警/巡检异常时推送）")
	flags.StringVar(&opts.inspectionInterval, "inspection-interval", "", "巡检间隔，设置后定时执行 doctor all")
	flags.StringVar(&opts.inspectionMode, "inspection-mode", "quick", "巡检模式: quick/deep")
	flags.StringVar(&opts.inspectionFailOn, "inspection-fail-on", "", "巡检 fail-on 阈值，默认继承全局 --fail-on")
	return cmd
}

func runAll(cmd *cobra.Command, opts *allOptions) error {
	startedAt := time.Now().UTC()

	cfgResult, err := config.Load(config.LoadOptions{
		ExplicitPath: strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "config")),
	})
	if err != nil {
		return err
	}
	layout, err := storage.EnsureLayout(cfgResult.Config.Storage.DataDir)
	if err != nil {
		return err
	}

	normalized, warn, err := normalizeOptions(opts, cfgResult.Config, cliutil.ResolveStringFlag(cmd, "fail-on"))
	if err != nil {
		return err
	}

	baseCtx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()
	ctx, stopNotify := signal.NotifyContext(baseCtx, os.Interrupt)
	defer stopNotify()

	monitorFile, writer, closeWriter, err := openMonitorWriter(layout.MonitorDir, startedAt)
	if err != nil {
		return err
	}
	defer closeWriter()

	data := &monitorOutput{
		IntervalMs:     normalized.interval.Milliseconds(),
		MaxSamples:     normalized.maxSamples,
		AlertThreshold: normalized.alertThreshold,
		StartedAt:      startedAt,
		StoppedReason:  "completed",
		MonitorFile:    monitorFile,
		Alerts:         []monitorAlert{},
		Inspections:    []monitorInspection{},
		Warnings:       append([]string{}, warn...),
	}
	if normalized.inspectionInterval > 0 {
		data.InspectionIntervalMs = normalized.inspectionInterval.Milliseconds()
		data.InspectionMode = normalized.inspectionMode
		data.InspectionFailOn = normalized.inspectionFailOn
	}

	thresholds := thresholdSnapshot{
		CPUPercent:      cfgResult.Config.Thresholds.CPUPercent,
		MemPercent:      cfgResult.Config.Thresholds.MemPercent,
		DiskPercent:     cfgResult.Config.Thresholds.DiskPercent,
		ConnectionCount: cfgResult.Config.Thresholds.ConnectionCount,
		ProcessCount:    cfgResult.Config.Thresholds.ProcessCount,
	}

	schedule := inspectSchedule{
		interval: normalized.inspectionInterval,
		mode:     normalized.inspectionMode,
		failOn:   normalized.inspectionFailOn,
	}

	alertConsecutive := make(map[string]int, 8)
	alertStats := make(map[string]*alertAccumulator, 8)
	ticker := time.NewTicker(normalized.interval)
	defer ticker.Stop()

loop:
	for {
		if ctx.Err() != nil {
			data.StoppedReason = stopReason(ctx.Err())
			break loop
		}

		sample := collectSample(ctx, time.Now().UTC())
		data.Warnings = appendUniqueStrings(data.Warnings, sample.Warnings...)
		triggered := evaluateAlerts(sample, thresholds, normalized.alertThreshold, alertConsecutive, alertStats)
		if len(triggered) > 0 {
			sample.TriggeredAlerts = triggered
		}

		inspection := maybeRunInspection(ctx, cmd, &schedule, sample.Timestamp)
		if inspection != nil {
			sample.Inspection = inspection
			data.Inspections = append(data.Inspections, *inspection)
			data.Warnings = appendUniqueStrings(data.Warnings, inspection.Warnings...)
			if inspection.Error != "" {
				data.Warnings = appendUniqueStrings(data.Warnings, "巡检执行失败: "+inspection.Error)
			}
		}

		if err := writeSample(writer, sample); err != nil {
			return err
		}

		data.SampleCount++
		data.LatestSample = sample
		updatePeaks(&data.Peaks, sample)

		webhookNeeded := len(sample.TriggeredAlerts) > 0 || inspectionNeedsWebhook(inspection)
		if webhookNeeded && normalized.alertURL != "" {
			if pushErr := pushAlert(ctx, normalized.alertURL, monitorFile, sample); pushErr != nil {
				data.Warnings = appendUniqueStrings(data.Warnings, "告警推送失败: "+pushErr.Error())
			}
		}

		if data.SampleCount >= normalized.maxSamples {
			data.StoppedReason = "max_samples"
			break loop
		}

		select {
		case <-ctx.Done():
			data.StoppedReason = stopReason(ctx.Err())
			break loop
		case <-ticker.C:
		}
	}

	data.EndedAt = time.Now().UTC()
	data.Alerts = buildMonitorAlerts(alertStats)
	data.Warnings = appendUniqueStrings(data.Warnings)

	msg := fmt.Sprintf("全系统监控结束（样本 %d，告警 %d）", data.SampleCount, len(data.Alerts))
	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newPresenter(data))
}

func normalizeOptions(opts *allOptions, cfg *config.Config, defaultFailOn string) (normalizedOptions, []string, error) {
	if opts == nil {
		opts = &allOptions{}
	}
	if cfg == nil {
		return normalizedOptions{}, nil, errs.ExecutionFailed("读取监控配置失败", fmt.Errorf("nil config"))
	}

	warnings := make([]string, 0, 2)
	interval := time.Duration(cfg.Monitor.IntervalSec) * time.Second
	if strings.TrimSpace(opts.interval) != "" {
		var err error
		interval, err = parsePositiveDuration(opts.interval, "--interval")
		if err != nil {
			return normalizedOptions{}, nil, err
		}
	}
	if interval <= 0 {
		return normalizedOptions{}, nil, errs.InvalidArgument("--interval 必须大于 0")
	}

	maxSamples := cfg.Monitor.MaxSamples
	if opts.maxSamples > 0 {
		maxSamples = opts.maxSamples
	}
	if maxSamples <= 0 {
		return normalizedOptions{}, nil, errs.InvalidArgument("--max-samples 必须大于 0")
	}

	alertThreshold := cfg.Monitor.AlertThreshold
	if alertThreshold <= 0 {
		warnings = append(warnings, "monitor.alert_threshold <= 0，已自动按 1 处理")
		alertThreshold = 1
	}

	inspectionInterval := time.Duration(0)
	if strings.TrimSpace(opts.inspectionInterval) != "" {
		var err error
		inspectionInterval, err = parsePositiveDuration(opts.inspectionInterval, "--inspection-interval")
		if err != nil {
			return normalizedOptions{}, nil, err
		}
	}

	inspectionMode, err := normalizeInspectionMode(opts.inspectionMode)
	if err != nil {
		return normalizedOptions{}, nil, err
	}
	inspectionFailOn, err := normalizeFailOn(opts.inspectionFailOn, defaultFailOn)
	if err != nil {
		return normalizedOptions{}, nil, err
	}

	return normalizedOptions{
		interval:           interval,
		maxSamples:         maxSamples,
		alertURL:           strings.TrimSpace(opts.alert),
		alertThreshold:     alertThreshold,
		inspectionInterval: inspectionInterval,
		inspectionMode:     inspectionMode,
		inspectionFailOn:   inspectionFailOn,
	}, warnings, nil
}

func parsePositiveDuration(raw string, flagName string) (time.Duration, error) {
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

func normalizeInspectionMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return "quick", nil
	}
	switch mode {
	case "quick", "deep":
		return mode, nil
	default:
		return "", errs.InvalidArgument("--inspection-mode 仅支持 quick/deep")
	}
}

func normalizeFailOn(raw string, fallback string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(fallback))
	}
	if value == "" {
		value = "high"
	}
	switch value {
	case "critical", "high", "medium", "low", "never":
		return value, nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("不支持的 fail-on 阈值: %s", value))
	}
}

func openMonitorWriter(dir string, now time.Time) (string, *os.File, func(), error) {
	filename := fmt.Sprintf("monitor_%s.jsonl", now.UTC().Format("20060102T150405Z"))
	path := filepath.Join(dir, filename)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", nil, nil, errs.ExecutionFailed("创建监控样本文件失败", err)
	}

	return path, file, func() {
		_ = file.Close()
	}, nil
}

func collectSample(ctx context.Context, timestamp time.Time) *monitorSample {
	sample := &monitorSample{
		Timestamp: timestamp,
		Warnings:  []string{},
		TopCPU:    []processSampleBrief{},
	}

	cpuOverview, cpuErr := cpucollector.CollectOverview(ctx, cpucollector.CollectOptions{
		Detail: false,
		TopN:   defaultMonitorTopN,
	})
	if cpuErr != nil {
		sample.Warnings = append(sample.Warnings, "CPU 采样失败: "+errs.Message(cpuErr))
	} else {
		sample.CPUPercent = cpuOverview.UsagePercent
		sample.Load1 = cpuOverview.Load1
		sample.Warnings = append(sample.Warnings, cpuOverview.Warnings...)
	}

	memOverview, memErr := memcollector.CollectOverview(ctx, false, defaultMonitorTopN)
	if memErr != nil {
		sample.Warnings = append(sample.Warnings, "内存采样失败: "+errs.Message(memErr))
	} else {
		sample.MemPercent = memOverview.UsagePercent
		sample.SwapPercent = memOverview.SwapUsagePercent
		sample.Warnings = append(sample.Warnings, memOverview.Warnings...)
	}

	diskOverview, diskErr := diskcollector.CollectOverview()
	if diskErr != nil {
		sample.Warnings = append(sample.Warnings, "磁盘采样失败: "+errs.Message(diskErr))
	} else {
		usage, mount := findDiskPeak(diskOverview)
		sample.DiskPercent = usage
		sample.DiskPeakMount = mount
		sample.Warnings = append(sample.Warnings, diskOverview.Warnings...)
	}

	procTop, procErr := proccollector.CollectTop(ctx, proccollector.TopOptions{
		By:   proccollector.SortByCPU,
		TopN: defaultMonitorTopN,
	})
	if procErr != nil {
		sample.Warnings = append(sample.Warnings, "进程采样失败: "+errs.Message(procErr))
	} else {
		sample.ProcessCount = procTop.TotalMatched
		sample.Warnings = append(sample.Warnings, procTop.Warnings...)
		for _, item := range procTop.Processes {
			sample.TopCPU = append(sample.TopCPU, processSampleBrief{
				PID:        item.PID,
				Name:       item.Name,
				Command:    item.Command,
				CPUPercent: item.CPUPercent,
				RSSBytes:   item.RSSBytes,
			})
		}
	}

	connResult, connErr := netcollector.CollectConnections(ctx, netcollector.ConnOptions{})
	if connErr != nil {
		sample.Warnings = append(sample.Warnings, "网络连接采样失败: "+errs.Message(connErr))
	} else {
		sample.ConnectionCount = connResult.Total
		sample.Warnings = append(sample.Warnings, connResult.Warnings...)
	}

	listenResult, listenErr := netcollector.CollectListen(ctx, netcollector.ListenOptions{})
	if listenErr != nil {
		sample.Warnings = append(sample.Warnings, "监听端口采样失败: "+errs.Message(listenErr))
	} else {
		sample.ListenCount = listenResult.Total
		sample.Warnings = append(sample.Warnings, listenResult.Warnings...)
	}

	sample.Warnings = appendUniqueStrings(sample.Warnings)
	return sample
}

func findDiskPeak(overview *diskcollector.Overview) (float64, string) {
	if overview == nil || len(overview.Partitions) == 0 {
		return 0, ""
	}

	var usage float64
	mount := ""
	for _, item := range overview.Partitions {
		if item.UsagePercent > usage {
			usage = item.UsagePercent
			mount = item.MountPoint
		}
	}
	return usage, mount
}

func evaluateAlerts(
	sample *monitorSample,
	thresholds thresholdSnapshot,
	alertThreshold int,
	consecutive map[string]int,
	stats map[string]*alertAccumulator,
) []triggeredAlert {
	if sample == nil {
		return nil
	}

	triggered := make([]triggeredAlert, 0, 4)
	triggered = append(triggered, evaluateSingleAlert(
		sample.Timestamp,
		"cpu_usage",
		"cpu",
		"CPU 使用率连续超阈值",
		sample.CPUPercent,
		thresholds.CPUPercent,
		alertThreshold,
		consecutive,
		stats,
	)...)
	triggered = append(triggered, evaluateSingleAlert(
		sample.Timestamp,
		"mem_usage",
		"memory",
		"内存使用率连续超阈值",
		sample.MemPercent,
		thresholds.MemPercent,
		alertThreshold,
		consecutive,
		stats,
	)...)
	triggered = append(triggered, evaluateSingleAlert(
		sample.Timestamp,
		"disk_usage",
		"disk",
		"磁盘使用率连续超阈值",
		sample.DiskPercent,
		thresholds.DiskPercent,
		alertThreshold,
		consecutive,
		stats,
	)...)
	triggered = append(triggered, evaluateSingleAlert(
		sample.Timestamp,
		"connection_count",
		"network",
		"连接数连续超阈值",
		float64(sample.ConnectionCount),
		float64(thresholds.ConnectionCount),
		alertThreshold,
		consecutive,
		stats,
	)...)
	triggered = append(triggered, evaluateSingleAlert(
		sample.Timestamp,
		"process_count",
		"process",
		"进程总数连续超阈值",
		float64(sample.ProcessCount),
		float64(thresholds.ProcessCount),
		alertThreshold,
		consecutive,
		stats,
	)...)
	return triggered
}

// evaluateSingleAlert 只在“首次达到连续阈值”时触发事件。
// 这样做可以避免高频监控在持续超阈值场景下重复推送同一告警。
func evaluateSingleAlert(
	at time.Time,
	key string,
	typ string,
	summary string,
	value float64,
	threshold float64,
	required int,
	consecutive map[string]int,
	stats map[string]*alertAccumulator,
) []triggeredAlert {
	if threshold <= 0 {
		consecutive[key] = 0
		return nil
	}
	if required <= 0 {
		required = 1
	}

	if value < threshold {
		consecutive[key] = 0
		return nil
	}

	current := consecutive[key] + 1
	consecutive[key] = current
	acc := stats[key]
	if acc == nil {
		acc = &alertAccumulator{
			key:                 key,
			typ:                 typ,
			threshold:           threshold,
			consecutiveRequired: required,
			firstSeenAt:         at,
			lastSeenAt:          at,
			peakValue:           value,
		}
		stats[key] = acc
	}
	acc.occurrences++
	acc.lastSeenAt = at
	if value > acc.peakValue {
		acc.peakValue = value
	}

	if current != required {
		return nil
	}
	return []triggeredAlert{
		{
			Key:         key,
			Type:        typ,
			Summary:     summary,
			Threshold:   threshold,
			Value:       value,
			Consecutive: current,
			Timestamp:   at,
		},
	}
}

func maybeRunInspection(ctx context.Context, cmd *cobra.Command, schedule *inspectSchedule, now time.Time) *monitorInspection {
	if schedule == nil || schedule.interval <= 0 {
		return nil
	}
	if !schedule.lastRun.IsZero() && now.Sub(schedule.lastRun) < schedule.interval {
		return nil
	}
	schedule.lastRun = now

	result, err := doctor.RunAllOnce(ctx, doctor.AllRunRequest{
		ConfigPath: strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "config")),
		PolicyPath: strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "policy")),
		Mode:       schedule.mode,
		FailOn:     schedule.failOn,
	})
	if err != nil {
		return &monitorInspection{
			Timestamp: now,
			Mode:      schedule.mode,
			FailOn:    schedule.failOn,
			ExitCode:  errs.Code(err),
			Error:     errs.Message(err),
			Warnings:  []string{errs.Suggestion(err)},
		}
	}

	return &monitorInspection{
		Timestamp:     now,
		Mode:          result.Mode,
		FailOn:        result.FailOn,
		HealthScore:   result.HealthScore,
		HealthLevel:   result.HealthLevel,
		IssueCount:    len(result.Issues),
		SkippedCount:  len(result.Skipped),
		FailOnMatched: result.FailOnMatched,
		ExitCode:      result.ExitCode,
		Warnings:      appendUniqueStrings(result.Warnings),
	}
}

func inspectionNeedsWebhook(item *monitorInspection) bool {
	if item == nil {
		return false
	}
	if item.Error != "" {
		return true
	}
	return item.FailOnMatched || item.ExitCode == errs.ExitWarning
}

func writeSample(writer *os.File, sample *monitorSample) error {
	if writer == nil || sample == nil {
		return errs.ExecutionFailed("写入监控样本失败", fmt.Errorf("invalid sample writer"))
	}
	payload, err := json.Marshal(sample)
	if err != nil {
		return errs.ExecutionFailed("序列化监控样本失败", err)
	}
	if _, err := writer.Write(append(payload, '\n')); err != nil {
		return errs.ExecutionFailed("写入监控样本失败", err)
	}
	return nil
}

func pushAlert(ctx context.Context, url string, monitorFile string, sample *monitorSample) error {
	body := map[string]any{
		"type":         "monitor_all_alert",
		"timestamp":    time.Now().UTC().Format(time.RFC3339Nano),
		"monitor_file": monitorFile,
		"sample":       sample,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
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

func updatePeaks(peaks *monitorPeaks, sample *monitorSample) {
	if peaks == nil || sample == nil {
		return
	}
	if sample.CPUPercent > peaks.CPUPercent {
		peaks.CPUPercent = sample.CPUPercent
	}
	if sample.Load1 > peaks.Load1 {
		peaks.Load1 = sample.Load1
	}
	if sample.MemPercent > peaks.MemPercent {
		peaks.MemPercent = sample.MemPercent
	}
	if sample.SwapPercent > peaks.SwapPercent {
		peaks.SwapPercent = sample.SwapPercent
	}
	if sample.DiskPercent > peaks.DiskPercent {
		peaks.DiskPercent = sample.DiskPercent
		peaks.DiskPeakMount = sample.DiskPeakMount
		peaks.DiskPeakCollected = true
	}
	if sample.ConnectionCount > peaks.ConnectionCount {
		peaks.ConnectionCount = sample.ConnectionCount
	}
	if sample.ListenCount > peaks.ListenCount {
		peaks.ListenCount = sample.ListenCount
	}
	if sample.ProcessCount > peaks.ProcessCount {
		peaks.ProcessCount = sample.ProcessCount
	}
}

func buildMonitorAlerts(items map[string]*alertAccumulator) []monitorAlert {
	if len(items) == 0 {
		return []monitorAlert{}
	}

	alerts := make([]monitorAlert, 0, len(items))
	for _, item := range items {
		if item == nil || item.occurrences == 0 {
			continue
		}
		alerts = append(alerts, monitorAlert{
			Key:                 item.key,
			Type:                item.typ,
			Summary:             summarizeAlert(item.typ, item.threshold, item.occurrences, item.peakValue),
			Threshold:           item.threshold,
			PeakValue:           item.peakValue,
			Occurrences:         item.occurrences,
			ConsecutiveRequired: item.consecutiveRequired,
			FirstSeenAt:         item.firstSeenAt,
			LastSeenAt:          item.lastSeenAt,
		})
	}

	sort.Slice(alerts, func(i int, j int) bool {
		if alerts[i].Occurrences != alerts[j].Occurrences {
			return alerts[i].Occurrences > alerts[j].Occurrences
		}
		if alerts[i].PeakValue != alerts[j].PeakValue {
			return alerts[i].PeakValue > alerts[j].PeakValue
		}
		return alerts[i].Key < alerts[j].Key
	})
	return alerts
}

func summarizeAlert(typ string, threshold float64, occurrences int, peak float64) string {
	switch typ {
	case "cpu":
		return fmt.Sprintf("CPU 使用率 %d 次超过阈值 %.2f%%，峰值 %.2f%%", occurrences, threshold, peak)
	case "memory":
		return fmt.Sprintf("内存使用率 %d 次超过阈值 %.2f%%，峰值 %.2f%%", occurrences, threshold, peak)
	case "disk":
		return fmt.Sprintf("磁盘使用率 %d 次超过阈值 %.2f%%，峰值 %.2f%%", occurrences, threshold, peak)
	case "network":
		return fmt.Sprintf("连接数 %d 次超过阈值 %.0f，峰值 %.0f", occurrences, threshold, peak)
	case "process":
		return fmt.Sprintf("进程数 %d 次超过阈值 %.0f，峰值 %.0f", occurrences, threshold, peak)
	default:
		return fmt.Sprintf("%s 告警命中 %d 次", typ, occurrences)
	}
}

func stopReason(err error) string {
	if err == nil {
		return "completed"
	}
	if err == context.DeadlineExceeded {
		return "timeout"
	}
	if err == context.Canceled {
		return "canceled"
	}
	return "interrupted"
}

func appendUniqueStrings(items []string, values ...string) []string {
	set := make(map[string]struct{}, len(items)+len(values))
	result := make([]string, 0, len(items)+len(values))
	appendOne := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := set[value]; ok {
			return
		}
		set[value] = struct{}{}
		result = append(result, value)
	}

	for _, item := range items {
		appendOne(item)
	}
	for _, item := range values {
		appendOne(item)
	}
	sort.Strings(result)
	return result
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

func formatPercent(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

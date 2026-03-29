package cpu

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"syskit/internal/errs"
	"time"
)

const (
	defaultWatchTopN         = 10
	defaultWatchInterval     = time.Second
	defaultWatchThresholdCPU = 80.0
)

// WatchOptions 定义 `cpu watch` 的采样参数。
type WatchOptions struct {
	TopN          int
	Interval      time.Duration
	ThresholdCPU  float64
	ThresholdLoad float64
}

// WatchAlert 表示持续监控期间聚合后的告警记录。
type WatchAlert struct {
	Type        string    `json:"type"`
	Summary     string    `json:"summary"`
	Threshold   float64   `json:"threshold"`
	PeakValue   float64   `json:"peak_value"`
	Occurrences int       `json:"occurrences"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	PID         int32     `json:"pid,omitempty"`
	ProcessName string    `json:"process_name,omitempty"`
	Command     string    `json:"command,omitempty"`
}

// WatchResult 是 `cpu watch` 的结构化输出。
type WatchResult struct {
	TopN          int          `json:"top_n"`
	IntervalMs    int64        `json:"interval_ms"`
	ThresholdCPU  float64      `json:"threshold_cpu"`
	ThresholdLoad float64      `json:"threshold_load"`
	CPUCores      int          `json:"cpu_cores"`
	SampleCount   int          `json:"sample_count"`
	StartedAt     time.Time    `json:"started_at"`
	EndedAt       time.Time    `json:"ended_at"`
	StoppedReason string       `json:"stopped_reason"`
	PeakCPU       float64      `json:"peak_cpu"`
	AvgCPU        float64      `json:"avg_cpu"`
	PeakLoad1     float64      `json:"peak_load1"`
	LastCPU       float64      `json:"last_cpu"`
	LastLoad1     float64      `json:"last_load1"`
	LastTop       []TopProcess `json:"last_top_processes"`
	Alerts        []WatchAlert `json:"alerts"`
	Warnings      []string     `json:"warnings,omitempty"`
}

type watchAlertAccumulator struct {
	typ         string
	threshold   float64
	pid         int32
	processName string
	command     string
	peakValue   float64
	occurrences int
	firstSeenAt time.Time
	lastSeenAt  time.Time
}

// CollectWatch 按固定间隔持续采样 CPU，并在上下文结束时输出汇总结果。
func CollectWatch(ctx context.Context, opts WatchOptions) (*WatchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	normalized, err := normalizeWatchOptions(opts)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now().UTC()
	result := &WatchResult{
		TopN:          normalized.TopN,
		IntervalMs:    normalized.Interval.Milliseconds(),
		ThresholdCPU:  normalized.ThresholdCPU,
		ThresholdLoad: normalized.ThresholdLoad,
		StartedAt:     startedAt,
		StoppedReason: "completed",
		LastTop:       []TopProcess{},
		Alerts:        []WatchAlert{},
	}

	cpuCores, coreErr := collectCPUCores(ctx)
	if coreErr != nil {
		result.Warnings = appendUnique(result.Warnings, "读取 CPU 核心数失败，已按 1 核计算系统负载阈值")
		cpuCores = 1
	}
	result.CPUCores = cpuCores
	if normalized.ThresholdLoad <= 0 {
		normalized.ThresholdLoad = float64(cpuCores) * 2
		result.ThresholdLoad = normalized.ThresholdLoad
	}

	alertMap := map[string]*watchAlertAccumulator{}
	ticker := time.NewTicker(normalized.Interval)
	defer ticker.Stop()

	var usageSum float64
	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			result.StoppedReason = watchStopReason(ctxErr)
			break
		}

		overview, collectErr := CollectOverview(ctx, CollectOptions{
			Detail: false,
			TopN:   normalized.TopN,
		})
		if collectErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				result.StoppedReason = watchStopReason(ctxErr)
				break
			}
			result.Warnings = appendUnique(result.Warnings, fmt.Sprintf("采样失败: %s", errs.Message(collectErr)))
		} else if overview != nil {
			result.SampleCount++
			usageSum += overview.UsagePercent
			result.LastCPU = roundBurstPercent(overview.UsagePercent)
			result.LastLoad1 = roundBurstPercent(overview.Load1)
			result.LastTop = append([]TopProcess{}, overview.TopProcesses...)

			if overview.UsagePercent > result.PeakCPU {
				result.PeakCPU = overview.UsagePercent
			}
			if overview.Load1 > result.PeakLoad1 {
				result.PeakLoad1 = overview.Load1
			}

			result.Warnings = appendUnique(result.Warnings, overview.Warnings...)
			collectWatchAlerts(alertMap, overview, normalized, time.Now().UTC())
		}

		select {
		case <-ctx.Done():
			result.StoppedReason = watchStopReason(ctx.Err())
			break
		case <-ticker.C:
			continue
		}
		break
	}

	if result.SampleCount > 0 {
		result.AvgCPU = roundBurstPercent(usageSum / float64(result.SampleCount))
	}
	result.PeakCPU = roundBurstPercent(result.PeakCPU)
	result.PeakLoad1 = roundBurstPercent(result.PeakLoad1)
	result.Alerts = buildWatchAlerts(alertMap)
	result.EndedAt = time.Now().UTC()
	return result, nil
}

func normalizeWatchOptions(opts WatchOptions) (WatchOptions, error) {
	if opts.TopN <= 0 {
		opts.TopN = defaultWatchTopN
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultWatchInterval
	}
	if opts.Interval <= 0 {
		return WatchOptions{}, errs.InvalidArgument("--interval 必须大于 0")
	}
	if opts.ThresholdCPU <= 0 {
		opts.ThresholdCPU = defaultWatchThresholdCPU
	}
	if opts.ThresholdCPU <= 0 {
		return WatchOptions{}, errs.InvalidArgument("--threshold-cpu 必须大于 0")
	}
	if opts.ThresholdLoad < 0 {
		return WatchOptions{}, errs.InvalidArgument("--threshold-load 不能小于 0")
	}
	return opts, nil
}

func collectCPUCores(ctx context.Context) (int, error) {
	overview, err := CollectOverview(ctx, CollectOptions{
		Detail: false,
		TopN:   1,
	})
	if err != nil {
		return 0, err
	}
	if overview == nil || overview.CPUCores <= 0 {
		return 0, errs.ExecutionFailed("读取 CPU 核心数失败", fmt.Errorf("cpu cores is zero"))
	}
	return overview.CPUCores, nil
}

func collectWatchAlerts(
	alerts map[string]*watchAlertAccumulator,
	overview *Overview,
	opts WatchOptions,
	sampleAt time.Time,
) {
	if alerts == nil || overview == nil {
		return
	}

	if overview.Load1 >= opts.ThresholdLoad {
		updateWatchAlert(alerts, "system_load", watchAlertInput{
			threshold: opts.ThresholdLoad,
			value:     overview.Load1,
			summary:   fmt.Sprintf("系统 load1 达到 %.2f，超过阈值 %.2f", roundBurstPercent(overview.Load1), opts.ThresholdLoad),
			at:        sampleAt,
		})
	}

	for _, proc := range overview.TopProcesses {
		if proc.CPUPercent < opts.ThresholdCPU {
			continue
		}
		key := fmt.Sprintf("process_cpu:%d", proc.PID)
		updateWatchAlert(alerts, key, watchAlertInput{
			typ:         "process_cpu",
			threshold:   opts.ThresholdCPU,
			value:       proc.CPUPercent,
			pid:         proc.PID,
			processName: proc.Name,
			command:     proc.Command,
			summary: fmt.Sprintf(
				"进程 %s(%d) CPU 达到 %.2f%%，超过阈值 %.2f%%",
				displayProcessName(proc.Name, proc.PID),
				proc.PID,
				roundBurstPercent(proc.CPUPercent),
				opts.ThresholdCPU,
			),
			at: sampleAt,
		})
	}
}

type watchAlertInput struct {
	typ         string
	threshold   float64
	value       float64
	pid         int32
	processName string
	command     string
	summary     string
	at          time.Time
}

func updateWatchAlert(alerts map[string]*watchAlertAccumulator, key string, in watchAlertInput) {
	if alerts == nil || key == "" {
		return
	}

	item := alerts[key]
	if item == nil {
		typ := in.typ
		if typ == "" {
			typ = "system_load"
		}
		item = &watchAlertAccumulator{
			typ:         typ,
			threshold:   in.threshold,
			pid:         in.pid,
			processName: in.processName,
			command:     in.command,
			peakValue:   in.value,
			firstSeenAt: in.at,
			lastSeenAt:  in.at,
		}
		alerts[key] = item
	}

	item.occurrences++
	item.lastSeenAt = in.at
	if item.firstSeenAt.IsZero() {
		item.firstSeenAt = in.at
	}
	if in.value > item.peakValue {
		item.peakValue = in.value
	}
	// summary 由 buildWatchAlerts 按标准模板生成，避免字段变更后内容漂移。
}

func buildWatchAlerts(alertMap map[string]*watchAlertAccumulator) []WatchAlert {
	if len(alertMap) == 0 {
		return []WatchAlert{}
	}

	result := make([]WatchAlert, 0, len(alertMap))
	for _, item := range alertMap {
		if item == nil || item.occurrences == 0 {
			continue
		}

		alert := WatchAlert{
			Type:        item.typ,
			Threshold:   roundBurstPercent(item.threshold),
			PeakValue:   roundBurstPercent(item.peakValue),
			Occurrences: item.occurrences,
			FirstSeenAt: item.firstSeenAt,
			LastSeenAt:  item.lastSeenAt,
			PID:         item.pid,
			ProcessName: item.processName,
			Command:     item.command,
		}
		if alert.Type == "process_cpu" {
			alert.Summary = fmt.Sprintf(
				"进程 %s(%d) 在监控窗口内 %d 次超过 CPU 阈值 %.2f%%，峰值 %.2f%%",
				displayProcessName(item.processName, item.pid),
				item.pid,
				item.occurrences,
				roundBurstPercent(item.threshold),
				roundBurstPercent(item.peakValue),
			)
		} else {
			alert.Type = "system_load"
			alert.Summary = fmt.Sprintf(
				"系统 load1 在监控窗口内 %d 次超过阈值 %.2f，峰值 %.2f",
				item.occurrences,
				roundBurstPercent(item.threshold),
				roundBurstPercent(item.peakValue),
			)
		}
		result = append(result, alert)
	}

	sort.Slice(result, func(i int, j int) bool {
		left := result[i]
		right := result[j]
		if left.Occurrences != right.Occurrences {
			return left.Occurrences > right.Occurrences
		}
		if left.PeakValue != right.PeakValue {
			return left.PeakValue > right.PeakValue
		}
		if left.Type != right.Type {
			return left.Type < right.Type
		}
		return left.PID < right.PID
	})
	return result
}

func watchStopReason(err error) string {
	if err == nil {
		return "completed"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "interrupted"
}

func displayProcessName(name string, pid int32) string {
	if name == "" {
		return fmt.Sprintf("pid-%d", pid)
	}
	return name
}

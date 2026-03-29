package mem

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"syskit/internal/errs"
	"time"
)

const (
	defaultWatchTopN          = 10
	defaultWatchInterval      = 5 * time.Second
	defaultWatchThresholdMem  = 90.0
	defaultWatchThresholdSwap = 50.0
)

// WatchOptions 定义 `mem watch` 的采样参数。
type WatchOptions struct {
	TopN          int
	Interval      time.Duration
	ThresholdMem  float64
	ThresholdSwap float64
}

// WatchAlert 表示监控窗口内聚合告警。
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

// WatchResult 是 `mem watch` 的结构化输出。
type WatchResult struct {
	TopN          int            `json:"top_n"`
	IntervalMs    int64          `json:"interval_ms"`
	ThresholdMem  float64        `json:"threshold_mem"`
	ThresholdSwap float64        `json:"threshold_swap"`
	SampleCount   int            `json:"sample_count"`
	StartedAt     time.Time      `json:"started_at"`
	EndedAt       time.Time      `json:"ended_at"`
	StoppedReason string         `json:"stopped_reason"`
	PeakMem       float64        `json:"peak_mem"`
	AvgMem        float64        `json:"avg_mem"`
	PeakSwap      float64        `json:"peak_swap"`
	LastMem       float64        `json:"last_mem"`
	LastSwap      float64        `json:"last_swap"`
	LastTop       []ProcessEntry `json:"last_top_processes"`
	Alerts        []WatchAlert   `json:"alerts"`
	Warnings      []string       `json:"warnings,omitempty"`
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

// CollectWatch 持续采样系统内存与高内存进程，并在上下文结束时输出汇总。
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
		ThresholdMem:  normalized.ThresholdMem,
		ThresholdSwap: normalized.ThresholdSwap,
		StartedAt:     startedAt,
		StoppedReason: "completed",
		LastTop:       []ProcessEntry{},
		Alerts:        []WatchAlert{},
	}

	alertMap := map[string]*watchAlertAccumulator{}
	ticker := time.NewTicker(normalized.Interval)
	defer ticker.Stop()

	var memSum float64
	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			result.StoppedReason = watchStopReason(ctxErr)
			break
		}

		overview, collectErr := CollectOverview(ctx, false, normalized.TopN)
		if collectErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				result.StoppedReason = watchStopReason(ctxErr)
				break
			}
			result.Warnings = appendUnique(result.Warnings, fmt.Sprintf("采样失败: %s", errs.Message(collectErr)))
		} else if overview != nil {
			result.SampleCount++
			memSum += overview.UsagePercent
			result.LastMem = roundWatchValue(overview.UsagePercent)
			result.LastSwap = roundWatchValue(overview.SwapUsagePercent)
			result.LastTop = append([]ProcessEntry{}, overview.TopProcesses...)

			if overview.UsagePercent > result.PeakMem {
				result.PeakMem = overview.UsagePercent
			}
			if overview.SwapUsagePercent > result.PeakSwap {
				result.PeakSwap = overview.SwapUsagePercent
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
		result.AvgMem = roundWatchValue(memSum / float64(result.SampleCount))
	}
	result.PeakMem = roundWatchValue(result.PeakMem)
	result.PeakSwap = roundWatchValue(result.PeakSwap)
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
	if opts.ThresholdMem <= 0 {
		opts.ThresholdMem = defaultWatchThresholdMem
	}
	if opts.ThresholdSwap <= 0 {
		opts.ThresholdSwap = defaultWatchThresholdSwap
	}
	if opts.ThresholdMem <= 0 {
		return WatchOptions{}, errs.InvalidArgument("--threshold-mem 必须大于 0")
	}
	if opts.ThresholdSwap <= 0 {
		return WatchOptions{}, errs.InvalidArgument("--threshold-swap 必须大于 0")
	}
	return opts, nil
}

func collectWatchAlerts(alerts map[string]*watchAlertAccumulator, overview *Overview, opts WatchOptions, at time.Time) {
	if alerts == nil || overview == nil {
		return
	}

	if overview.UsagePercent >= opts.ThresholdMem {
		updateWatchAlert(alerts, "system_mem", watchAlertInput{
			typ:       "system_mem",
			threshold: opts.ThresholdMem,
			value:     overview.UsagePercent,
			at:        at,
		})
	}
	if overview.SwapUsagePercent >= opts.ThresholdSwap {
		updateWatchAlert(alerts, "system_swap", watchAlertInput{
			typ:       "system_swap",
			threshold: opts.ThresholdSwap,
			value:     overview.SwapUsagePercent,
			at:        at,
		})
	}

	for _, item := range overview.TopProcesses {
		memPercent := float64(item.MemPercent)
		if memPercent < opts.ThresholdMem {
			continue
		}
		key := fmt.Sprintf("process_mem:%d", item.PID)
		updateWatchAlert(alerts, key, watchAlertInput{
			typ:         "process_mem",
			threshold:   opts.ThresholdMem,
			value:       memPercent,
			pid:         item.PID,
			processName: item.Name,
			command:     item.Command,
			at:          at,
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
	at          time.Time
}

func updateWatchAlert(alerts map[string]*watchAlertAccumulator, key string, in watchAlertInput) {
	if alerts == nil || key == "" {
		return
	}

	item := alerts[key]
	if item == nil {
		item = &watchAlertAccumulator{
			typ:         in.typ,
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
			Threshold:   roundWatchValue(item.threshold),
			PeakValue:   roundWatchValue(item.peakValue),
			Occurrences: item.occurrences,
			FirstSeenAt: item.firstSeenAt,
			LastSeenAt:  item.lastSeenAt,
			PID:         item.pid,
			ProcessName: item.processName,
			Command:     item.command,
		}

		switch item.typ {
		case "system_mem":
			alert.Summary = fmt.Sprintf(
				"系统内存使用率在窗口内 %d 次超过阈值 %.2f%%，峰值 %.2f%%",
				item.occurrences,
				roundWatchValue(item.threshold),
				roundWatchValue(item.peakValue),
			)
		case "system_swap":
			alert.Summary = fmt.Sprintf(
				"Swap 使用率在窗口内 %d 次超过阈值 %.2f%%，峰值 %.2f%%",
				item.occurrences,
				roundWatchValue(item.threshold),
				roundWatchValue(item.peakValue),
			)
		default:
			alert.Type = "process_mem"
			alert.Summary = fmt.Sprintf(
				"进程 %s(%d) 在窗口内 %d 次超过内存阈值 %.2f%%，峰值 %.2f%%",
				displayProcessName(item.processName, item.pid),
				item.pid,
				item.occurrences,
				roundWatchValue(item.threshold),
				roundWatchValue(item.peakValue),
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

func roundWatchValue(value float64) float64 {
	return math.Round(value*100) / 100
}

func displayProcessName(name string, pid int32) string {
	if name == "" {
		return fmt.Sprintf("pid-%d", pid)
	}
	return name
}

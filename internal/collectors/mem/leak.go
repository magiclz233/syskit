package mem

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"syskit/internal/errs"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

const (
	defaultLeakDuration = 30 * time.Minute
	defaultLeakInterval = 10 * time.Second
)

// LeakOptions 定义 `mem leak` 的采样参数。
type LeakOptions struct {
	PID      int32
	Duration time.Duration
	Interval time.Duration
}

// LeakSample 表示单次内存采样结果。
type LeakSample struct {
	Timestamp time.Time `json:"timestamp"`
	RSSBytes  uint64    `json:"rss_bytes"`
	VMSBytes  uint64    `json:"vms_bytes"`
	SwapBytes uint64    `json:"swap_bytes"`
}

// LeakResult 是 `mem leak` 的结构化输出。
type LeakResult struct {
	PID                int32        `json:"pid"`
	DurationMs         int64        `json:"duration_ms"`
	IntervalMs         int64        `json:"interval_ms"`
	SampleCount        int          `json:"sample_count"`
	StartedAt          time.Time    `json:"started_at"`
	EndedAt            time.Time    `json:"ended_at"`
	StoppedReason      string       `json:"stopped_reason"`
	Samples            []LeakSample `json:"samples"`
	RSSStartBytes      uint64       `json:"rss_start_bytes"`
	RSSEndBytes        uint64       `json:"rss_end_bytes"`
	RSSPeakBytes       uint64       `json:"rss_peak_bytes"`
	RSSGrowthBytes     int64        `json:"rss_growth_bytes"`
	RSSGrowthRateMBMin float64      `json:"rss_growth_rate_mb_min"`
	LeakRisk           string       `json:"leak_risk"`
	LeakReason         string       `json:"leak_reason"`
	Warnings           []string     `json:"warnings,omitempty"`
}

// CollectLeak 按固定间隔采样指定进程内存，并输出泄漏风险评估。
func CollectLeak(ctx context.Context, opts LeakOptions) (*LeakResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	normalized, err := normalizeLeakOptions(opts)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now().UTC()
	result := &LeakResult{
		PID:           normalized.PID,
		DurationMs:    normalized.Duration.Milliseconds(),
		IntervalMs:    normalized.Interval.Milliseconds(),
		StartedAt:     startedAt,
		StoppedReason: "completed",
		Samples:       make([]LeakSample, 0, 16),
		LeakRisk:      "low",
		LeakReason:    "采样窗口内未观察到持续增长趋势",
	}

	deadline := startedAt.Add(normalized.Duration)
	for {
		if err := ctx.Err(); err != nil {
			result.StoppedReason = leakStopReason(err)
			break
		}

		sample, sampleErr := collectLeakSample(ctx, normalized.PID)
		if sampleErr != nil {
			if isProcessGoneError(sampleErr) {
				result.StoppedReason = "process_exited"
				result.Warnings = appendUnique(result.Warnings, "目标进程已退出，监控提前结束")
				break
			}
			return nil, sampleErr
		}

		result.SampleCount++
		result.Samples = append(result.Samples, sample)
		if result.SampleCount == 1 {
			result.RSSStartBytes = sample.RSSBytes
			result.RSSPeakBytes = sample.RSSBytes
		}
		result.RSSEndBytes = sample.RSSBytes
		if sample.RSSBytes > result.RSSPeakBytes {
			result.RSSPeakBytes = sample.RSSBytes
		}

		now := time.Now()
		if !now.Before(deadline) {
			break
		}

		wait := normalized.Interval
		remaining := time.Until(deadline)
		if remaining < wait {
			wait = remaining
		}
		if wait <= 0 {
			break
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			result.StoppedReason = leakStopReason(ctx.Err())
			break
		case <-timer.C:
			continue
		}
		break
	}

	result.EndedAt = time.Now().UTC()
	result.RSSGrowthBytes = int64(result.RSSEndBytes) - int64(result.RSSStartBytes)
	durationMin := result.EndedAt.Sub(result.StartedAt).Minutes()
	if durationMin > 0 {
		result.RSSGrowthRateMBMin = roundLeakValue(float64(result.RSSGrowthBytes) / 1024 / 1024 / durationMin)
	}
	result.LeakRisk, result.LeakReason = classifyLeakRisk(result.RSSGrowthBytes, result.RSSGrowthRateMBMin, result.SampleCount)
	return result, nil
}

func normalizeLeakOptions(opts LeakOptions) (LeakOptions, error) {
	if opts.PID <= 0 {
		return LeakOptions{}, errs.InvalidArgument("PID 必须大于 0")
	}
	if opts.Duration <= 0 {
		opts.Duration = defaultLeakDuration
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultLeakInterval
	}
	if opts.Duration <= 0 {
		return LeakOptions{}, errs.InvalidArgument("--duration 必须大于 0")
	}
	if opts.Interval <= 0 {
		return LeakOptions{}, errs.InvalidArgument("--interval 必须大于 0")
	}
	if opts.Duration < opts.Interval {
		return LeakOptions{}, errs.InvalidArgument("--duration 不能小于 --interval")
	}
	return opts, nil
}

func collectLeakSample(ctx context.Context, pid int32) (LeakSample, error) {
	procRef, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return LeakSample{}, mapLeakProcessError(pid, err)
	}
	memInfo, memErr := procRef.MemoryInfoWithContext(ctx)
	if memErr != nil || memInfo == nil {
		if memErr == nil {
			memErr = fmt.Errorf("memory info is nil")
		}
		return LeakSample{}, mapLeakProcessError(pid, memErr)
	}
	return LeakSample{
		Timestamp: time.Now().UTC(),
		RSSBytes:  memInfo.RSS,
		VMSBytes:  memInfo.VMS,
		SwapBytes: memInfo.Swap,
	}, nil
}

func mapLeakProcessError(pid int32, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "命令执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
	}
	if isProcessGoneError(err) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, fmt.Sprintf("未找到 PID=%d 的进程", pid))
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "permission denied") || strings.Contains(text, "access denied") || strings.Contains(text, "operation not permitted") {
		return errs.PermissionDenied(fmt.Sprintf("读取 PID=%d 进程内存失败", pid), "请提升权限后重试")
	}
	return errs.ExecutionFailed(fmt.Sprintf("读取 PID=%d 进程内存失败", pid), err)
}

func isProcessGoneError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "not found") ||
		strings.Contains(text, "no such process") ||
		strings.Contains(text, "not exist")
}

func leakStopReason(err error) string {
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

func classifyLeakRisk(growthBytes int64, rateMBMin float64, samples int) (string, string) {
	if samples < 3 {
		return "low", "采样点不足，建议拉长监控窗口后再判断"
	}
	if growthBytes <= 0 {
		return "low", "RSS 无持续增长，暂未发现泄漏迹象"
	}
	if rateMBMin >= 20 || growthBytes >= 1024*1024*1024 {
		return "high", "RSS 增长速度过快，存在较高内存泄漏风险"
	}
	if rateMBMin >= 5 || growthBytes >= 256*1024*1024 {
		return "medium", "RSS 持续增长，建议进一步排查对象生命周期和缓存回收"
	}
	return "low", "RSS 有增长但幅度较小，建议继续观察"
}

func roundLeakValue(value float64) float64 {
	return math.Round(value*1000) / 1000
}

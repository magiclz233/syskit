package cpu

import (
	"context"
	"fmt"
	"math"
	"sort"
	"syskit/internal/errs"
	"time"

	gocpu "github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	defaultBurstInterval  = 500 * time.Millisecond
	defaultBurstThreshold = 50.0
)

// BurstOptions 定义 `cpu burst` 的采样参数。
type BurstOptions struct {
	Interval         time.Duration
	Duration         time.Duration
	ThresholdPercent float64
}

// BurstProcess 表示单个被捕捉进程的突发证据。
type BurstProcess struct {
	PID            int32     `json:"pid"`
	Name           string    `json:"name"`
	User           string    `json:"user,omitempty"`
	PeakCPUPercent float64   `json:"peak_cpu_percent"`
	AvgCPUPercent  float64   `json:"avg_cpu_percent"`
	HitCount       int       `json:"hit_count"`
	DurationSec    float64   `json:"duration_sec"`
	FirstSeenAt    time.Time `json:"first_seen_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	PeakAt         time.Time `json:"peak_at"`
	Command        string    `json:"command,omitempty"`
}

// BurstResult 是 `cpu burst` 的结构化输出。
type BurstResult struct {
	IntervalMs       int64          `json:"interval_ms"`
	DurationMs       int64          `json:"duration_ms"`
	Continuous       bool           `json:"continuous"`
	ThresholdPercent float64        `json:"threshold_percent"`
	CPUCores         int            `json:"cpu_cores"`
	SampleCount      int            `json:"sample_count"`
	StartedAt        time.Time      `json:"started_at"`
	EndedAt          time.Time      `json:"ended_at"`
	Processes        []BurstProcess `json:"processes"`
	Warnings         []string       `json:"warnings,omitempty"`
}

type burstMeta struct {
	name    string
	user    string
	command string
}

type burstAccumulator struct {
	meta     burstMeta
	hits     int
	sum      float64
	peak     float64
	peakAt   time.Time
	firstHit time.Time
	lastHit  time.Time
}

// CollectBurst 对进程 CPU 进行高频采样，捕捉超过阈值的突发进程。
func CollectBurst(ctx context.Context, opts BurstOptions) (*BurstResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	normalized, err := normalizeBurstOptions(opts)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now().UTC()
	result := &BurstResult{
		IntervalMs:       normalized.Interval.Milliseconds(),
		DurationMs:       normalized.Duration.Milliseconds(),
		Continuous:       normalized.Duration == 0,
		ThresholdPercent: normalized.ThresholdPercent,
		CPUCores:         1,
		StartedAt:        startedAt,
		Processes:        make([]BurstProcess, 0, 8),
	}

	cpuCores, coreErr := gocpu.CountsWithContext(ctx, true)
	if coreErr != nil || cpuCores <= 0 {
		result.Warnings = appendUnique(result.Warnings, "读取 CPU 核心数失败，已按 1 核计算百分比")
		cpuCores = 1
	}
	result.CPUCores = cpuCores

	prevTotalCPU, prevProcessCPU, err := readBurstSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	accumulators := make(map[int32]*burstAccumulator, 16)
	deadline := time.Time{}
	if normalized.Duration > 0 {
		deadline = startedAt.Add(normalized.Duration)
	}

	for {
		wait := normalized.Interval
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				break
			}
			if remaining < wait {
				wait = remaining
			}
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, mapCollectionError("CPU 突发采样失败", ctx.Err())
		case <-timer.C:
		}

		currentTotalCPU, currentProcessCPU, snapErr := readBurstSnapshot(ctx)
		if snapErr != nil {
			return nil, snapErr
		}

		deltaTotalCPU := currentTotalCPU - prevTotalCPU
		if deltaTotalCPU <= 0 {
			result.Warnings = appendUnique(result.Warnings, "采样窗口内 CPU 总时间未变化，已跳过当前窗口")
			prevTotalCPU = currentTotalCPU
			prevProcessCPU = currentProcessCPU
			continue
		}

		result.SampleCount++
		sampleAt := time.Now().UTC()

		for pid, currentCPU := range currentProcessCPU {
			prevCPU, exists := prevProcessCPU[pid]
			if !exists {
				continue
			}
			deltaProcessCPU := currentCPU - prevCPU
			if deltaProcessCPU <= 0 {
				continue
			}

			// 这里用“进程 CPU 时间增量 / 全机 CPU 时间增量”计算窗口占比，
			// 再乘核心数以保持与 top 一致的多核百分比语义（单进程可超过 100%）。
			usagePercent := deltaProcessCPU / deltaTotalCPU * 100 * float64(cpuCores)
			if usagePercent < normalized.ThresholdPercent {
				continue
			}

			item := accumulators[pid]
			if item == nil {
				item = &burstAccumulator{
					meta: loadBurstMeta(ctx, pid),
				}
				accumulators[pid] = item
			}

			item.hits++
			item.sum += usagePercent
			if item.firstHit.IsZero() {
				item.firstHit = sampleAt
			}
			item.lastHit = sampleAt
			if usagePercent > item.peak || item.peakAt.IsZero() {
				item.peak = usagePercent
				item.peakAt = sampleAt
			}
		}

		prevTotalCPU = currentTotalCPU
		prevProcessCPU = currentProcessCPU
	}

	if result.SampleCount == 0 {
		result.Warnings = appendUnique(result.Warnings, "采样窗口过短，未产生有效 CPU 增量样本")
	}

	result.Processes = buildBurstProcesses(accumulators, normalized.Interval)
	result.EndedAt = time.Now().UTC()
	return result, nil
}

func normalizeBurstOptions(opts BurstOptions) (BurstOptions, error) {
	if opts.Interval <= 0 {
		opts.Interval = defaultBurstInterval
	}
	if opts.Interval <= 0 {
		return BurstOptions{}, errs.InvalidArgument("--interval 必须大于 0")
	}

	if opts.Duration < 0 {
		return BurstOptions{}, errs.InvalidArgument("--duration 不能小于 0")
	}
	if opts.Duration > 0 && opts.Duration < opts.Interval {
		return BurstOptions{}, errs.InvalidArgument("--duration 不能小于 --interval")
	}

	// duration=0 表示持续采样模式，直到超时或用户中断。
	if opts.ThresholdPercent <= 0 {
		opts.ThresholdPercent = defaultBurstThreshold
	}
	if opts.ThresholdPercent <= 0 {
		return BurstOptions{}, errs.InvalidArgument("--threshold 必须大于 0")
	}
	return opts, nil
}

func readBurstSnapshot(ctx context.Context) (float64, map[int32]float64, error) {
	totalCPU, err := readTotalCPUSeconds(ctx)
	if err != nil {
		return 0, nil, err
	}
	processCPU, err := readProcessCPUSeconds(ctx)
	if err != nil {
		return 0, nil, err
	}
	return totalCPU, processCPU, nil
}

func readTotalCPUSeconds(ctx context.Context) (float64, error) {
	times, err := gocpu.TimesWithContext(ctx, false)
	if err != nil {
		return 0, mapCollectionError("读取系统 CPU 采样失败", err)
	}
	if len(times) == 0 {
		return 0, errs.ExecutionFailed("读取系统 CPU 采样失败", fmt.Errorf("cpu times is empty"))
	}
	return sumCPUTimes(times[0]), nil
}

func readProcessCPUSeconds(ctx context.Context) (map[int32]float64, error) {
	processes, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, mapCollectionError("读取进程列表失败", err)
	}

	result := make(map[int32]float64, len(processes))
	for _, procRef := range processes {
		if err := ctx.Err(); err != nil {
			return nil, mapCollectionError("读取进程 CPU 采样失败", err)
		}
		times, timesErr := procRef.TimesWithContext(ctx)
		if timesErr != nil || times == nil {
			continue
		}
		seconds := times.User + times.System
		if seconds < 0 {
			continue
		}
		result[procRef.Pid] = seconds
	}
	return result, nil
}

func loadBurstMeta(ctx context.Context, pid int32) burstMeta {
	item := burstMeta{
		name: fmt.Sprintf("pid-%d", pid),
	}
	procRef, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return item
	}
	if name, nameErr := procRef.NameWithContext(ctx); nameErr == nil && name != "" {
		item.name = name
	}
	if user, userErr := procRef.UsernameWithContext(ctx); userErr == nil {
		item.user = user
	}
	if command, cmdErr := procRef.CmdlineWithContext(ctx); cmdErr == nil {
		item.command = command
	}
	return item
}

func buildBurstProcesses(accumulators map[int32]*burstAccumulator, interval time.Duration) []BurstProcess {
	if len(accumulators) == 0 {
		return []BurstProcess{}
	}

	result := make([]BurstProcess, 0, len(accumulators))
	for pid, item := range accumulators {
		if item == nil || item.hits == 0 {
			continue
		}

		durationSec := item.lastHit.Sub(item.firstHit).Seconds()
		if item.hits == 1 {
			durationSec = interval.Seconds()
		} else {
			durationSec += interval.Seconds()
		}
		if durationSec < 0 {
			durationSec = 0
		}

		result = append(result, BurstProcess{
			PID:            pid,
			Name:           item.meta.name,
			User:           item.meta.user,
			PeakCPUPercent: roundBurstPercent(item.peak),
			AvgCPUPercent:  roundBurstPercent(item.sum / float64(item.hits)),
			HitCount:       item.hits,
			DurationSec:    roundBurstSeconds(durationSec),
			FirstSeenAt:    item.firstHit,
			LastSeenAt:     item.lastHit,
			PeakAt:         item.peakAt,
			Command:        item.meta.command,
		})
	}

	sort.Slice(result, func(i int, j int) bool {
		left := result[i]
		right := result[j]
		if left.PeakCPUPercent != right.PeakCPUPercent {
			return left.PeakCPUPercent > right.PeakCPUPercent
		}
		if left.AvgCPUPercent != right.AvgCPUPercent {
			return left.AvgCPUPercent > right.AvgCPUPercent
		}
		return left.PID < right.PID
	})
	return result
}

func sumCPUTimes(stat gocpu.TimesStat) float64 {
	return stat.User +
		stat.System +
		stat.Idle +
		stat.Nice +
		stat.Iowait +
		stat.Irq +
		stat.Softirq +
		stat.Steal +
		stat.Guest +
		stat.GuestNice
}

func roundBurstPercent(value float64) float64 {
	return math.Round(value*100) / 100
}

func roundBurstSeconds(value float64) float64 {
	return math.Round(value*1000) / 1000
}

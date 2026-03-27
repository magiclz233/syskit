package networkprobe

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"syskit/internal/errs"
)

const (
	defaultPingCount    = 4
	defaultPingInterval = time.Second
	defaultPingTimeout  = 2 * time.Second
	defaultPingSize     = 32
	maxPingSize         = 65500
)

var (
	pingLatencyPattern = regexp.MustCompile(`(?i)(?:time|时间)\s*[=<]?\s*([0-9]+(?:[.,][0-9]+)?)\s*ms`)
	pingTTLPattern     = regexp.MustCompile(`(?i)(?:ttl|生存时间)\s*[=:]?\s*([0-9]+)`)
)

type preparedPingOptions struct {
	Target   string
	Count    int
	Interval time.Duration
	Timeout  time.Duration
	Size     int
}

// Ping 对目标执行多次系统 Ping，并输出可解析的结构化结果。
func Ping(ctx context.Context, opts PingOptions) (*PingResult, error) {
	prepared, err := normalizePingOptions(opts)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	result := &PingResult{
		Target:     prepared.Target,
		Count:      prepared.Count,
		IntervalMs: prepared.Interval.Milliseconds(),
		TimeoutMs:  prepared.Timeout.Milliseconds(),
		Size:       prepared.Size,
		Attempts:   make([]PingAttempt, 0, prepared.Count),
	}
	warningSet := make(map[string]struct{})
	latencies := make([]float64, 0, prepared.Count)

	for seq := 1; seq <= prepared.Count; seq++ {
		if ctxErr := mapContextError(ctx.Err(), "Ping 测试超时"); ctxErr != nil {
			return nil, ctxErr
		}

		attempt := PingAttempt{Seq: seq}
		startedAt := time.Now()
		output, probeErr := runPingOnce(ctx, prepared)
		if probeErr != nil {
			if isCommandNotFound(probeErr) {
				return nil, commandUnavailableError("ping")
			}
			if ctxErr := mapContextError(probeErr, "Ping 测试超时"); ctxErr != nil {
				return nil, ctxErr
			}

			attempt.Success = false
			attempt.Error = normalizePingError(probeErr, string(output))
			result.FailureCount++
		} else {
			attempt.Success = true
			latencyMs, ok := parsePingLatency(output)
			if !ok {
				latencyMs = durationToMs(time.Since(startedAt))
				addWarning(warningSet, fmt.Sprintf("第 %d 次探测未解析到时延，已使用命令耗时替代", seq))
			}
			attempt.LatencyMs = latencyMs
			if ttl, ok := parsePingTTL(output); ok {
				attempt.TTL = ttl
			}
			result.SuccessCount++
			latencies = append(latencies, latencyMs)
		}

		result.Attempts = append(result.Attempts, attempt)

		if seq < prepared.Count && prepared.Interval > 0 {
			timer := time.NewTimer(prepared.Interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, mapContextError(ctx.Err(), "Ping 测试超时")
			case <-timer.C:
			}
		}
	}

	result.FailureCount = result.Count - result.SuccessCount
	if result.Count > 0 {
		result.LossRate = float64(result.FailureCount) * 100 / float64(result.Count)
	}
	if len(latencies) > 0 {
		result.MinLatencyMs = latencies[0]
		result.MaxLatencyMs = latencies[0]
		for _, value := range latencies {
			if value < result.MinLatencyMs {
				result.MinLatencyMs = value
			}
			if value > result.MaxLatencyMs {
				result.MaxLatencyMs = value
			}
		}
		result.AvgLatencyMs = average(latencies)
		result.JitterMs = computeJitter(latencies)
	}
	result.Warnings = warningSlice(warningSet)
	return result, nil
}

func normalizePingOptions(opts PingOptions) (*preparedPingOptions, error) {
	target := strings.TrimSpace(opts.Target)
	if target == "" {
		return nil, errs.InvalidArgument("target 不能为空")
	}
	count := opts.Count
	if count == 0 {
		count = defaultPingCount
	}
	if count < 0 {
		return nil, errs.InvalidArgument("--count 不能小于 0")
	}
	interval := opts.Interval
	if interval == 0 {
		interval = defaultPingInterval
	}
	if interval < 0 {
		return nil, errs.InvalidArgument("--interval 不能小于 0")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultPingTimeout
	}
	size := opts.Size
	if size == 0 {
		size = defaultPingSize
	}
	if size <= 0 || size > maxPingSize {
		return nil, errs.InvalidArgument(fmt.Sprintf("--size 仅支持 1-%d 字节", maxPingSize))
	}
	return &preparedPingOptions{
		Target:   target,
		Count:    count,
		Interval: interval,
		Timeout:  timeout,
		Size:     size,
	}, nil
}

func runPingOnce(ctx context.Context, opts *preparedPingOptions) ([]byte, error) {
	name, args := buildPingCommand(opts)
	return commandRunner(ctx, name, args...)
}

func buildPingCommand(opts *preparedPingOptions) (string, []string) {
	timeoutMs := opts.Timeout.Milliseconds()
	if timeoutMs <= 0 {
		timeoutMs = 1
	}

	switch runtimeName {
	case "windows":
		return "ping", []string{
			"-n", "1",
			"-w", strconv.FormatInt(timeoutMs, 10),
			"-l", strconv.Itoa(opts.Size),
			opts.Target,
		}
	case "darwin":
		return "ping", []string{
			"-c", "1",
			"-W", strconv.FormatInt(timeoutMs, 10),
			"-s", strconv.Itoa(opts.Size),
			opts.Target,
		}
	default:
		timeoutSec := int(math.Ceil(opts.Timeout.Seconds()))
		if timeoutSec <= 0 {
			timeoutSec = 1
		}
		return "ping", []string{
			"-c", "1",
			"-W", strconv.Itoa(timeoutSec),
			"-s", strconv.Itoa(opts.Size),
			opts.Target,
		}
	}
}

func parsePingLatency(output []byte) (float64, bool) {
	matches := pingLatencyPattern.FindSubmatch(output)
	if len(matches) < 2 {
		return 0, false
	}
	return parseFloatToken(string(matches[1]))
}

func parsePingTTL(output []byte) (int, bool) {
	matches := pingTTLPattern.FindSubmatch(output)
	if len(matches) < 2 {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(matches[1])))
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func normalizePingError(err error, output string) string {
	if ctxErr := mapContextError(err, ""); ctxErr != nil {
		return "timeout"
	}
	text := strings.ToLower(strings.TrimSpace(output))
	switch {
	case strings.Contains(text, "timed out"),
		strings.Contains(text, "timeout"),
		strings.Contains(text, "超时"):
		return "timeout"
	case strings.Contains(text, "unreachable"),
		strings.Contains(text, "不可达"):
		return "unreachable"
	case strings.Contains(text, "unknown host"),
		strings.Contains(text, "could not find host"),
		strings.Contains(text, "name or service not known"),
		strings.Contains(text, "找不到主机"):
		return "resolve_failed"
	}
	if line := firstUsefulLine(text); line != "" {
		return line
	}
	if err != nil {
		return strings.TrimSpace(err.Error())
	}
	return "probe_failed"
}

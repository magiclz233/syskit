package port

import (
	"context"
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syskit/internal/errs"
	"time"
)

const (
	defaultPingCount      = 4
	defaultProbeTimeout   = time.Second
	defaultProbeInterval  = 200 * time.Millisecond
	defaultScanTimeout    = 500 * time.Millisecond
	maxScanWorkerPoolSize = 128
)

// PingPort 对目标端口执行多次 TCP 建连探测，用于判断可达性和时延。
func PingPort(ctx context.Context, opts PingOptions) (*PingResult, error) {
	normalized, err := normalizePingOptions(opts)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	result := &PingResult{
		Target:     normalized.Target,
		Port:       normalized.Port,
		Count:      normalized.Count,
		TimeoutMs:  normalized.Timeout.Milliseconds(),
		IntervalMs: normalized.Interval.Milliseconds(),
		Attempts:   make([]PingAttempt, 0, normalized.Count),
	}

	successLatency := make([]float64, 0, normalized.Count)
	for i := 1; i <= normalized.Count; i++ {
		if err := ctx.Err(); err != nil {
			if timeoutErr := mapTimeoutError(err); timeoutErr != nil {
				return nil, timeoutErr
			}
			return nil, errs.ExecutionFailed("端口探测被取消", err)
		}

		attempt := PingAttempt{Seq: i}
		open, latency, probeErr := probeTCP(ctx, normalized.Target, normalized.Port, normalized.Timeout)
		if probeErr != nil {
			if errors.Is(probeErr, context.DeadlineExceeded) || errors.Is(probeErr, context.Canceled) {
				if timeoutErr := mapTimeoutError(probeErr); timeoutErr != nil {
					return nil, timeoutErr
				}
				return nil, errs.ExecutionFailed("端口探测被取消", probeErr)
			}
			attempt.Success = false
			attempt.Error = normalizeProbeError(probeErr)
			result.FailureCount++
		} else {
			attempt.Success = open
			attempt.LatencyMs = durationMilliseconds(latency)
			result.SuccessCount++
			successLatency = append(successLatency, attempt.LatencyMs)
		}
		result.Attempts = append(result.Attempts, attempt)

		if i == normalized.Count || normalized.Interval <= 0 {
			continue
		}
		timer := time.NewTimer(normalized.Interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if timeoutErr := mapTimeoutError(ctx.Err()); timeoutErr != nil {
				return nil, timeoutErr
			}
			return nil, errs.ExecutionFailed("端口探测被取消", ctx.Err())
		case <-timer.C:
		}
	}

	result.FailureCount = result.Count - result.SuccessCount
	if result.Count > 0 {
		result.SuccessRate = (float64(result.SuccessCount) / float64(result.Count)) * 100
	}
	if len(successLatency) > 0 {
		minVal, maxVal, avgVal := summarizeLatency(successLatency)
		result.MinLatencyMs = minVal
		result.MaxLatencyMs = maxVal
		result.AvgLatencyMs = avgVal
	}
	return result, nil
}

// ScanPorts 对端口集合做并发 TCP 扫描，返回开放端口清单和逐端口结果。
func ScanPorts(ctx context.Context, opts ScanOptions) (*ScanResult, error) {
	normalized, err := normalizeScanOptions(opts)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	results := make([]ScanPortResult, 0, len(normalized.Ports))
	resultCh := make(chan ScanPortResult, len(normalized.Ports))
	errCh := make(chan error, 1)
	jobCh := make(chan int)

	workerCount := scanWorkerCount(len(normalized.Ports))
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for port := range jobCh {
				if err := ctx.Err(); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
				open, latency, probeErr := probeTCP(ctx, normalized.Target, port, normalized.Timeout)
				item := ScanPortResult{Port: port}
				if probeErr != nil {
					if errors.Is(probeErr, context.DeadlineExceeded) || errors.Is(probeErr, context.Canceled) {
						select {
						case errCh <- probeErr:
						default:
						}
						return
					}
					item.Open = false
					item.Error = normalizeProbeError(probeErr)
				} else {
					item.Open = open
					item.LatencyMs = durationMilliseconds(latency)
				}
				resultCh <- item
			}
		}()
	}

sendLoop:
	for _, port := range normalized.Ports {
		select {
		case <-ctx.Done():
			select {
			case errCh <- ctx.Err():
			default:
			}
			break sendLoop
		case jobCh <- port:
		}
	}
	close(jobCh)
	wg.Wait()
	close(resultCh)

	for item := range resultCh {
		results = append(results, item)
	}

	select {
	case scanErr := <-errCh:
		if timeoutErr := mapTimeoutError(scanErr); timeoutErr != nil {
			return nil, timeoutErr
		}
		return nil, errs.ExecutionFailed("端口扫描失败", scanErr)
	default:
	}

	sort.Slice(results, func(i int, j int) bool {
		return results[i].Port < results[j].Port
	})

	openPorts := make([]int, 0, len(results))
	for _, item := range results {
		if item.Open {
			openPorts = append(openPorts, item.Port)
		}
	}

	return &ScanResult{
		Target:      normalized.Target,
		Mode:        string(normalized.Mode),
		TotalPorts:  len(normalized.Ports),
		OpenCount:   len(openPorts),
		ClosedCount: len(normalized.Ports) - len(openPorts),
		OpenPorts:   openPorts,
		Results:     results,
		TimeoutMs:   normalized.Timeout.Milliseconds(),
	}, nil
}

func normalizePingOptions(opts PingOptions) (PingOptions, error) {
	normalized := PingOptions{
		Target:   strings.TrimSpace(opts.Target),
		Port:     opts.Port,
		Count:    opts.Count,
		Timeout:  opts.Timeout,
		Interval: opts.Interval,
	}
	if normalized.Target == "" {
		return PingOptions{}, errs.InvalidArgument("target 不能为空")
	}
	if normalized.Port <= 0 || normalized.Port > 65535 {
		return PingOptions{}, errs.InvalidArgument("port 必须在 1-65535 之间")
	}
	if normalized.Count <= 0 {
		normalized.Count = defaultPingCount
	}
	if normalized.Timeout <= 0 {
		normalized.Timeout = defaultProbeTimeout
	}
	if normalized.Interval < 0 {
		return PingOptions{}, errs.InvalidArgument("--interval 不能小于 0")
	}
	if normalized.Interval == 0 && normalized.Count > 1 {
		normalized.Interval = defaultProbeInterval
	}
	return normalized, nil
}

func normalizeScanOptions(opts ScanOptions) (ScanOptions, error) {
	normalized := ScanOptions{
		Target:  strings.TrimSpace(opts.Target),
		Mode:    opts.Mode,
		Ports:   append([]int(nil), opts.Ports...),
		Timeout: opts.Timeout,
	}
	if normalized.Target == "" {
		return ScanOptions{}, errs.InvalidArgument("target 不能为空")
	}
	if normalized.Mode != ScanModeQuick && normalized.Mode != ScanModeFull {
		normalized.Mode = ScanModeQuick
	}
	if normalized.Timeout <= 0 {
		normalized.Timeout = defaultScanTimeout
	}
	if len(normalized.Ports) == 0 {
		return ScanOptions{}, errs.InvalidArgument("扫描端口集合不能为空")
	}

	portSet := make(map[int]struct{}, len(normalized.Ports))
	ports := make([]int, 0, len(normalized.Ports))
	for _, port := range normalized.Ports {
		if port <= 0 || port > 65535 {
			return ScanOptions{}, errs.InvalidArgument("扫描端口必须在 1-65535 之间")
		}
		if _, ok := portSet[port]; ok {
			continue
		}
		portSet[port] = struct{}{}
		ports = append(ports, port)
	}
	sort.Ints(ports)
	normalized.Ports = ports
	return normalized, nil
}

func probeTCP(ctx context.Context, target string, port int, timeout time.Duration) (bool, time.Duration, error) {
	address := net.JoinHostPort(target, strconv.Itoa(port))
	startedAt := time.Now()
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	latency := time.Since(startedAt)
	if err != nil {
		return false, latency, err
	}
	_ = conn.Close()
	return true, latency, nil
}

func normalizeProbeError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "connection refused"):
		return "connection_refused"
	case strings.Contains(message, "actively refused"):
		return "connection_refused"
	case strings.Contains(message, "no such host"):
		return "host_not_found"
	case strings.Contains(message, "network is unreachable"):
		return "network_unreachable"
	case strings.Contains(message, "too many open files"):
		return "resource_exhausted"
	default:
		if message == "" {
			return "unknown_error"
		}
		return message
	}
}

func summarizeLatency(values []float64) (minVal float64, maxVal float64, avgVal float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	minVal = values[0]
	maxVal = values[0]
	total := 0.0
	for _, value := range values {
		if value < minVal {
			minVal = value
		}
		if value > maxVal {
			maxVal = value
		}
		total += value
	}
	avgVal = total / float64(len(values))
	return minVal, maxVal, avgVal
}

func durationMilliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func scanWorkerCount(totalPorts int) int {
	if totalPorts <= 1 {
		return 1
	}
	if totalPorts < maxScanWorkerPoolSize {
		return totalPorts
	}
	return maxScanWorkerPoolSize
}

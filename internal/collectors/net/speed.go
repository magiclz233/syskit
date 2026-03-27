package net

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"syskit/internal/errs"
)

const (
	defaultSpeedServer        = "https://speed.cloudflare.com"
	defaultSpeedTimeout       = 15 * time.Second
	defaultSpeedPingCount     = 3
	defaultSpeedDownloadBytes = int64(2 * 1024 * 1024)
	defaultSpeedUploadBytes   = int64(512 * 1024)
)

var speedHTTPClientFactory = func(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

type preparedSpeedOptions struct {
	ServerURL   *url.URL
	Mode        SpeedMode
	Timeout     time.Duration
	PingURL     string
	DownloadURL string
	UploadURL   string
}

// CollectSpeed 执行 `net speed` 所需的延迟、下载和上传测速。
func CollectSpeed(ctx context.Context, opts SpeedOptions) (*SpeedResult, error) {
	prepared, err := normalizeSpeedOptions(opts)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	client := speedHTTPClientFactory(prepared.Timeout)
	startedAt := time.Now()
	result := &SpeedResult{
		Server: prepared.ServerURL.String(),
		Mode:   string(prepared.Mode),
	}
	warningSet := make(map[string]struct{})

	if prepared.Mode == SpeedModeFull {
		pingStats, publicIP, warnings, err := measureHTTPPing(ctx, client, prepared)
		if err != nil {
			return nil, err
		}
		result.Ping = pingStats
		result.PublicIP = publicIP
		for _, warning := range warnings {
			addWarning(warningSet, warning)
		}
	}

	if prepared.Mode == SpeedModeFull || prepared.Mode == SpeedModeDownload {
		download, err := measureHTTPDownload(ctx, client, prepared)
		if err != nil {
			return nil, err
		}
		result.Download = download
	}

	if prepared.Mode == SpeedModeFull || prepared.Mode == SpeedModeUpload {
		upload, err := measureHTTPUpload(ctx, client, prepared)
		if err != nil {
			return nil, err
		}
		result.Upload = upload
	}

	result.DurationMs = durationMs(time.Since(startedAt))
	result.Warnings = warningSlice(warningSet)
	return result, nil
}

func normalizeSpeedOptions(opts SpeedOptions) (*preparedSpeedOptions, error) {
	mode := opts.Mode
	if mode == "" {
		mode = SpeedModeFull
	}
	switch mode {
	case SpeedModeFull, SpeedModeDownload, SpeedModeUpload:
	default:
		return nil, errs.InvalidArgument(fmt.Sprintf("不支持的测速模式: %s", mode))
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultSpeedTimeout
	}

	rawServer := strings.TrimSpace(opts.Server)
	if rawServer == "" {
		rawServer = defaultSpeedServer
	}
	if !strings.Contains(rawServer, "://") {
		rawServer = "https://" + rawServer
	}
	serverURL, err := url.Parse(rawServer)
	if err != nil || strings.TrimSpace(serverURL.Host) == "" {
		return nil, errs.InvalidArgument(fmt.Sprintf("--server 无效: %s", opts.Server))
	}

	pingURL := resolveSpeedEndpoint(serverURL, "/cdn-cgi/trace")
	downURL := resolveSpeedEndpoint(serverURL, "/__down?bytes="+strconv.FormatInt(defaultSpeedDownloadBytes, 10))
	upURL := resolveSpeedEndpoint(serverURL, "/__up")

	return &preparedSpeedOptions{
		ServerURL:   serverURL,
		Mode:        mode,
		Timeout:     timeout,
		PingURL:     pingURL,
		DownloadURL: downURL,
		UploadURL:   upURL,
	}, nil
}

func resolveSpeedEndpoint(baseURL *url.URL, path string) string {
	if baseURL == nil {
		return ""
	}
	ref, err := url.Parse(path)
	if err != nil {
		return baseURL.String()
	}
	return baseURL.ResolveReference(ref).String()
}

func measureHTTPPing(ctx context.Context, client *http.Client, opts *preparedSpeedOptions) (*SpeedPingStats, string, []string, error) {
	latencies := make([]float64, 0, defaultSpeedPingCount)
	warnings := make([]string, 0, 1)
	publicIP := ""
	successCount := 0
	failureCount := 0

	for idx := 0; idx < defaultSpeedPingCount; idx++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.PingURL, nil)
		if err != nil {
			return nil, "", nil, wrapSpeedError("构建延迟测试请求", err)
		}

		startedAt := time.Now()
		response, err := client.Do(request)
		if err != nil {
			failureCount++
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		_ = response.Body.Close()
		if readErr != nil {
			failureCount++
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 400 {
			failureCount++
			continue
		}
		successCount++
		latencies = append(latencies, durationMs(time.Since(startedAt)))
		if publicIP == "" {
			publicIP = extractTraceIP(string(body))
		}
	}

	if successCount == 0 {
		return nil, "", nil, errs.NewWithSuggestion(
			errs.ExitExecutionFailed,
			errs.CodeExecutionFailed,
			"网络延迟测试失败，无法连接测速服务",
			"请检查网络连通性或通过 --server 指定可访问的测速服务",
		)
	}

	if failureCount > 0 {
		warnings = append(warnings, fmt.Sprintf("延迟测试有 %d 次失败", failureCount))
	}
	if publicIP == "" {
		warnings = append(warnings, "未从测速服务返回中解析到公网 IP")
	}

	stats := &SpeedPingStats{
		Count:        defaultSpeedPingCount,
		SuccessCount: successCount,
		FailureCount: failureCount,
		LossRate:     float64(failureCount) * 100 / float64(defaultSpeedPingCount),
		MinMs:        latencies[0],
		MaxMs:        latencies[0],
		AvgMs:        averageLatency(latencies),
		JitterMs:     jitterLatency(latencies),
	}
	for _, value := range latencies {
		if value < stats.MinMs {
			stats.MinMs = value
		}
		if value > stats.MaxMs {
			stats.MaxMs = value
		}
	}
	return stats, publicIP, warnings, nil
}

func measureHTTPDownload(ctx context.Context, client *http.Client, opts *preparedSpeedOptions) (*SpeedSample, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.DownloadURL, nil)
	if err != nil {
		return nil, wrapSpeedError("构建下载测速请求", err)
	}
	startedAt := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return nil, wrapSpeedError("下载测速", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return nil, errs.New(
			errs.ExitExecutionFailed,
			errs.CodeExecutionFailed,
			fmt.Sprintf("下载测速失败: 服务返回 %s", response.Status),
		)
	}
	bytesRead, err := io.Copy(io.Discard, response.Body)
	if err != nil {
		return nil, wrapSpeedError("读取下载流", err)
	}
	if bytesRead <= 0 {
		return nil, errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "下载测速失败: 未读取到有效数据")
	}
	elapsed := time.Since(startedAt)
	return &SpeedSample{
		Bytes:      bytesRead,
		DurationMs: durationMs(elapsed),
		Mbps:       toMbps(bytesRead, elapsed),
	}, nil
}

func measureHTTPUpload(ctx context.Context, client *http.Client, opts *preparedSpeedOptions) (*SpeedSample, error) {
	payload := bytes.Repeat([]byte("x"), int(defaultSpeedUploadBytes))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.UploadURL, bytes.NewReader(payload))
	if err != nil {
		return nil, wrapSpeedError("构建上传测速请求", err)
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	startedAt := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return nil, wrapSpeedError("上传测速", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return nil, errs.New(
			errs.ExitExecutionFailed,
			errs.CodeExecutionFailed,
			fmt.Sprintf("上传测速失败: 服务返回 %s", response.Status),
		)
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		return nil, wrapSpeedError("读取上传响应", err)
	}
	elapsed := time.Since(startedAt)
	bytesSent := int64(len(payload))
	return &SpeedSample{
		Bytes:      bytesSent,
		DurationMs: durationMs(elapsed),
		Mbps:       toMbps(bytesSent, elapsed),
	}, nil
}

func wrapSpeedError(action string, err error) error {
	if err == nil {
		return nil
	}
	if timeoutErr := mapTimeoutError(err); timeoutErr != nil {
		return timeoutErr
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		if timeoutErr := mapTimeoutError(err); timeoutErr != nil {
			return timeoutErr
		}
	}
	return errs.ExecutionFailed(action+"失败", err)
}

func extractTraceIP(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "ip=") {
			continue
		}
		return strings.TrimSpace(line[3:])
	}
	return ""
}

func durationMs(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func toMbps(bytesCount int64, duration time.Duration) float64 {
	if bytesCount <= 0 || duration <= 0 {
		return 0
	}
	return float64(bytesCount*8) / duration.Seconds() / 1_000_000
}

func averageLatency(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func jitterLatency(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	total := 0.0
	for idx := 1; idx < len(values); idx++ {
		total += math.Abs(values[idx] - values[idx-1])
	}
	return total / float64(len(values)-1)
}


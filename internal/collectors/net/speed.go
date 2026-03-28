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
		emitSpeedProgress(opts.Progress, "ping", "开始延迟与公网出口探测")
		stageStartedAt := time.Now()
		pingStats, traceInfo, warnings, err := measureHTTPPing(ctx, client, prepared)
		if err != nil {
			return nil, err
		}
		result.Ping = pingStats
		result.Trace = traceInfo
		if traceInfo != nil {
			result.PublicIP = traceInfo.PublicIP
		}
		result.Phases = append(result.Phases, SpeedPhase{
			Name:       "ping",
			Status:     "ok",
			DurationMs: durationMs(time.Since(stageStartedAt)),
			Detail:     buildPingPhaseDetail(pingStats, traceInfo),
		})
		for _, warning := range warnings {
			addWarning(warningSet, warning)
		}
		emitSpeedProgress(opts.Progress, "ping", "延迟与出口探测完成")
	}

	if prepared.Mode == SpeedModeFull || prepared.Mode == SpeedModeDownload {
		emitSpeedProgress(opts.Progress, "download", "开始下载测速")
		stageStartedAt := time.Now()
		download, err := measureHTTPDownload(ctx, client, prepared)
		if err != nil {
			return nil, err
		}
		result.Download = download
		result.Phases = append(result.Phases, SpeedPhase{
			Name:       "download",
			Status:     "ok",
			DurationMs: durationMs(time.Since(stageStartedAt)),
			Bytes:      download.Bytes,
			Mbps:       download.Mbps,
			Detail:     buildSamplePhaseDetail("下载", download),
		})
		emitSpeedProgress(opts.Progress, "download", "下载测速完成")
	}

	if prepared.Mode == SpeedModeFull || prepared.Mode == SpeedModeUpload {
		emitSpeedProgress(opts.Progress, "upload", "开始上传测速")
		stageStartedAt := time.Now()
		upload, err := measureHTTPUpload(ctx, client, prepared)
		if err != nil {
			return nil, err
		}
		result.Upload = upload
		result.Phases = append(result.Phases, SpeedPhase{
			Name:       "upload",
			Status:     "ok",
			DurationMs: durationMs(time.Since(stageStartedAt)),
			Bytes:      upload.Bytes,
			Mbps:       upload.Mbps,
			Detail:     buildSamplePhaseDetail("上传", upload),
		})
		emitSpeedProgress(opts.Progress, "upload", "上传测速完成")
	}

	result.DurationMs = durationMs(time.Since(startedAt))
	result.Assessment = buildSpeedAssessment(result)
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

func measureHTTPPing(ctx context.Context, client *http.Client, opts *preparedSpeedOptions) (*SpeedPingStats, *SpeedTraceInfo, []string, error) {
	latencies := make([]float64, 0, defaultSpeedPingCount)
	warnings := make([]string, 0, 1)
	traceFields := map[string]string{}
	successCount := 0
	failureCount := 0

	for idx := 0; idx < defaultSpeedPingCount; idx++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.PingURL, nil)
		if err != nil {
			return nil, nil, nil, wrapSpeedError("构建延迟测试请求", err)
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
		if len(traceFields) == 0 {
			traceFields = parseTraceFields(string(body))
			mergeMissingTraceFields(traceFields, parseTraceHeaderFields(response.Header))
		}
	}

	if successCount == 0 {
		return nil, nil, nil, errs.NewWithSuggestion(
			errs.ExitExecutionFailed,
			errs.CodeExecutionFailed,
			"网络延迟测试失败，无法连接测速服务",
			"请检查网络连通性或通过 --server 指定可访问的测速服务",
		)
	}

	if failureCount > 0 {
		warnings = append(warnings, fmt.Sprintf("延迟测试有 %d 次失败", failureCount))
	}
	traceInfo := buildTraceInfo(traceFields)
	if traceInfo == nil || traceInfo.PublicIP == "" {
		warnings = append(warnings, "未从测速服务返回中解析到公网 IP")
	}
	if traceInfo == nil || strings.TrimSpace(traceInfo.Operator) == "" {
		warnings = append(warnings, "测速服务未返回运营商信息")
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
	return stats, traceInfo, warnings, nil
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

func emitSpeedProgress(callback func(SpeedProgressEvent), stage string, message string) {
	if callback == nil {
		return
	}
	callback(SpeedProgressEvent{
		Stage:   stage,
		Message: message,
	})
}

func buildPingPhaseDetail(stats *SpeedPingStats, traceInfo *SpeedTraceInfo) string {
	if stats == nil {
		return ""
	}
	detail := fmt.Sprintf(
		"%d 次延迟探测，平均 %.2fms，抖动 %.2fms，丢包 %.1f%%",
		stats.Count,
		stats.AvgMs,
		stats.JitterMs,
		stats.LossRate,
	)
	if traceInfo == nil {
		return detail
	}

	parts := []string{detail}
	if strings.TrimSpace(traceInfo.PublicIP) != "" {
		parts = append(parts, "公网 IP "+traceInfo.PublicIP)
	}
	if strings.TrimSpace(traceInfo.Location) != "" || strings.TrimSpace(traceInfo.Colo) != "" {
		parts = append(parts, fmt.Sprintf("出口 loc=%s colo=%s", displayTraceValue(traceInfo.Location), displayTraceValue(traceInfo.Colo)))
	}
	return strings.Join(parts, "，")
}

func buildSamplePhaseDetail(name string, sample *SpeedSample) string {
	if sample == nil {
		return ""
	}
	return fmt.Sprintf("%s %.0f 字节，耗时 %.2fms，平均 %.2f Mbps", name, float64(sample.Bytes), sample.DurationMs, sample.Mbps)
}

func parseTraceFields(raw string) map[string]string {
	fields := make(map[string]string)
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		fields[key] = value
	}
	return fields
}

func parseTraceHeaderFields(headers http.Header) map[string]string {
	fields := make(map[string]string)
	if headers == nil {
		return fields
	}

	appendHeaderField(fields, headers, "operator", "CF-Meta-ASN-Org")
	appendHeaderField(fields, headers, "operator", "X-Syskit-Operator")
	appendHeaderField(fields, headers, "colo", "CF-RAY")
	return fields
}

func mergeMissingTraceFields(dest map[string]string, src map[string]string) {
	if dest == nil || len(src) == 0 {
		return
	}
	for key, value := range src {
		if strings.TrimSpace(dest[key]) != "" {
			continue
		}
		dest[key] = value
	}
}

func appendHeaderField(fields map[string]string, headers http.Header, key string, headerName string) {
	if len(fields[key]) > 0 {
		return
	}
	value := strings.TrimSpace(headers.Get(headerName))
	if value == "" {
		return
	}
	if key == "colo" {
		parts := strings.Split(value, "-")
		value = strings.TrimSpace(parts[len(parts)-1])
	}
	if value != "" {
		fields[key] = value
	}
}

func buildTraceInfo(fields map[string]string) *SpeedTraceInfo {
	if len(fields) == 0 {
		return nil
	}

	traceInfo := &SpeedTraceInfo{
		PublicIP:    strings.TrimSpace(fields["ip"]),
		Location:    strings.ToUpper(strings.TrimSpace(fields["loc"])),
		Colo:        strings.ToUpper(strings.TrimSpace(fields["colo"])),
		VisitScheme: strings.TrimSpace(fields["visit_scheme"]),
		Operator:    firstNonEmpty(fields["operator"], fields["org"], fields["asn_org"]),
	}
	if traceInfo.PublicIP == "" && traceInfo.Location == "" && traceInfo.Colo == "" && traceInfo.VisitScheme == "" && traceInfo.Operator == "" {
		return nil
	}
	return traceInfo
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func buildSpeedAssessment(result *SpeedResult) *SpeedAssessment {
	if result == nil {
		return nil
	}

	highlights := make([]string, 0, 4)
	summaryParts := make([]string, 0, 3)

	if result.Ping != nil {
		latencySummary, latencyHighlight := summarizeLatency(result.Ping)
		if latencySummary != "" {
			summaryParts = append(summaryParts, latencySummary)
		}
		if latencyHighlight != "" {
			highlights = append(highlights, latencyHighlight)
		}
	}
	if result.Download != nil {
		downloadSummary, downloadHighlight := summarizeThroughput("下载", result.Download.Mbps)
		if downloadSummary != "" {
			summaryParts = append(summaryParts, downloadSummary)
		}
		if downloadHighlight != "" {
			highlights = append(highlights, downloadHighlight)
		}
	}
	if result.Upload != nil {
		uploadSummary, uploadHighlight := summarizeThroughput("上传", result.Upload.Mbps)
		if uploadSummary != "" {
			summaryParts = append(summaryParts, uploadSummary)
		}
		if uploadHighlight != "" {
			highlights = append(highlights, uploadHighlight)
		}
	}
	if result.Download != nil && result.Upload != nil && result.Upload.Mbps > 0 {
		ratio := result.Download.Mbps / result.Upload.Mbps
		switch {
		case ratio >= 4:
			highlights = append(highlights, "上下行差异明显，当前链路更偏向下载型业务。")
		case ratio <= 1.5:
			highlights = append(highlights, "上下行较为均衡，适合文件同步或远程协作。")
		}
	}
	if result.Trace != nil && strings.TrimSpace(result.Trace.Operator) != "" {
		highlights = append(highlights, "测速服务返回运营商信息："+result.Trace.Operator)
	}

	summary := strings.Join(summaryParts, "；")
	if summary == "" && len(highlights) == 0 {
		return nil
	}
	return &SpeedAssessment{
		Summary:    summary,
		Highlights: highlights,
	}
}

func summarizeLatency(stats *SpeedPingStats) (string, string) {
	if stats == nil {
		return "", ""
	}
	switch {
	case stats.AvgMs <= 20:
		return "延迟优秀", "平均延迟很低，交互型业务通常不会感觉到明显等待。"
	case stats.AvgMs <= 60:
		return "延迟良好", "平均延迟处于健康区间，日常办公和 API 调用通常稳定。"
	case stats.AvgMs <= 120:
		return "延迟一般", "延迟已有感知，远程桌面或语音通话高峰期可能受影响。"
	default:
		return "延迟偏高", "延迟较高，实时互动、远程操作或链路敏感业务可能抖动明显。"
	}
}

func summarizeThroughput(direction string, mbps float64) (string, string) {
	if mbps <= 0 {
		return "", ""
	}
	switch {
	case mbps >= 200:
		return direction + "带宽充足", direction + "速度较高，可支撑镜像分发、大文件传输等重流量场景。"
	case mbps >= 50:
		return direction + "带宽良好", direction + "速度处于常见宽带/办公网络可接受区间。"
	case mbps >= 10:
		return direction + "带宽一般", direction + "速度可满足常规网页、视频会议和中小文件传输。"
	default:
		return direction + "带宽偏低", direction + "速度偏低，大文件传输、备份或回传任务会明显变慢。"
	}
}

func displayTraceValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

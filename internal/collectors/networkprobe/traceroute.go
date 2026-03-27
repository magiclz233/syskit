package networkprobe

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"syskit/internal/errs"
	"time"
	"unicode"
)

const (
	defaultTracerouteMaxHops = 30
	defaultTracerouteTimeout = 2 * time.Second
)

var (
	tracerouteHopPattern     = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)
	tracerouteLatencyPattern = regexp.MustCompile(`([0-9]+(?:[.,][0-9]+)?)\s*ms`)
)

type preparedTracerouteOptions struct {
	Target   string
	MaxHops  int
	Timeout  time.Duration
	Protocol TraceProtocol
}

// Traceroute 对目标执行系统路由跟踪并解析逐跳结果。
func Traceroute(ctx context.Context, opts TracerouteOptions) (*TracerouteResult, error) {
	prepared, err := normalizeTracerouteOptions(opts)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	command, args, commandWarnings := buildTracerouteCommand(prepared)
	output, runErr := commandRunner(ctx, command, args...)
	if isCommandNotFound(runErr) {
		return nil, commandUnavailableError(command)
	}
	if ctxErr := mapContextError(ctx.Err(), "路由跟踪超时"); ctxErr != nil {
		return nil, ctxErr
	}

	hops := parseTracerouteHops(output)
	if len(hops) == 0 {
		if runErr != nil {
			return nil, errs.ExecutionFailed("路由跟踪执行失败", runErr)
		}
		return nil, errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "路由跟踪未返回可解析结果")
	}

	warningSet := make(map[string]struct{})
	for _, warning := range commandWarnings {
		addWarning(warningSet, warning)
	}
	if runErr != nil {
		addWarning(warningSet, "路由跟踪命令返回非零退出码，结果可能不完整")
		if line := firstUsefulLine(string(output)); line != "" {
			addWarning(warningSet, "命令输出: "+line)
		}
	}

	reached := detectTracerouteReached(prepared, hops, runErr)
	if !reached {
		addWarning(warningSet, "未在最大跳数内确认到达目标")
	}

	return &TracerouteResult{
		Target:    prepared.Target,
		Protocol:  string(prepared.Protocol),
		MaxHops:   prepared.MaxHops,
		TimeoutMs: prepared.Timeout.Milliseconds(),
		HopCount:  len(hops),
		Reached:   reached,
		Hops:      hops,
		Warnings:  warningSlice(warningSet),
	}, nil
}

func normalizeTracerouteOptions(opts TracerouteOptions) (*preparedTracerouteOptions, error) {
	target := strings.TrimSpace(opts.Target)
	if target == "" {
		return nil, errs.InvalidArgument("target 不能为空")
	}
	maxHops := opts.MaxHops
	if maxHops == 0 {
		maxHops = defaultTracerouteMaxHops
	}
	if maxHops <= 0 {
		return nil, errs.InvalidArgument("--max-hops 必须大于 0")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTracerouteTimeout
	}
	protocol := opts.Protocol
	if protocol == "" {
		protocol = TraceProtocolICMP
	}
	if protocol != TraceProtocolICMP && protocol != TraceProtocolTCP {
		return nil, errs.InvalidArgument(fmt.Sprintf("不支持的 traceroute 协议: %s", protocol))
	}
	return &preparedTracerouteOptions{
		Target:   target,
		MaxHops:  maxHops,
		Timeout:  timeout,
		Protocol: protocol,
	}, nil
}

func buildTracerouteCommand(opts *preparedTracerouteOptions) (string, []string, []string) {
	timeoutMs := opts.Timeout.Milliseconds()
	if timeoutMs <= 0 {
		timeoutMs = 1
	}

	switch runtimeName {
	case "windows":
		warnings := []string(nil)
		if opts.Protocol == TraceProtocolTCP {
			warnings = append(warnings, "Windows 平台 tracert 不支持 TCP 探测，已降级为 ICMP")
		}
		return "tracert", []string{
			"-d",
			"-h", strconv.Itoa(opts.MaxHops),
			"-w", strconv.FormatInt(timeoutMs, 10),
			opts.Target,
		}, warnings
	default:
		args := []string{
			"-n",
			"-m", strconv.Itoa(opts.MaxHops),
			"-w", formatUnixTraceTimeout(opts.Timeout),
		}
		if opts.Protocol == TraceProtocolTCP {
			args = append(args, "-T")
		} else {
			args = append(args, "-I")
		}
		args = append(args, opts.Target)
		return "traceroute", args, nil
	}
}

func formatUnixTraceTimeout(timeout time.Duration) string {
	seconds := timeout.Seconds()
	if seconds <= 0 {
		seconds = 1
	}
	return strconv.FormatFloat(seconds, 'f', 1, 64)
}

func parseTracerouteHops(output []byte) []TracerouteHop {
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	hops := make([]TracerouteHop, 0, len(lines))

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		matches := tracerouteHopPattern.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		hopIndex, err := strconv.Atoi(matches[1])
		if err != nil || hopIndex <= 0 {
			continue
		}
		body := strings.TrimSpace(matches[2])
		latencies := parseTracerouteLatencies(body)
		host, ip := parseTracerouteEndpoint(body)
		hop := TracerouteHop{
			Hop:     hopIndex,
			Host:    host,
			IP:      ip,
			RTTsMs:  latencies,
			Timeout: len(latencies) == 0 || strings.Contains(body, "*"),
		}
		if len(latencies) > 0 {
			hop.AvgLatencyMs = average(latencies)
		}
		hops = append(hops, hop)
	}
	return hops
}

func parseTracerouteLatencies(body string) []float64 {
	matches := tracerouteLatencyPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	values := make([]float64, 0, len(matches))
	for _, item := range matches {
		if len(item) < 2 {
			continue
		}
		if value, ok := parseFloatToken(item[1]); ok {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func parseTracerouteEndpoint(body string) (string, string) {
	fields := strings.FieldsFunc(body, func(r rune) bool {
		return unicode.IsSpace(r) || r == '(' || r == ')' || r == '[' || r == ']' || r == ','
	})
	for idx, field := range fields {
		candidate := strings.TrimSpace(field)
		if candidate == "" {
			continue
		}
		ip := net.ParseIP(candidate)
		if ip == nil {
			continue
		}
		host := ""
		if idx > 0 {
			previous := strings.TrimSpace(fields[idx-1])
			if isTracerouteHostToken(previous) {
				host = previous
			}
		}
		if host == "" {
			host = ip.String()
		}
		return host, ip.String()
	}
	return "", ""
}

func detectTracerouteReached(opts *preparedTracerouteOptions, hops []TracerouteHop, runErr error) bool {
	if len(hops) == 0 {
		return false
	}
	lastHop := hops[len(hops)-1]
	if lastHop.Timeout {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(opts.Target))
	if target == "" {
		return runErr == nil
	}
	if strings.EqualFold(lastHop.IP, target) || strings.EqualFold(lastHop.Host, target) {
		return true
	}
	return runErr == nil
}
func isTracerouteHostToken(token string) bool {
	normalized := strings.TrimSpace(strings.TrimPrefix(token, "<"))
	if normalized == "" || normalized == "*" {
		return false
	}
	if strings.EqualFold(normalized, "ms") {
		return false
	}
	if net.ParseIP(normalized) != nil {
		return false
	}
	if _, ok := parseFloatToken(normalized); ok {
		return false
	}
	return true
}

package traceroute

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	networkprobe "syskit/internal/collectors/networkprobe"
	"syskit/internal/errs"
)

type traceroutePresenter struct {
	result *networkprobe.TracerouteResult
}

func newTraceroutePresenter(result *networkprobe.TracerouteResult) *traceroutePresenter {
	return &traceroutePresenter{result: result}
}

func (p *traceroutePresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("路由跟踪结果为空")
	}
	fmt.Fprintf(w, "目标: %s\n", p.result.Target)
	fmt.Fprintf(w, "协议: %s, 最大跳数: %d, timeout: %dms\n", p.result.Protocol, p.result.MaxHops, p.result.TimeoutMs)
	fmt.Fprintf(w, "结果: reached=%t, hops=%d\n", p.result.Reached, p.result.HopCount)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-6s %-24s %-24s %-20s %-12s %s\n", "HOP", "HOST", "IP", "RTT_MS", "AVG_MS", "TIMEOUT")
	fmt.Fprintln(w, strings.Repeat("-", 120))
	for _, hop := range p.result.Hops {
		fmt.Fprintf(
			w,
			"%-6d %-24s %-24s %-20s %-12.2f %t\n",
			hop.Hop,
			compact(displayValue(hop.Host, "-"), 24),
			compact(displayValue(hop.IP, "-"), 24),
			compact(joinRTTs(hop.RTTsMs), 20),
			hop.AvgLatencyMs,
			hop.Timeout,
		)
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *traceroutePresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("路由跟踪结果为空")
	}
	fmt.Fprintln(w, "# Traceroute")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- target: `%s`\n", mdCell(p.result.Target))
	fmt.Fprintf(w, "- protocol: `%s`\n", p.result.Protocol)
	fmt.Fprintf(w, "- max_hops: `%d`\n", p.result.MaxHops)
	fmt.Fprintf(w, "- timeout_ms: `%d`\n", p.result.TimeoutMs)
	fmt.Fprintf(w, "- reached: `%t`\n", p.result.Reached)
	fmt.Fprintf(w, "- hop_count: `%d`\n", p.result.HopCount)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| HOP | HOST | IP | RTT_MS | AVG_MS | TIMEOUT |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|")
	for _, hop := range p.result.Hops {
		fmt.Fprintf(
			w,
			"| %d | %s | %s | %s | %.2f | %t |\n",
			hop.Hop,
			mdCell(displayValue(hop.Host, "-")),
			mdCell(displayValue(hop.IP, "-")),
			mdCell(joinRTTs(hop.RTTsMs)),
			hop.AvgLatencyMs,
			hop.Timeout,
		)
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *traceroutePresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("路由跟踪结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"target", "protocol", "hop", "host", "ip", "rtt_ms", "avg_latency_ms", "timeout"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, hop := range p.result.Hops {
		if err := writer.Write([]string{
			p.result.Target,
			p.result.Protocol,
			strconv.Itoa(hop.Hop),
			hop.Host,
			hop.IP,
			joinRTTs(hop.RTTsMs),
			fmt.Sprintf("%.2f", hop.AvgLatencyMs),
			strconv.FormatBool(hop.Timeout),
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func renderWarningsTable(w io.Writer, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w, "\n提示")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	for _, warning := range warnings {
		fmt.Fprintf(w, "- %s\n", warning)
	}
}

func renderWarningsMarkdown(w io.Writer, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w, "\n## 提示")
	fmt.Fprintln(w)
	for _, warning := range warnings {
		fmt.Fprintf(w, "- %s\n", warning)
	}
}

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func displayValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func compact(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func joinRTTs(values []float64) string {
	if len(values) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%.2f", value))
	}
	return strings.Join(parts, "/")
}

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

package ping

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	networkprobe "syskit/internal/collectors/networkprobe"
	"syskit/internal/errs"
)

type pingPresenter struct {
	result *networkprobe.PingResult
}

func newPingPresenter(result *networkprobe.PingResult) *pingPresenter {
	return &pingPresenter{result: result}
}

func (p *pingPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("Ping 结果为空")
	}
	fmt.Fprintf(w, "目标: %s\n", p.result.Target)
	fmt.Fprintf(w, "探测次数: %d, 成功: %d, 失败: %d, 丢包率: %.1f%%\n", p.result.Count, p.result.SuccessCount, p.result.FailureCount, p.result.LossRate)
	fmt.Fprintf(w, "timeout: %dms, interval: %dms, size: %dB\n", p.result.TimeoutMs, p.result.IntervalMs, p.result.Size)
	if p.result.SuccessCount > 0 {
		fmt.Fprintf(w, "时延统计(ms): min=%.2f avg=%.2f max=%.2f jitter=%.2f\n", p.result.MinLatencyMs, p.result.AvgLatencyMs, p.result.MaxLatencyMs, p.result.JitterMs)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-6s %-8s %-12s %-8s %s\n", "SEQ", "SUCCESS", "LATENCY_MS", "TTL", "ERROR")
	fmt.Fprintln(w, strings.Repeat("-", 90))
	for _, item := range p.result.Attempts {
		fmt.Fprintf(w, "%-6d %-8t %-12.2f %-8s %s\n", item.Seq, item.Success, item.LatencyMs, displayTTL(item.TTL), displayValue(item.Error, "-"))
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *pingPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("Ping 结果为空")
	}
	fmt.Fprintln(w, "# Ping")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- target: `%s`\n", mdCell(p.result.Target))
	fmt.Fprintf(w, "- count: `%d`\n", p.result.Count)
	fmt.Fprintf(w, "- success_count: `%d`\n", p.result.SuccessCount)
	fmt.Fprintf(w, "- failure_count: `%d`\n", p.result.FailureCount)
	fmt.Fprintf(w, "- loss_rate: `%.1f%%`\n", p.result.LossRate)
	fmt.Fprintf(w, "- timeout_ms: `%d`\n", p.result.TimeoutMs)
	fmt.Fprintf(w, "- interval_ms: `%d`\n", p.result.IntervalMs)
	fmt.Fprintf(w, "- size: `%d`\n", p.result.Size)
	if p.result.SuccessCount > 0 {
		fmt.Fprintf(w, "- latency_min_ms: `%.2f`\n", p.result.MinLatencyMs)
		fmt.Fprintf(w, "- latency_avg_ms: `%.2f`\n", p.result.AvgLatencyMs)
		fmt.Fprintf(w, "- latency_max_ms: `%.2f`\n", p.result.MaxLatencyMs)
		fmt.Fprintf(w, "- jitter_ms: `%.2f`\n", p.result.JitterMs)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| SEQ | SUCCESS | LATENCY_MS | TTL | ERROR |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	for _, item := range p.result.Attempts {
		fmt.Fprintf(w, "| %d | %t | %.2f | %s | %s |\n", item.Seq, item.Success, item.LatencyMs, mdCell(displayTTL(item.TTL)), mdCell(displayValue(item.Error, "-")))
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *pingPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("Ping 结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"target", "seq", "success", "latency_ms", "ttl", "error"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.result.Attempts {
		if err := writer.Write([]string{
			p.result.Target,
			strconv.Itoa(item.Seq),
			strconv.FormatBool(item.Success),
			fmt.Sprintf("%.2f", item.LatencyMs),
			displayTTL(item.TTL),
			item.Error,
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

func displayTTL(ttl int) string {
	if ttl <= 0 {
		return "-"
	}
	return strconv.Itoa(ttl)
}

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

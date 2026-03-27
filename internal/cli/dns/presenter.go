package dns

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	dnscollector "syskit/internal/collectors/dns"
	"syskit/internal/errs"
)

type resolvePresenter struct {
	result *dnscollector.ResolveResult
}

type benchPresenter struct {
	result *dnscollector.BenchResult
}

func newResolvePresenter(result *dnscollector.ResolveResult) *resolvePresenter {
	return &resolvePresenter{result: result}
}

func newBenchPresenter(result *dnscollector.BenchResult) *benchPresenter {
	return &benchPresenter{result: result}
}

func (p *resolvePresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("DNS 解析结果为空")
	}
	fmt.Fprintf(w, "域名: %s\n", p.result.Domain)
	fmt.Fprintf(w, "类型: %s\n", p.result.QueryType)
	if strings.TrimSpace(p.result.DNSServer) != "" {
		fmt.Fprintf(w, "DNS: %s\n", p.result.DNSServer)
	}
	fmt.Fprintf(w, "耗时: %.2fms\n", p.result.DurationMs)
	fmt.Fprintf(w, "记录数: %d\n\n", p.result.Count)
	fmt.Fprintf(w, "%-8s %s\n", "TYPE", "VALUE")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	for _, item := range p.result.Records {
		fmt.Fprintf(w, "%-8s %s\n", item.Type, item.Value)
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *resolvePresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("DNS 解析结果为空")
	}
	fmt.Fprintln(w, "# DNS 解析")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- domain: `%s`\n", mdCell(p.result.Domain))
	fmt.Fprintf(w, "- query_type: `%s`\n", p.result.QueryType)
	if strings.TrimSpace(p.result.DNSServer) != "" {
		fmt.Fprintf(w, "- dns_server: `%s`\n", mdCell(p.result.DNSServer))
	}
	fmt.Fprintf(w, "- duration_ms: `%.2f`\n", p.result.DurationMs)
	fmt.Fprintf(w, "- count: `%d`\n", p.result.Count)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| TYPE | VALUE |")
	fmt.Fprintln(w, "|---|---|")
	for _, item := range p.result.Records {
		fmt.Fprintf(w, "| %s | %s |\n", mdCell(item.Type), mdCell(item.Value))
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *resolvePresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("DNS 解析结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"domain", "query_type", "dns_server", "duration_ms", "type", "value"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.result.Records {
		if err := writer.Write([]string{
			p.result.Domain,
			p.result.QueryType,
			p.result.DNSServer,
			fmt.Sprintf("%.2f", p.result.DurationMs),
			item.Type,
			item.Value,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *benchPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("DNS bench 结果为空")
	}
	fmt.Fprintf(w, "域名: %s\n", p.result.Domain)
	fmt.Fprintf(w, "类型: %s\n", p.result.QueryType)
	if strings.TrimSpace(p.result.DNSServer) != "" {
		fmt.Fprintf(w, "DNS: %s\n", p.result.DNSServer)
	}
	fmt.Fprintf(w, "次数: %d, 成功: %d, 失败: %d\n", p.result.Count, p.result.SuccessCount, p.result.FailureCount)
	if p.result.SuccessCount > 0 {
		fmt.Fprintf(w, "时延统计(ms): min=%.2f avg=%.2f max=%.2f\n", p.result.MinMs, p.result.AvgMs, p.result.MaxMs)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-6s %-8s %-12s %-8s %s\n", "SEQ", "SUCCESS", "DURATION_MS", "COUNT", "ERROR")
	fmt.Fprintln(w, strings.Repeat("-", 90))
	for _, item := range p.result.Attempts {
		fmt.Fprintf(w, "%-6d %-8t %-12.2f %-8d %s\n", item.Seq, item.Success, item.DurationMs, item.RecordCnt, displayValue(item.Error, "-"))
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *benchPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("DNS bench 结果为空")
	}
	fmt.Fprintln(w, "# DNS Bench")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- domain: `%s`\n", mdCell(p.result.Domain))
	fmt.Fprintf(w, "- query_type: `%s`\n", p.result.QueryType)
	if strings.TrimSpace(p.result.DNSServer) != "" {
		fmt.Fprintf(w, "- dns_server: `%s`\n", mdCell(p.result.DNSServer))
	}
	fmt.Fprintf(w, "- count: `%d`\n", p.result.Count)
	fmt.Fprintf(w, "- success_count: `%d`\n", p.result.SuccessCount)
	fmt.Fprintf(w, "- failure_count: `%d`\n", p.result.FailureCount)
	if p.result.SuccessCount > 0 {
		fmt.Fprintf(w, "- min_duration_ms: `%.2f`\n", p.result.MinMs)
		fmt.Fprintf(w, "- avg_duration_ms: `%.2f`\n", p.result.AvgMs)
		fmt.Fprintf(w, "- max_duration_ms: `%.2f`\n", p.result.MaxMs)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| SEQ | SUCCESS | DURATION_MS | RECORD_COUNT | ERROR |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	for _, item := range p.result.Attempts {
		fmt.Fprintf(w, "| %d | %t | %.2f | %d | %s |\n", item.Seq, item.Success, item.DurationMs, item.RecordCnt, mdCell(displayValue(item.Error, "-")))
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *benchPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("DNS bench 结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"domain", "query_type", "dns_server", "seq", "success", "duration_ms", "record_count", "error"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.result.Attempts {
		if err := writer.Write([]string{
			p.result.Domain,
			p.result.QueryType,
			p.result.DNSServer,
			strconv.Itoa(item.Seq),
			strconv.FormatBool(item.Success),
			fmt.Sprintf("%.2f", item.DurationMs),
			strconv.Itoa(item.RecordCnt),
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

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

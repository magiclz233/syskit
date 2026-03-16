package mem

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	memcollector "syskit/internal/collectors/mem"
	"syskit/internal/errs"
	"syskit/pkg/utils"
)

type overviewPresenter struct {
	overview *memcollector.Overview
	detail   bool
}

type topPresenter struct {
	result *memcollector.TopResult
}

func newOverviewPresenter(overview *memcollector.Overview, detail bool) *overviewPresenter {
	return &overviewPresenter{
		overview: overview,
		detail:   detail,
	}
}

func newTopPresenter(result *memcollector.TopResult) *topPresenter {
	return &topPresenter{
		result: result,
	}
}

func (p *overviewPresenter) RenderTable(w io.Writer) error {
	if p.overview == nil {
		return emptyResultError("内存总览结果为空")
	}

	fmt.Fprintln(w, "内存总览")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "总内存: %s\n", formatBytes(p.overview.TotalBytes))
	fmt.Fprintf(w, "可用内存: %s\n", formatBytes(p.overview.AvailableBytes))
	fmt.Fprintf(w, "已用内存: %s\n", formatBytes(p.overview.UsedBytes))
	fmt.Fprintf(w, "空闲内存: %s\n", formatBytes(p.overview.FreeBytes))
	fmt.Fprintf(w, "内存使用率: %.2f%%\n", p.overview.UsagePercent)
	fmt.Fprintf(w, "Swap 总量: %s\n", formatBytes(p.overview.SwapTotalBytes))
	fmt.Fprintf(w, "Swap 已用: %s\n", formatBytes(p.overview.SwapUsedBytes))
	fmt.Fprintf(w, "Swap 可用: %s\n", formatBytes(p.overview.SwapFreeBytes))
	fmt.Fprintf(w, "Swap 使用率: %.2f%%\n", p.overview.SwapUsagePercent)
	if p.detail {
		fmt.Fprintf(w, "缓存(Cached): %s\n", formatBytes(p.overview.CachedBytes))
		fmt.Fprintf(w, "缓冲(Buffers): %s\n", formatBytes(p.overview.BuffersBytes))
	}

	fmt.Fprintln(w, "\n高内存进程（Top 5 by rss）")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	renderProcessTable(w, p.overview.TopProcesses)
	renderWarningsTable(w, p.overview.Warnings)
	return nil
}

func (p *overviewPresenter) RenderMarkdown(w io.Writer) error {
	if p.overview == nil {
		return emptyResultError("内存总览结果为空")
	}

	fmt.Fprintln(w, "# 内存总览")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| 指标 | 值 |")
	fmt.Fprintln(w, "|---|---|")
	fmt.Fprintf(w, "| total_bytes | %d |\n", p.overview.TotalBytes)
	fmt.Fprintf(w, "| available_bytes | %d |\n", p.overview.AvailableBytes)
	fmt.Fprintf(w, "| used_bytes | %d |\n", p.overview.UsedBytes)
	fmt.Fprintf(w, "| free_bytes | %d |\n", p.overview.FreeBytes)
	fmt.Fprintf(w, "| usage_percent | %.2f |\n", p.overview.UsagePercent)
	fmt.Fprintf(w, "| swap_total_bytes | %d |\n", p.overview.SwapTotalBytes)
	fmt.Fprintf(w, "| swap_used_bytes | %d |\n", p.overview.SwapUsedBytes)
	fmt.Fprintf(w, "| swap_free_bytes | %d |\n", p.overview.SwapFreeBytes)
	fmt.Fprintf(w, "| swap_usage_percent | %.2f |\n", p.overview.SwapUsagePercent)
	if p.detail {
		fmt.Fprintf(w, "| cached_bytes | %d |\n", p.overview.CachedBytes)
		fmt.Fprintf(w, "| buffers_bytes | %d |\n", p.overview.BuffersBytes)
	}

	fmt.Fprintln(w, "\n## 高内存进程")
	fmt.Fprintln(w)
	renderProcessMarkdown(w, p.overview.TopProcesses)
	renderWarningsMarkdown(w, p.overview.Warnings)
	return nil
}

func (p *overviewPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.overview == nil {
		return emptyResultError("内存总览结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"row_type",
		"total_bytes",
		"available_bytes",
		"used_bytes",
		"free_bytes",
		"usage_percent",
		"swap_total_bytes",
		"swap_used_bytes",
		"swap_free_bytes",
		"swap_usage_percent",
		"cached_bytes",
		"buffers_bytes",
		"pid",
		"name",
		"user",
		"rss_bytes",
		"vms_bytes",
		"swap_bytes",
		"mem_percent",
		"command",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	if err := writer.Write([]string{
		"summary",
		strconv.FormatUint(p.overview.TotalBytes, 10),
		strconv.FormatUint(p.overview.AvailableBytes, 10),
		strconv.FormatUint(p.overview.UsedBytes, 10),
		strconv.FormatUint(p.overview.FreeBytes, 10),
		fmt.Sprintf("%.2f", p.overview.UsagePercent),
		strconv.FormatUint(p.overview.SwapTotalBytes, 10),
		strconv.FormatUint(p.overview.SwapUsedBytes, 10),
		strconv.FormatUint(p.overview.SwapFreeBytes, 10),
		fmt.Sprintf("%.2f", p.overview.SwapUsagePercent),
		strconv.FormatUint(p.overview.CachedBytes, 10),
		strconv.FormatUint(p.overview.BuffersBytes, 10),
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 内容失败", err)
	}

	for _, item := range p.overview.TopProcesses {
		if err := writer.Write([]string{
			"process",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			strconv.FormatInt(int64(item.PID), 10),
			item.Name,
			item.User,
			strconv.FormatUint(item.RSSBytes, 10),
			strconv.FormatUint(item.VMSBytes, 10),
			strconv.FormatUint(item.SwapBytes, 10),
			fmt.Sprintf("%.2f", item.MemPercent),
			item.Command,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}

	return nil
}

func (p *topPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("内存排行结果为空")
	}

	fmt.Fprintf(w, "内存排行（by=%s, top=%d）\n", p.result.By, p.result.TopN)
	fmt.Fprintf(w, "命中进程数: %d\n", p.result.TotalMatched)
	fmt.Fprintln(w, strings.Repeat("-", 80))
	renderProcessTable(w, p.result.Processes)
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *topPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("内存排行结果为空")
	}

	fmt.Fprintln(w, "# 内存排行")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- by: `%s`\n", p.result.By)
	fmt.Fprintf(w, "- top_n: `%d`\n", p.result.TopN)
	fmt.Fprintf(w, "- total_matched: `%d`\n", p.result.TotalMatched)
	fmt.Fprintln(w)
	renderProcessMarkdown(w, p.result.Processes)
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *topPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("内存排行结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"pid",
		"name",
		"user",
		"rss_bytes",
		"vms_bytes",
		"swap_bytes",
		"mem_percent",
		"command",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	for _, item := range p.result.Processes {
		if err := writer.Write([]string{
			strconv.FormatInt(int64(item.PID), 10),
			item.Name,
			item.User,
			strconv.FormatUint(item.RSSBytes, 10),
			strconv.FormatUint(item.VMSBytes, 10),
			strconv.FormatUint(item.SwapBytes, 10),
			fmt.Sprintf("%.2f", item.MemPercent),
			item.Command,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}

	return nil
}

func renderProcessTable(w io.Writer, items []memcollector.ProcessEntry) {
	if len(items) == 0 {
		fmt.Fprintln(w, "(无结果)")
		return
	}

	fmt.Fprintf(w, "%-8s %-20s %-18s %-12s %-12s %-12s %-8s %s\n", "PID", "NAME", "USER", "RSS", "VMS", "SWAP", "MEM%", "COMMAND")
	for _, item := range items {
		fmt.Fprintf(
			w,
			"%-8d %-20s %-18s %-12s %-12s %-12s %-8.2f %s\n",
			item.PID,
			compact(displayValue(item.Name, "-"), 20),
			compact(displayValue(item.User, "-"), 18),
			formatBytes(item.RSSBytes),
			formatBytes(item.VMSBytes),
			formatBytes(item.SwapBytes),
			item.MemPercent,
			compact(displayValue(item.Command, "-"), 80),
		)
	}
}

func renderProcessMarkdown(w io.Writer, items []memcollector.ProcessEntry) {
	fmt.Fprintln(w, "| PID | NAME | USER | RSS | VMS | SWAP | MEM% | COMMAND |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|---|---|")
	for _, item := range items {
		fmt.Fprintf(
			w,
			"| %d | %s | %s | %s | %s | %s | %.2f | %s |\n",
			item.PID,
			mdCell(displayValue(item.Name, "-")),
			mdCell(displayValue(item.User, "-")),
			mdCell(formatBytes(item.RSSBytes)),
			mdCell(formatBytes(item.VMSBytes)),
			mdCell(formatBytes(item.SwapBytes)),
			item.MemPercent,
			mdCell(compact(displayValue(item.Command, "-"), 120)),
		)
	}
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

func formatBytes(value uint64) string {
	if value > math.MaxInt64 {
		return fmt.Sprintf("%d B", value)
	}
	return utils.FormatBytes(int64(value))
}

func displayValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func compact(text string, limit int) string {
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

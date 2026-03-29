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
	"time"
)

type overviewPresenter struct {
	overview *memcollector.Overview
	detail   bool
}

type topPresenter struct {
	result *memcollector.TopResult
}

type leakPresenter struct {
	result *memcollector.LeakResult
}

type watchPresenter struct {
	result *memcollector.WatchResult
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

func newLeakPresenter(result *memcollector.LeakResult) *leakPresenter {
	return &leakPresenter{result: result}
}

func newWatchPresenter(result *memcollector.WatchResult) *watchPresenter {
	return &watchPresenter{result: result}
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

func (p *leakPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("内存泄漏结果为空")
	}

	fmt.Fprintln(w, "内存泄漏趋势监控")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "PID: %d\n", p.result.PID)
	fmt.Fprintf(w, "采样时长: %dms\n", p.result.DurationMs)
	fmt.Fprintf(w, "采样间隔: %dms\n", p.result.IntervalMs)
	fmt.Fprintf(w, "样本数: %d\n", p.result.SampleCount)
	fmt.Fprintf(w, "结束原因: %s\n", p.result.StoppedReason)
	fmt.Fprintf(w, "RSS(start/end/peak): %s / %s / %s\n", formatBytes(p.result.RSSStartBytes), formatBytes(p.result.RSSEndBytes), formatBytes(p.result.RSSPeakBytes))
	fmt.Fprintf(w, "RSS 增长: %s\n", formatSignedBytes(p.result.RSSGrowthBytes))
	fmt.Fprintf(w, "RSS 增长速率: %.3f MB/min\n", p.result.RSSGrowthRateMBMin)
	fmt.Fprintf(w, "泄漏风险: %s\n", p.result.LeakRisk)
	fmt.Fprintf(w, "判断依据: %s\n", p.result.LeakReason)

	fmt.Fprintln(w, "\n采样明细")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	if len(p.result.Samples) == 0 {
		fmt.Fprintln(w, "(无采样)")
	} else {
		fmt.Fprintf(w, "%-30s %-12s %-12s %-12s\n", "TIMESTAMP", "RSS", "VMS", "SWAP")
		for _, item := range p.result.Samples {
			fmt.Fprintf(
				w,
				"%-30s %-12s %-12s %-12s\n",
				item.Timestamp.Format(time.RFC3339Nano),
				formatBytes(item.RSSBytes),
				formatBytes(item.VMSBytes),
				formatBytes(item.SwapBytes),
			)
		}
	}

	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *leakPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("内存泄漏结果为空")
	}

	fmt.Fprintln(w, "# 内存泄漏趋势监控")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| 字段 | 值 |")
	fmt.Fprintln(w, "|---|---|")
	fmt.Fprintf(w, "| pid | %d |\n", p.result.PID)
	fmt.Fprintf(w, "| duration_ms | %d |\n", p.result.DurationMs)
	fmt.Fprintf(w, "| interval_ms | %d |\n", p.result.IntervalMs)
	fmt.Fprintf(w, "| sample_count | %d |\n", p.result.SampleCount)
	fmt.Fprintf(w, "| stopped_reason | %s |\n", mdCell(p.result.StoppedReason))
	fmt.Fprintf(w, "| rss_start_bytes | %d |\n", p.result.RSSStartBytes)
	fmt.Fprintf(w, "| rss_end_bytes | %d |\n", p.result.RSSEndBytes)
	fmt.Fprintf(w, "| rss_peak_bytes | %d |\n", p.result.RSSPeakBytes)
	fmt.Fprintf(w, "| rss_growth_bytes | %d |\n", p.result.RSSGrowthBytes)
	fmt.Fprintf(w, "| rss_growth_rate_mb_min | %.3f |\n", p.result.RSSGrowthRateMBMin)
	fmt.Fprintf(w, "| leak_risk | %s |\n", mdCell(p.result.LeakRisk))
	fmt.Fprintf(w, "| leak_reason | %s |\n", mdCell(p.result.LeakReason))

	fmt.Fprintln(w, "\n## 采样明细")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| TIMESTAMP | RSS_BYTES | VMS_BYTES | SWAP_BYTES |")
	fmt.Fprintln(w, "|---|---|---|---|")
	for _, item := range p.result.Samples {
		fmt.Fprintf(
			w,
			"| %s | %d | %d | %d |\n",
			item.Timestamp.Format(time.RFC3339Nano),
			item.RSSBytes,
			item.VMSBytes,
			item.SwapBytes,
		)
	}

	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *leakPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("内存泄漏结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"row_type",
		"pid",
		"duration_ms",
		"interval_ms",
		"sample_count",
		"stopped_reason",
		"rss_start_bytes",
		"rss_end_bytes",
		"rss_peak_bytes",
		"rss_growth_bytes",
		"rss_growth_rate_mb_min",
		"leak_risk",
		"leak_reason",
		"timestamp",
		"rss_bytes",
		"vms_bytes",
		"swap_bytes",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	if err := writer.Write([]string{
		"summary",
		strconv.FormatInt(int64(p.result.PID), 10),
		strconv.FormatInt(p.result.DurationMs, 10),
		strconv.FormatInt(p.result.IntervalMs, 10),
		strconv.Itoa(p.result.SampleCount),
		p.result.StoppedReason,
		strconv.FormatUint(p.result.RSSStartBytes, 10),
		strconv.FormatUint(p.result.RSSEndBytes, 10),
		strconv.FormatUint(p.result.RSSPeakBytes, 10),
		strconv.FormatInt(p.result.RSSGrowthBytes, 10),
		fmt.Sprintf("%.3f", p.result.RSSGrowthRateMBMin),
		p.result.LeakRisk,
		p.result.LeakReason,
		"",
		"",
		"",
		"",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 内容失败", err)
	}

	for _, item := range p.result.Samples {
		if err := writer.Write([]string{
			"sample",
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
			"",
			item.Timestamp.Format(time.RFC3339Nano),
			strconv.FormatUint(item.RSSBytes, 10),
			strconv.FormatUint(item.VMSBytes, 10),
			strconv.FormatUint(item.SwapBytes, 10),
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *watchPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("内存持续监控结果为空")
	}

	fmt.Fprintln(w, "内存持续监控汇总")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	fmt.Fprintf(w, "采样间隔: %dms\n", p.result.IntervalMs)
	fmt.Fprintf(w, "top_n: %d\n", p.result.TopN)
	fmt.Fprintf(w, "阈值(mem/swap): %.2f%% / %.2f%%\n", p.result.ThresholdMem, p.result.ThresholdSwap)
	fmt.Fprintf(w, "采样数: %d\n", p.result.SampleCount)
	fmt.Fprintf(w, "结束原因: %s\n", p.result.StoppedReason)
	fmt.Fprintf(w, "内存使用率(peak/avg/last): %.2f%% / %.2f%% / %.2f%%\n", p.result.PeakMem, p.result.AvgMem, p.result.LastMem)
	fmt.Fprintf(w, "Swap 使用率(peak/last): %.2f%% / %.2f%%\n", p.result.PeakSwap, p.result.LastSwap)
	fmt.Fprintf(w, "开始时间: %s\n", p.result.StartedAt.Format(time.RFC3339Nano))
	fmt.Fprintf(w, "结束时间: %s\n", p.result.EndedAt.Format(time.RFC3339Nano))

	fmt.Fprintln(w, "\n最后一次采样的高内存进程")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	renderProcessTable(w, p.result.LastTop)

	fmt.Fprintln(w, "\n告警汇总")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	if len(p.result.Alerts) == 0 {
		fmt.Fprintln(w, "(无告警)")
	} else {
		fmt.Fprintf(w, "%-14s %-8s %-8s %-8s %-20s %s\n", "TYPE", "THR", "PEAK", "COUNT", "FIRST_SEEN", "SUMMARY")
		for _, alert := range p.result.Alerts {
			fmt.Fprintf(
				w,
				"%-14s %-8.2f %-8.2f %-8d %-20s %s\n",
				alert.Type,
				alert.Threshold,
				alert.PeakValue,
				alert.Occurrences,
				compact(alert.FirstSeenAt.Format(time.RFC3339Nano), 20),
				compact(alert.Summary, 120),
			)
		}
	}

	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *watchPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("内存持续监控结果为空")
	}

	fmt.Fprintln(w, "# 内存持续监控汇总")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- interval_ms: `%d`\n", p.result.IntervalMs)
	fmt.Fprintf(w, "- top_n: `%d`\n", p.result.TopN)
	fmt.Fprintf(w, "- threshold_mem: `%.2f`\n", p.result.ThresholdMem)
	fmt.Fprintf(w, "- threshold_swap: `%.2f`\n", p.result.ThresholdSwap)
	fmt.Fprintf(w, "- sample_count: `%d`\n", p.result.SampleCount)
	fmt.Fprintf(w, "- stopped_reason: `%s`\n", p.result.StoppedReason)
	fmt.Fprintf(w, "- mem_peak/avg/last: `%.2f / %.2f / %.2f`\n", p.result.PeakMem, p.result.AvgMem, p.result.LastMem)
	fmt.Fprintf(w, "- swap_peak/last: `%.2f / %.2f`\n", p.result.PeakSwap, p.result.LastSwap)

	fmt.Fprintln(w, "\n## 最后一次采样的高内存进程")
	fmt.Fprintln(w)
	renderProcessMarkdown(w, p.result.LastTop)

	fmt.Fprintln(w, "\n## 告警汇总")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| TYPE | THRESHOLD | PEAK | COUNT | FIRST_SEEN | LAST_SEEN | SUMMARY |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|---|")
	for _, alert := range p.result.Alerts {
		fmt.Fprintf(
			w,
			"| %s | %.2f | %.2f | %d | %s | %s | %s |\n",
			mdCell(alert.Type),
			alert.Threshold,
			alert.PeakValue,
			alert.Occurrences,
			mdCell(alert.FirstSeenAt.Format(time.RFC3339Nano)),
			mdCell(alert.LastSeenAt.Format(time.RFC3339Nano)),
			mdCell(alert.Summary),
		)
	}

	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *watchPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("内存持续监控结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"row_type",
		"interval_ms",
		"top_n",
		"threshold_mem",
		"threshold_swap",
		"sample_count",
		"stopped_reason",
		"peak_mem",
		"avg_mem",
		"last_mem",
		"peak_swap",
		"last_swap",
		"alert_type",
		"alert_threshold",
		"alert_peak",
		"alert_count",
		"alert_summary",
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
		strconv.FormatInt(p.result.IntervalMs, 10),
		strconv.Itoa(p.result.TopN),
		fmt.Sprintf("%.2f", p.result.ThresholdMem),
		fmt.Sprintf("%.2f", p.result.ThresholdSwap),
		strconv.Itoa(p.result.SampleCount),
		p.result.StoppedReason,
		fmt.Sprintf("%.2f", p.result.PeakMem),
		fmt.Sprintf("%.2f", p.result.AvgMem),
		fmt.Sprintf("%.2f", p.result.LastMem),
		fmt.Sprintf("%.2f", p.result.PeakSwap),
		fmt.Sprintf("%.2f", p.result.LastSwap),
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
		"",
		"",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 内容失败", err)
	}

	for _, alert := range p.result.Alerts {
		if err := writer.Write([]string{
			"alert",
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
			alert.Type,
			fmt.Sprintf("%.2f", alert.Threshold),
			fmt.Sprintf("%.2f", alert.PeakValue),
			strconv.Itoa(alert.Occurrences),
			alert.Summary,
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
	}

	for _, item := range p.result.LastTop {
		if err := writer.Write([]string{
			"last_top",
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

func formatSignedBytes(value int64) string {
	if value == 0 {
		return "0 B"
	}
	sign := "+"
	abs := value
	if value < 0 {
		sign = "-"
		abs = -value
	}
	return sign + utils.FormatBytes(abs)
}

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

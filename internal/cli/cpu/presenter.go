package cpu

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	cpucollector "syskit/internal/collectors/cpu"
	"syskit/internal/errs"
	"time"
)

type presenter struct {
	overview *cpucollector.Overview
	detail   bool
}

type burstPresenter struct {
	result *cpucollector.BurstResult
}

type watchPresenter struct {
	result *cpucollector.WatchResult
}

func newPresenter(overview *cpucollector.Overview, detail bool) *presenter {
	return &presenter{
		overview: overview,
		detail:   detail,
	}
}

func newBurstPresenter(result *cpucollector.BurstResult) *burstPresenter {
	return &burstPresenter{result: result}
}

func newWatchPresenter(result *cpucollector.WatchResult) *watchPresenter {
	return &watchPresenter{result: result}
}

func (p *presenter) RenderTable(w io.Writer) error {
	if p.overview == nil {
		return emptyResultError("CPU 总览结果为空")
	}

	fmt.Fprintln(w, "CPU 总览")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "核心数: %d\n", p.overview.CPUCores)
	fmt.Fprintf(w, "总使用率: %.2f%%\n", p.overview.UsagePercent)
	fmt.Fprintf(w, "负载(load1/load5/load15): %.2f / %.2f / %.2f\n", p.overview.Load1, p.overview.Load5, p.overview.Load15)

	if p.detail && len(p.overview.PerCPU) > 0 {
		fmt.Fprintln(w, "\n每核心使用率")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		for index, usage := range p.overview.PerCPU {
			fmt.Fprintf(w, "CPU%-2d: %.2f%%\n", index, usage)
		}
	}

	fmt.Fprintln(w, "\n高 CPU 进程")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	if len(p.overview.TopProcesses) == 0 {
		fmt.Fprintln(w, "(无结果)")
	} else {
		fmt.Fprintf(w, "%-8s %-20s %-20s %-8s %s\n", "PID", "NAME", "USER", "CPU%", "COMMAND")
		for _, item := range p.overview.TopProcesses {
			fmt.Fprintf(
				w,
				"%-8d %-20s %-20s %-8.2f %s\n",
				item.PID,
				compact(displayValue(item.Name, "-"), 20),
				compact(displayValue(item.User, "-"), 20),
				item.CPUPercent,
				compact(displayValue(item.Command, "-"), 80),
			)
		}
	}

	renderWarningsTable(w, p.overview.Warnings)
	return nil
}

func (p *presenter) RenderMarkdown(w io.Writer) error {
	if p.overview == nil {
		return emptyResultError("CPU 总览结果为空")
	}

	fmt.Fprintln(w, "# CPU 总览")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- cpu_cores: `%d`\n", p.overview.CPUCores)
	fmt.Fprintf(w, "- usage_percent: `%.2f`\n", p.overview.UsagePercent)
	fmt.Fprintf(w, "- load1/load5/load15: `%.2f / %.2f / %.2f`\n", p.overview.Load1, p.overview.Load5, p.overview.Load15)

	if p.detail && len(p.overview.PerCPU) > 0 {
		fmt.Fprintln(w, "\n## 每核心使用率")
		fmt.Fprintln(w)
		for index, usage := range p.overview.PerCPU {
			fmt.Fprintf(w, "- cpu%d: %.2f%%\n", index, usage)
		}
	}

	fmt.Fprintln(w, "\n## 高 CPU 进程")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| PID | NAME | USER | CPU% | COMMAND |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	for _, item := range p.overview.TopProcesses {
		fmt.Fprintf(
			w,
			"| %d | %s | %s | %.2f | %s |\n",
			item.PID,
			mdCell(displayValue(item.Name, "-")),
			mdCell(displayValue(item.User, "-")),
			item.CPUPercent,
			mdCell(compact(displayValue(item.Command, "-"), 120)),
		)
	}

	renderWarningsMarkdown(w, p.overview.Warnings)
	return nil
}

func (p *presenter) RenderCSV(w io.Writer, prefix string) error {
	if p.overview == nil {
		return emptyResultError("CPU 总览结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"row_type",
		"core_index",
		"cpu_cores",
		"usage_percent",
		"load1",
		"load5",
		"load15",
		"pid",
		"name",
		"user",
		"cpu_percent",
		"command",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	if err := writer.Write([]string{
		"summary",
		"",
		strconv.Itoa(p.overview.CPUCores),
		fmt.Sprintf("%.2f", p.overview.UsagePercent),
		fmt.Sprintf("%.2f", p.overview.Load1),
		fmt.Sprintf("%.2f", p.overview.Load5),
		fmt.Sprintf("%.2f", p.overview.Load15),
		"",
		"",
		"",
		"",
		"",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 内容失败", err)
	}

	for index, usage := range p.overview.PerCPU {
		if err := writer.Write([]string{
			"core",
			strconv.Itoa(index),
			"",
			fmt.Sprintf("%.2f", usage),
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

	for _, item := range p.overview.TopProcesses {
		if err := writer.Write([]string{
			"process",
			"",
			"",
			"",
			"",
			"",
			"",
			strconv.FormatInt(int64(item.PID), 10),
			item.Name,
			item.User,
			fmt.Sprintf("%.2f", item.CPUPercent),
			item.Command,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}

	return nil
}

func (p *burstPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("CPU 突发采样结果为空")
	}

	durationText := "continuous"
	if !p.result.Continuous {
		durationText = fmt.Sprintf("%dms", p.result.DurationMs)
	}

	fmt.Fprintln(w, "CPU 突发采样")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	fmt.Fprintf(w, "采样间隔: %dms\n", p.result.IntervalMs)
	fmt.Fprintf(w, "采样时长: %s\n", durationText)
	fmt.Fprintf(w, "命中阈值: %.2f%%\n", p.result.ThresholdPercent)
	fmt.Fprintf(w, "CPU 核心数: %d\n", p.result.CPUCores)
	fmt.Fprintf(w, "有效样本数: %d\n", p.result.SampleCount)
	fmt.Fprintf(w, "命中进程数: %d\n", len(p.result.Processes))
	fmt.Fprintf(w, "开始时间: %s\n", formatBurstTime(p.result.StartedAt))
	fmt.Fprintf(w, "结束时间: %s\n", formatBurstTime(p.result.EndedAt))

	fmt.Fprintln(w, "\n命中进程")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	if len(p.result.Processes) == 0 {
		fmt.Fprintln(w, "(无结果)")
	} else {
		fmt.Fprintf(w, "%-8s %-20s %-16s %-8s %-8s %-8s %-8s %-12s %s\n", "PID", "NAME", "USER", "PEAK%", "AVG%", "HITS", "DUR(s)", "PEAK_AT", "COMMAND")
		for _, item := range p.result.Processes {
			fmt.Fprintf(
				w,
				"%-8d %-20s %-16s %-8.2f %-8.2f %-8d %-8.3f %-12s %s\n",
				item.PID,
				compact(displayValue(item.Name, "-"), 20),
				compact(displayValue(item.User, "-"), 16),
				item.PeakCPUPercent,
				item.AvgCPUPercent,
				item.HitCount,
				item.DurationSec,
				formatBurstClock(item.PeakAt),
				compact(displayValue(item.Command, "-"), 80),
			)
		}
	}

	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *burstPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("CPU 突发采样结果为空")
	}

	fmt.Fprintln(w, "# CPU 突发采样")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- interval_ms: `%d`\n", p.result.IntervalMs)
	fmt.Fprintf(w, "- duration_ms: `%d`\n", p.result.DurationMs)
	fmt.Fprintf(w, "- continuous: `%t`\n", p.result.Continuous)
	fmt.Fprintf(w, "- threshold_percent: `%.2f`\n", p.result.ThresholdPercent)
	fmt.Fprintf(w, "- cpu_cores: `%d`\n", p.result.CPUCores)
	fmt.Fprintf(w, "- sample_count: `%d`\n", p.result.SampleCount)
	fmt.Fprintf(w, "- process_count: `%d`\n", len(p.result.Processes))

	fmt.Fprintln(w, "\n## 命中进程")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| PID | NAME | USER | PEAK% | AVG% | HITS | DURATION_SEC | FIRST_SEEN | LAST_SEEN | PEAK_AT | COMMAND |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|---|---|---|---|---|")
	for _, item := range p.result.Processes {
		fmt.Fprintf(
			w,
			"| %d | %s | %s | %.2f | %.2f | %d | %.3f | %s | %s | %s | %s |\n",
			item.PID,
			mdCell(displayValue(item.Name, "-")),
			mdCell(displayValue(item.User, "-")),
			item.PeakCPUPercent,
			item.AvgCPUPercent,
			item.HitCount,
			item.DurationSec,
			mdCell(formatBurstTime(item.FirstSeenAt)),
			mdCell(formatBurstTime(item.LastSeenAt)),
			mdCell(formatBurstTime(item.PeakAt)),
			mdCell(compact(displayValue(item.Command, "-"), 120)),
		)
	}

	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *burstPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("CPU 突发采样结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"interval_ms",
		"duration_ms",
		"continuous",
		"threshold_percent",
		"cpu_cores",
		"sample_count",
		"pid",
		"name",
		"user",
		"peak_cpu_percent",
		"avg_cpu_percent",
		"hit_count",
		"duration_sec",
		"first_seen_at",
		"last_seen_at",
		"peak_at",
		"command",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	if len(p.result.Processes) == 0 {
		if err := writer.Write([]string{
			strconv.FormatInt(p.result.IntervalMs, 10),
			strconv.FormatInt(p.result.DurationMs, 10),
			strconv.FormatBool(p.result.Continuous),
			fmt.Sprintf("%.2f", p.result.ThresholdPercent),
			strconv.Itoa(p.result.CPUCores),
			strconv.Itoa(p.result.SampleCount),
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
		return nil
	}

	for _, item := range p.result.Processes {
		if err := writer.Write([]string{
			strconv.FormatInt(p.result.IntervalMs, 10),
			strconv.FormatInt(p.result.DurationMs, 10),
			strconv.FormatBool(p.result.Continuous),
			fmt.Sprintf("%.2f", p.result.ThresholdPercent),
			strconv.Itoa(p.result.CPUCores),
			strconv.Itoa(p.result.SampleCount),
			strconv.FormatInt(int64(item.PID), 10),
			item.Name,
			item.User,
			fmt.Sprintf("%.2f", item.PeakCPUPercent),
			fmt.Sprintf("%.2f", item.AvgCPUPercent),
			strconv.Itoa(item.HitCount),
			fmt.Sprintf("%.3f", item.DurationSec),
			formatBurstTime(item.FirstSeenAt),
			formatBurstTime(item.LastSeenAt),
			formatBurstTime(item.PeakAt),
			item.Command,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *watchPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("CPU 持续监控结果为空")
	}

	fmt.Fprintln(w, "CPU 持续监控汇总")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	fmt.Fprintf(w, "采样间隔: %dms\n", p.result.IntervalMs)
	fmt.Fprintf(w, "top_n: %d\n", p.result.TopN)
	fmt.Fprintf(w, "阈值(process_cpu/load1): %.2f%% / %.2f\n", p.result.ThresholdCPU, p.result.ThresholdLoad)
	fmt.Fprintf(w, "采样数: %d\n", p.result.SampleCount)
	fmt.Fprintf(w, "结束原因: %s\n", p.result.StoppedReason)
	fmt.Fprintf(w, "CPU(peak/avg/last): %.2f%% / %.2f%% / %.2f%%\n", p.result.PeakCPU, p.result.AvgCPU, p.result.LastCPU)
	fmt.Fprintf(w, "load1(peak/last): %.2f / %.2f\n", p.result.PeakLoad1, p.result.LastLoad1)
	fmt.Fprintf(w, "开始时间: %s\n", formatBurstTime(p.result.StartedAt))
	fmt.Fprintf(w, "结束时间: %s\n", formatBurstTime(p.result.EndedAt))

	fmt.Fprintln(w, "\n最后一次采样的高 CPU 进程")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	if len(p.result.LastTop) == 0 {
		fmt.Fprintln(w, "(无结果)")
	} else {
		fmt.Fprintf(w, "%-8s %-20s %-16s %-8s %s\n", "PID", "NAME", "USER", "CPU%", "COMMAND")
		for _, item := range p.result.LastTop {
			fmt.Fprintf(
				w,
				"%-8d %-20s %-16s %-8.2f %s\n",
				item.PID,
				compact(displayValue(item.Name, "-"), 20),
				compact(displayValue(item.User, "-"), 16),
				item.CPUPercent,
				compact(displayValue(item.Command, "-"), 80),
			)
		}
	}

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
				compact(formatBurstTime(alert.FirstSeenAt), 20),
				compact(alert.Summary, 120),
			)
		}
	}

	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *watchPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("CPU 持续监控结果为空")
	}

	fmt.Fprintln(w, "# CPU 持续监控汇总")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- interval_ms: `%d`\n", p.result.IntervalMs)
	fmt.Fprintf(w, "- top_n: `%d`\n", p.result.TopN)
	fmt.Fprintf(w, "- threshold_cpu: `%.2f`\n", p.result.ThresholdCPU)
	fmt.Fprintf(w, "- threshold_load: `%.2f`\n", p.result.ThresholdLoad)
	fmt.Fprintf(w, "- sample_count: `%d`\n", p.result.SampleCount)
	fmt.Fprintf(w, "- stopped_reason: `%s`\n", p.result.StoppedReason)
	fmt.Fprintf(w, "- cpu_peak/avg/last: `%.2f / %.2f / %.2f`\n", p.result.PeakCPU, p.result.AvgCPU, p.result.LastCPU)
	fmt.Fprintf(w, "- load1_peak/last: `%.2f / %.2f`\n", p.result.PeakLoad1, p.result.LastLoad1)

	fmt.Fprintln(w, "\n## 最后一次采样的高 CPU 进程")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| PID | NAME | USER | CPU% | COMMAND |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	for _, item := range p.result.LastTop {
		fmt.Fprintf(
			w,
			"| %d | %s | %s | %.2f | %s |\n",
			item.PID,
			mdCell(displayValue(item.Name, "-")),
			mdCell(displayValue(item.User, "-")),
			item.CPUPercent,
			mdCell(compact(displayValue(item.Command, "-"), 120)),
		)
	}

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
			mdCell(formatBurstTime(alert.FirstSeenAt)),
			mdCell(formatBurstTime(alert.LastSeenAt)),
			mdCell(alert.Summary),
		)
	}

	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *watchPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("CPU 持续监控结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"row_type",
		"interval_ms",
		"top_n",
		"threshold_cpu",
		"threshold_load",
		"sample_count",
		"stopped_reason",
		"peak_cpu",
		"avg_cpu",
		"last_cpu",
		"peak_load1",
		"last_load1",
		"alert_type",
		"alert_threshold",
		"alert_peak",
		"alert_count",
		"alert_summary",
		"pid",
		"name",
		"user",
		"cpu_percent",
		"command",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	if err := writer.Write([]string{
		"summary",
		strconv.FormatInt(p.result.IntervalMs, 10),
		strconv.Itoa(p.result.TopN),
		fmt.Sprintf("%.2f", p.result.ThresholdCPU),
		fmt.Sprintf("%.2f", p.result.ThresholdLoad),
		strconv.Itoa(p.result.SampleCount),
		p.result.StoppedReason,
		fmt.Sprintf("%.2f", p.result.PeakCPU),
		fmt.Sprintf("%.2f", p.result.AvgCPU),
		fmt.Sprintf("%.2f", p.result.LastCPU),
		fmt.Sprintf("%.2f", p.result.PeakLoad1),
		fmt.Sprintf("%.2f", p.result.LastLoad1),
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
			fmt.Sprintf("%.2f", item.CPUPercent),
			item.Command,
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

func formatBurstTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func formatBurstClock(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("15:04:05.000")
}

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

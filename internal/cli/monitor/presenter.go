package monitor

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"syskit/internal/errs"
	"time"
)

type presenter struct {
	data *monitorOutput
}

func newPresenter(data *monitorOutput) *presenter {
	return &presenter{data: data}
}

func (p *presenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "monitor 输出结果为空")
	}

	fmt.Fprintln(w, "Monitor All 汇总")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	fmt.Fprintf(w, "开始时间: %s\n", formatTime(p.data.StartedAt))
	fmt.Fprintf(w, "结束时间: %s\n", formatTime(p.data.EndedAt))
	fmt.Fprintf(w, "停止原因: %s\n", p.data.StoppedReason)
	fmt.Fprintf(w, "样本数: %d/%d\n", p.data.SampleCount, p.data.MaxSamples)
	fmt.Fprintf(w, "监控文件: %s\n", p.data.MonitorFile)
	fmt.Fprintf(w, "采样间隔: %dms\n", p.data.IntervalMs)
	fmt.Fprintf(w, "告警连续阈值: %d\n", p.data.AlertThreshold)
	if p.data.InspectionIntervalMs > 0 {
		fmt.Fprintf(w, "巡检间隔: %dms (mode=%s, fail-on=%s)\n", p.data.InspectionIntervalMs, p.data.InspectionMode, p.data.InspectionFailOn)
	}

	renderPeaksTable(w, p.data.Peaks)
	renderAlertsTable(w, p.data.Alerts)
	renderInspectionsTable(w, p.data.Inspections)
	renderWarningsTable(w, p.data.Warnings)
	return nil
}

func (p *presenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "monitor 输出结果为空")
	}

	fmt.Fprintln(w, "# Monitor All")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- started_at: `%s`\n", formatTime(p.data.StartedAt))
	fmt.Fprintf(w, "- ended_at: `%s`\n", formatTime(p.data.EndedAt))
	fmt.Fprintf(w, "- stopped_reason: `%s`\n", p.data.StoppedReason)
	fmt.Fprintf(w, "- sample_count: `%d`\n", p.data.SampleCount)
	fmt.Fprintf(w, "- max_samples: `%d`\n", p.data.MaxSamples)
	fmt.Fprintf(w, "- interval_ms: `%d`\n", p.data.IntervalMs)
	fmt.Fprintf(w, "- monitor_file: `%s`\n", mdCell(p.data.MonitorFile))
	fmt.Fprintf(w, "- alert_threshold: `%d`\n", p.data.AlertThreshold)
	if p.data.InspectionIntervalMs > 0 {
		fmt.Fprintf(w, "- inspection_interval_ms: `%d`\n", p.data.InspectionIntervalMs)
		fmt.Fprintf(w, "- inspection_mode: `%s`\n", p.data.InspectionMode)
		fmt.Fprintf(w, "- inspection_fail_on: `%s`\n", p.data.InspectionFailOn)
	}

	fmt.Fprintln(w, "\n## Peaks")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- cpu_percent: `%s`\n", formatPercent(p.data.Peaks.CPUPercent))
	fmt.Fprintf(w, "- load1: `%s`\n", formatPercent(p.data.Peaks.Load1))
	fmt.Fprintf(w, "- mem_percent: `%s`\n", formatPercent(p.data.Peaks.MemPercent))
	fmt.Fprintf(w, "- swap_percent: `%s`\n", formatPercent(p.data.Peaks.SwapPercent))
	fmt.Fprintf(w, "- disk_percent: `%s`\n", formatPercent(p.data.Peaks.DiskPercent))
	if p.data.Peaks.DiskPeakMount != "" {
		fmt.Fprintf(w, "- disk_peak_mount: `%s`\n", mdCell(p.data.Peaks.DiskPeakMount))
	}
	fmt.Fprintf(w, "- connection_count: `%d`\n", p.data.Peaks.ConnectionCount)
	fmt.Fprintf(w, "- listen_count: `%d`\n", p.data.Peaks.ListenCount)
	fmt.Fprintf(w, "- process_count: `%d`\n", p.data.Peaks.ProcessCount)

	if len(p.data.Alerts) > 0 {
		fmt.Fprintln(w, "\n## Alerts")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| KEY | TYPE | OCCURRENCES | THRESHOLD | PEAK | FIRST | LAST |")
		fmt.Fprintln(w, "|---|---|---:|---:|---:|---|---|")
		for _, item := range p.data.Alerts {
			fmt.Fprintf(
				w,
				"| %s | %s | %d | %.2f | %.2f | %s | %s |\n",
				mdCell(item.Key),
				mdCell(item.Type),
				item.Occurrences,
				item.Threshold,
				item.PeakValue,
				formatTime(item.FirstSeenAt),
				formatTime(item.LastSeenAt),
			)
		}
	}

	if len(p.data.Inspections) > 0 {
		fmt.Fprintln(w, "\n## Inspections")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| TIME | MODE | FAIL_ON | SCORE | LEVEL | ISSUE | SKIPPED | MATCHED | EXIT | ERROR |")
		fmt.Fprintln(w, "|---|---|---|---:|---|---:|---:|---|---:|---|")
		for _, item := range p.data.Inspections {
			fmt.Fprintf(
				w,
				"| %s | %s | %s | %d | %s | %d | %d | %t | %d | %s |\n",
				formatTime(item.Timestamp),
				mdCell(item.Mode),
				mdCell(item.FailOn),
				item.HealthScore,
				mdCell(item.HealthLevel),
				item.IssueCount,
				item.SkippedCount,
				item.FailOnMatched,
				item.ExitCode,
				mdCell(item.Error),
			)
		}
	}

	if len(p.data.Warnings) > 0 {
		fmt.Fprintln(w, "\n## Warnings")
		fmt.Fprintln(w)
		for _, warning := range p.data.Warnings {
			fmt.Fprintf(w, "- %s\n", warning)
		}
	}
	return nil
}

func (p *presenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "monitor 输出结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"row_type", "key", "value", "extra", "timestamp"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	writeRow := func(rowType string, key string, value string, extra string, timestamp string) error {
		if err := writer.Write([]string{rowType, key, value, extra, timestamp}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
		return nil
	}

	if err := writeRow("summary", "sample_count", strconv.Itoa(p.data.SampleCount), "", ""); err != nil {
		return err
	}
	if err := writeRow("summary", "stopped_reason", p.data.StoppedReason, "", ""); err != nil {
		return err
	}
	if err := writeRow("summary", "monitor_file", p.data.MonitorFile, "", ""); err != nil {
		return err
	}
	if err := writeRow("peak", "cpu_percent", formatPercent(p.data.Peaks.CPUPercent), "", ""); err != nil {
		return err
	}
	if err := writeRow("peak", "mem_percent", formatPercent(p.data.Peaks.MemPercent), "", ""); err != nil {
		return err
	}
	if err := writeRow("peak", "disk_percent", formatPercent(p.data.Peaks.DiskPercent), p.data.Peaks.DiskPeakMount, ""); err != nil {
		return err
	}

	for _, item := range p.data.Alerts {
		if err := writeRow("alert", item.Key, strconv.Itoa(item.Occurrences), item.Summary, formatTime(item.LastSeenAt)); err != nil {
			return err
		}
	}
	for _, item := range p.data.Inspections {
		value := strconv.Itoa(item.HealthScore)
		if item.Error != "" {
			value = item.Error
		}
		extra := fmt.Sprintf("mode=%s fail_on=%s issue=%d skipped=%d matched=%t exit=%d", item.Mode, item.FailOn, item.IssueCount, item.SkippedCount, item.FailOnMatched, item.ExitCode)
		if err := writeRow("inspection", "doctor_all", value, extra, formatTime(item.Timestamp)); err != nil {
			return err
		}
	}
	for _, warning := range p.data.Warnings {
		if err := writeRow("warning", "message", warning, "", ""); err != nil {
			return err
		}
	}
	return nil
}

func renderPeaksTable(w io.Writer, peaks monitorPeaks) {
	fmt.Fprintln(w, "\n峰值统计")
	fmt.Fprintf(w, "CPU 使用率峰值: %.2f%%\n", peaks.CPUPercent)
	fmt.Fprintf(w, "load1 峰值: %.2f\n", peaks.Load1)
	fmt.Fprintf(w, "内存使用率峰值: %.2f%%\n", peaks.MemPercent)
	fmt.Fprintf(w, "Swap 使用率峰值: %.2f%%\n", peaks.SwapPercent)
	fmt.Fprintf(w, "磁盘使用率峰值: %.2f%%\n", peaks.DiskPercent)
	if peaks.DiskPeakMount != "" {
		fmt.Fprintf(w, "磁盘峰值挂载点: %s\n", peaks.DiskPeakMount)
	}
	fmt.Fprintf(w, "连接数峰值: %d\n", peaks.ConnectionCount)
	fmt.Fprintf(w, "监听端口峰值: %d\n", peaks.ListenCount)
	fmt.Fprintf(w, "进程数峰值: %d\n", peaks.ProcessCount)
}

func renderAlertsTable(w io.Writer, alerts []monitorAlert) {
	fmt.Fprintln(w, "\n告警聚合")
	if len(alerts) == 0 {
		fmt.Fprintln(w, "(无告警)")
		return
	}
	fmt.Fprintf(w, "%-18s %-10s %-12s %-12s %-12s %-20s %s\n", "KEY", "TYPE", "HITS", "THRESHOLD", "PEAK", "LAST_SEEN", "SUMMARY")
	for _, item := range alerts {
		fmt.Fprintf(
			w,
			"%-18s %-10s %-12d %-12.2f %-12.2f %-20s %s\n",
			item.Key,
			item.Type,
			item.Occurrences,
			item.Threshold,
			item.PeakValue,
			formatTime(item.LastSeenAt),
			item.Summary,
		)
	}
}

func renderInspectionsTable(w io.Writer, inspections []monitorInspection) {
	if len(inspections) == 0 {
		return
	}
	fmt.Fprintln(w, "\n巡检记录")
	fmt.Fprintf(w, "%-20s %-8s %-8s %-8s %-10s %-8s %-8s %-8s %s\n", "TIME", "MODE", "FAIL_ON", "SCORE", "LEVEL", "ISSUE", "SKIPPED", "EXIT", "ERROR")
	for _, item := range inspections {
		fmt.Fprintf(
			w,
			"%-20s %-8s %-8s %-8d %-10s %-8d %-8d %-8d %s\n",
			formatTime(item.Timestamp),
			item.Mode,
			item.FailOn,
			item.HealthScore,
			item.HealthLevel,
			item.IssueCount,
			item.SkippedCount,
			item.ExitCode,
			item.Error,
		)
	}
}

func renderWarningsTable(w io.Writer, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w, "\n提示")
	for _, warning := range warnings {
		fmt.Fprintf(w, "- %s\n", warning)
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

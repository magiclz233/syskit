package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"syskit/internal/errs"
	"time"
)

type reportPresenter struct {
	data *generateOutputData
}

func newReportPresenter(data *generateOutputData) *reportPresenter {
	return &reportPresenter{data: data}
}

func (p *reportPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "report 输出结果为空")
	}

	fmt.Fprintln(w, "Report 生成结果")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	fmt.Fprintf(w, "类型: %s\n", p.data.Type)
	fmt.Fprintf(w, "生成时间: %s\n", formatTime(p.data.GeneratedAt))
	fmt.Fprintf(w, "时间范围: %s\n", p.data.TimeRange)
	fmt.Fprintf(w, "窗口: [%s, %s]\n", formatTime(p.data.WindowStart), formatTime(p.data.WindowEnd))

	switch p.data.Type {
	case reportTypeHealth:
		renderHealthTable(w, p.data.Health)
	case reportTypeInspection:
		renderInspectionTable(w, p.data.Inspection)
	case reportTypeMonitor:
		renderMonitorTable(w, p.data.Monitor)
	}
	renderWarningsTable(w, p.data.Warnings)
	return nil
}

func (p *reportPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "report 输出结果为空")
	}

	fmt.Fprintln(w, "# Report Generate")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- type: `%s`\n", p.data.Type)
	fmt.Fprintf(w, "- generated_at: `%s`\n", formatTime(p.data.GeneratedAt))
	fmt.Fprintf(w, "- time_range: `%s`\n", p.data.TimeRange)
	fmt.Fprintf(w, "- window_start: `%s`\n", formatTime(p.data.WindowStart))
	fmt.Fprintf(w, "- window_end: `%s`\n", formatTime(p.data.WindowEnd))

	switch p.data.Type {
	case reportTypeHealth:
		renderHealthMarkdown(w, p.data.Health)
	case reportTypeInspection:
		renderInspectionMarkdown(w, p.data.Inspection)
	case reportTypeMonitor:
		renderMonitorMarkdown(w, p.data.Monitor)
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

func (p *reportPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "report 输出结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"row_type", "report_type", "generated_at", "time_range", "window_start", "window_end", "key", "value", "extra"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	common := []string{
		p.data.Type,
		formatTime(p.data.GeneratedAt),
		p.data.TimeRange,
		formatTime(p.data.WindowStart),
		formatTime(p.data.WindowEnd),
	}
	writeRow := func(rowType string, key string, value string, extra string) error {
		row := []string{
			rowType,
			common[0],
			common[1],
			common[2],
			common[3],
			common[4],
			key,
			value,
			extra,
		}
		if err := writer.Write(row); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
		return nil
	}

	if err := writeRow("summary", "type", p.data.Type, ""); err != nil {
		return err
	}

	switch p.data.Type {
	case reportTypeHealth:
		if p.data.Health != nil {
			if err := writeRow("health", "health_score", strconv.Itoa(p.data.Health.HealthScore), ""); err != nil {
				return err
			}
			if err := writeRow("health", "health_level", p.data.Health.HealthLevel, ""); err != nil {
				return err
			}
			if err := writeRow("health", "warning_count", strconv.Itoa(p.data.Health.WarningCount), ""); err != nil {
				return err
			}
			if err := writeRow("health", "module_coverage", fmt.Sprintf("%.1f", p.data.Health.ModuleCoverage), ""); err != nil {
				return err
			}
			if err := writeRow("health", "snapshot_count", strconv.Itoa(p.data.Health.SnapshotCount), ""); err != nil {
				return err
			}
			if p.data.Health.LatestSnapshot != nil {
				if err := writeRow("health", "latest_snapshot_id", p.data.Health.LatestSnapshot.ID, ""); err != nil {
					return err
				}
			}
		}
	case reportTypeInspection:
		if p.data.Inspection != nil {
			if err := writeRow("inspection", "snapshot_count", strconv.Itoa(p.data.Inspection.SnapshotCount), ""); err != nil {
				return err
			}
			if p.data.Inspection.FirstAt != nil {
				if err := writeRow("inspection", "first_at", formatTime(*p.data.Inspection.FirstAt), ""); err != nil {
					return err
				}
			}
			if p.data.Inspection.LastAt != nil {
				if err := writeRow("inspection", "last_at", formatTime(*p.data.Inspection.LastAt), ""); err != nil {
					return err
				}
			}
			for _, module := range p.data.Inspection.Modules {
				if err := writeRow("inspection_module", "module_count", strconv.Itoa(module.Count), module.Module); err != nil {
					return err
				}
			}
		}
	case reportTypeMonitor:
		if p.data.Monitor != nil {
			if err := writeRow("monitor", "monitor_file_count", strconv.Itoa(p.data.Monitor.MonitorFileCount), ""); err != nil {
				return err
			}
			if err := writeRow("monitor", "range_file_count", strconv.Itoa(p.data.Monitor.RangeFileCount), ""); err != nil {
				return err
			}
			if err := writeRow("monitor", "snapshot_samples", strconv.Itoa(p.data.Monitor.SnapshotSamples), ""); err != nil {
				return err
			}
			if strings.TrimSpace(p.data.Monitor.Note) != "" {
				if err := writeRow("monitor", "note", p.data.Monitor.Note, ""); err != nil {
					return err
				}
			}
		}
	}

	for _, warning := range p.data.Warnings {
		if err := writeRow("warning", "message", warning, ""); err != nil {
			return err
		}
	}
	return nil
}

func renderHealthTable(w io.Writer, health *healthReportData) {
	fmt.Fprintln(w, "\nHealth 数据")
	if health == nil {
		fmt.Fprintln(w, "(无 health 数据)")
		return
	}
	fmt.Fprintf(w, "健康分: %d\n", health.HealthScore)
	fmt.Fprintf(w, "健康等级: %s\n", health.HealthLevel)
	fmt.Fprintf(w, "告警数: %d\n", health.WarningCount)
	fmt.Fprintf(w, "模块覆盖率: %.1f%%\n", health.ModuleCoverage)
	fmt.Fprintf(w, "快照样本数: %d\n", health.SnapshotCount)
	if health.LatestSnapshot != nil {
		fmt.Fprintf(w, "最新快照: %s (%s)\n", health.LatestSnapshot.ID, formatTime(health.LatestSnapshot.CreatedAt))
	}
}

func renderInspectionTable(w io.Writer, inspection *inspectionReportData) {
	fmt.Fprintln(w, "\nInspection 数据")
	if inspection == nil {
		fmt.Fprintln(w, "(无 inspection 数据)")
		return
	}
	fmt.Fprintf(w, "快照样本数: %d\n", inspection.SnapshotCount)
	if inspection.FirstAt != nil {
		fmt.Fprintf(w, "窗口首条: %s\n", formatTime(*inspection.FirstAt))
	}
	if inspection.LastAt != nil {
		fmt.Fprintf(w, "窗口末条: %s\n", formatTime(*inspection.LastAt))
	}
	if len(inspection.Modules) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%-18s %s\n", "MODULE", "COUNT")
	for _, item := range inspection.Modules {
		fmt.Fprintf(w, "%-18s %d\n", item.Module, item.Count)
	}
}

func renderMonitorTable(w io.Writer, monitor *monitorReportData) {
	fmt.Fprintln(w, "\nMonitor 数据")
	if monitor == nil {
		fmt.Fprintln(w, "(无 monitor 数据)")
		return
	}
	fmt.Fprintf(w, "monitor 文件总数: %d\n", monitor.MonitorFileCount)
	fmt.Fprintf(w, "窗口内文件数: %d\n", monitor.RangeFileCount)
	fmt.Fprintf(w, "快照替代样本数: %d\n", monitor.SnapshotSamples)
	if strings.TrimSpace(monitor.Note) != "" {
		fmt.Fprintf(w, "说明: %s\n", monitor.Note)
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

func renderHealthMarkdown(w io.Writer, health *healthReportData) {
	fmt.Fprintln(w, "\n## Health")
	fmt.Fprintln(w)
	if health == nil {
		fmt.Fprintln(w, "(无 health 数据)")
		return
	}
	fmt.Fprintf(w, "- health_score: `%d`\n", health.HealthScore)
	fmt.Fprintf(w, "- health_level: `%s`\n", health.HealthLevel)
	fmt.Fprintf(w, "- warning_count: `%d`\n", health.WarningCount)
	fmt.Fprintf(w, "- module_coverage: `%.1f`\n", health.ModuleCoverage)
	fmt.Fprintf(w, "- snapshot_count: `%d`\n", health.SnapshotCount)
	if health.LatestSnapshot != nil {
		fmt.Fprintf(w, "- latest_snapshot_id: `%s`\n", mdCell(health.LatestSnapshot.ID))
		fmt.Fprintf(w, "- latest_snapshot_created_at: `%s`\n", formatTime(health.LatestSnapshot.CreatedAt))
	}
}

func renderInspectionMarkdown(w io.Writer, inspection *inspectionReportData) {
	fmt.Fprintln(w, "\n## Inspection")
	fmt.Fprintln(w)
	if inspection == nil {
		fmt.Fprintln(w, "(无 inspection 数据)")
		return
	}
	fmt.Fprintf(w, "- snapshot_count: `%d`\n", inspection.SnapshotCount)
	if inspection.FirstAt != nil {
		fmt.Fprintf(w, "- first_at: `%s`\n", formatTime(*inspection.FirstAt))
	}
	if inspection.LastAt != nil {
		fmt.Fprintf(w, "- last_at: `%s`\n", formatTime(*inspection.LastAt))
	}
	if len(inspection.Modules) == 0 {
		return
	}
	fmt.Fprintln(w, "\n| MODULE | COUNT |")
	fmt.Fprintln(w, "|---|---|")
	for _, item := range inspection.Modules {
		fmt.Fprintf(w, "| %s | %d |\n", mdCell(item.Module), item.Count)
	}
}

func renderMonitorMarkdown(w io.Writer, monitor *monitorReportData) {
	fmt.Fprintln(w, "\n## Monitor")
	fmt.Fprintln(w)
	if monitor == nil {
		fmt.Fprintln(w, "(无 monitor 数据)")
		return
	}
	fmt.Fprintf(w, "- monitor_file_count: `%d`\n", monitor.MonitorFileCount)
	fmt.Fprintf(w, "- range_file_count: `%d`\n", monitor.RangeFileCount)
	fmt.Fprintf(w, "- snapshot_samples: `%d`\n", monitor.SnapshotSamples)
	if strings.TrimSpace(monitor.Note) != "" {
		fmt.Fprintf(w, "- note: `%s`\n", mdCell(monitor.Note))
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

package cpu

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	cpucollector "syskit/internal/collectors/cpu"
	"syskit/internal/errs"
)

type presenter struct {
	overview *cpucollector.Overview
	detail   bool
}

func newPresenter(overview *cpucollector.Overview, detail bool) *presenter {
	return &presenter{
		overview: overview,
		detail:   detail,
	}
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

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

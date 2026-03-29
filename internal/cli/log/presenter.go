package logcmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	logcollector "syskit/internal/collectors/log"
	"syskit/internal/errs"
	"time"
)

type overviewPresenter struct {
	data *logcollector.OverviewResult
}

func newOverviewPresenter(data *logcollector.OverviewResult) *overviewPresenter {
	return &overviewPresenter{data: data}
}

func (p *overviewPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "log 输出结果为空")
	}
	fmt.Fprintln(w, "Log 体检结果")
	fmt.Fprintf(w, "files=%d total=%d matched=%d error_rate=%.2f%%\n", p.data.FileCount, p.data.TotalLines, p.data.MatchedLines, p.data.ErrorRate)
	fmt.Fprintf(w, "level=%s since=%ds top=%d\n", p.data.Level, p.data.SinceSec, p.data.Top)

	fmt.Fprintln(w, "\n按级别统计")
	for _, key := range []string{"error", "warn", "info", "debug", "other"} {
		fmt.Fprintf(w, "- %s: %d\n", key, p.data.LevelCounts[key])
	}

	fmt.Fprintln(w, "\n文件统计")
	fmt.Fprintf(w, "%-48s %-10s %-10s %-10s %-10s\n", "PATH", "TOTAL", "MATCHED", "ERROR", "WARN")
	for _, item := range p.data.Files {
		fmt.Fprintf(w, "%-48s %-10d %-10d %-10d %-10d\n", compact(item.Path, 48), item.TotalLines, item.MatchedLines, item.ErrorLines, item.WarnLines)
	}

	if len(p.data.TopMessages) > 0 {
		fmt.Fprintln(w, "\n高频消息")
		for _, item := range p.data.TopMessages {
			fmt.Fprintf(w, "- [%d] %s\n", item.Count, item.Message)
		}
	}
	if len(p.data.Samples) > 0 {
		fmt.Fprintln(w, "\n样本行")
		for _, sample := range p.data.Samples {
			fmt.Fprintf(w, "- %s:%d [%s] %s\n", sample.File, sample.Line, sample.Level, sample.Text)
		}
	}
	renderWarnings(w, p.data.Warnings)
	return nil
}

func (p *overviewPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "log 输出结果为空")
	}
	fmt.Fprintln(w, "# Log Overview")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- file_count: `%d`\n", p.data.FileCount)
	fmt.Fprintf(w, "- total_lines: `%d`\n", p.data.TotalLines)
	fmt.Fprintf(w, "- matched_lines: `%d`\n", p.data.MatchedLines)
	fmt.Fprintf(w, "- error_rate: `%.2f`\n", p.data.ErrorRate)
	fmt.Fprintf(w, "- level: `%s`\n", p.data.Level)
	fmt.Fprintf(w, "- since_sec: `%d`\n", p.data.SinceSec)
	fmt.Fprintf(w, "- top: `%d`\n", p.data.Top)
	fmt.Fprintln(w, "\n## Level Counts")
	fmt.Fprintln(w)
	for _, key := range []string{"error", "warn", "info", "debug", "other"} {
		fmt.Fprintf(w, "- %s: `%d`\n", key, p.data.LevelCounts[key])
	}

	if len(p.data.TopMessages) > 0 {
		fmt.Fprintln(w, "\n## Top Messages")
		fmt.Fprintln(w)
		for _, item := range p.data.TopMessages {
			fmt.Fprintf(w, "- [%d] %s\n", item.Count, item.Message)
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

func (p *overviewPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "log 输出结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"row_type", "key", "value", "extra"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	write := func(rowType string, key string, value string, extra string) error {
		if err := writer.Write([]string{rowType, key, value, extra}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
		return nil
	}
	if err := write("summary", "matched_lines", strconv.Itoa(p.data.MatchedLines), ""); err != nil {
		return err
	}
	if err := write("summary", "error_rate", fmt.Sprintf("%.2f", p.data.ErrorRate), ""); err != nil {
		return err
	}
	for _, file := range p.data.Files {
		if err := write("file", file.Path, strconv.Itoa(file.MatchedLines), fmt.Sprintf("error=%d warn=%d", file.ErrorLines, file.WarnLines)); err != nil {
			return err
		}
	}
	for _, item := range p.data.TopMessages {
		if err := write("top_message", item.Message, strconv.Itoa(item.Count), ""); err != nil {
			return err
		}
	}
	for _, warning := range p.data.Warnings {
		if err := write("warning", "message", warning, ""); err != nil {
			return err
		}
	}
	return nil
}

type searchPresenter struct {
	data *logcollector.SearchResult
}

func newSearchPresenter(data *logcollector.SearchResult) *searchPresenter {
	return &searchPresenter{data: data}
}

func (p *searchPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "log search 输出结果为空")
	}
	fmt.Fprintf(w, "Log Search keyword=%s matches=%d files=%d\n", p.data.Keyword, p.data.TotalMatches, p.data.FileCount)
	for _, item := range p.data.Matches {
		fmt.Fprintf(w, "- %s:%d [%s] %s\n", item.File, item.Line, item.Level, item.Text)
		if len(item.Before) > 0 {
			fmt.Fprintf(w, "  before: %s\n", strings.Join(item.Before, " | "))
		}
		if len(item.After) > 0 {
			fmt.Fprintf(w, "  after : %s\n", strings.Join(item.After, " | "))
		}
	}
	renderWarnings(w, p.data.Warnings)
	return nil
}

func (p *searchPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "log search 输出结果为空")
	}
	fmt.Fprintln(w, "# Log Search")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- keyword: `%s`\n", mdCell(p.data.Keyword))
	fmt.Fprintf(w, "- total_matches: `%d`\n", p.data.TotalMatches)
	fmt.Fprintf(w, "- file_count: `%d`\n", p.data.FileCount)
	if len(p.data.Matches) > 0 {
		fmt.Fprintln(w, "\n| FILE | LINE | LEVEL | TEXT |")
		fmt.Fprintln(w, "|---|---:|---|---|")
		for _, item := range p.data.Matches {
			fmt.Fprintf(w, "| %s | %d | %s | %s |\n", mdCell(item.File), item.Line, mdCell(item.Level), mdCell(item.Text))
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

func (p *searchPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "log search 输出结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"file", "line", "level", "timestamp", "text"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.data.Matches {
		row := []string{
			item.File,
			strconv.Itoa(item.Line),
			item.Level,
			formatTime(item.Timestamp),
			item.Text,
		}
		if err := writer.Write(row); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

type watchPresenter struct {
	data *logcollector.WatchResult
}

func newWatchPresenter(data *logcollector.WatchResult) *watchPresenter {
	return &watchPresenter{data: data}
}

func (p *watchPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "log watch 输出结果为空")
	}
	fmt.Fprintf(w, "Log Watch samples=%d stopped=%s alerts=%d\n", p.data.SampleCount, p.data.StoppedReason, len(p.data.Alerts))
	fmt.Fprintf(w, "growth=%dB lines=%d errors=%d\n", p.data.TotalGrowthBytes, p.data.TotalNewLines, p.data.TotalErrorLines)
	for _, alert := range p.data.Alerts {
		fmt.Fprintf(w, "- [%s] %.2f >= %.2f (%s)\n", alert.Type, alert.Value, alert.Threshold, alert.Summary)
	}
	renderWarnings(w, p.data.Warnings)
	return nil
}

func (p *watchPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "log watch 输出结果为空")
	}
	fmt.Fprintln(w, "# Log Watch")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- sample_count: `%d`\n", p.data.SampleCount)
	fmt.Fprintf(w, "- stopped_reason: `%s`\n", mdCell(p.data.StoppedReason))
	fmt.Fprintf(w, "- total_growth_bytes: `%d`\n", p.data.TotalGrowthBytes)
	fmt.Fprintf(w, "- total_new_lines: `%d`\n", p.data.TotalNewLines)
	fmt.Fprintf(w, "- total_error_lines: `%d`\n", p.data.TotalErrorLines)
	if len(p.data.Alerts) > 0 {
		fmt.Fprintln(w, "\n## Alerts")
		fmt.Fprintln(w)
		for _, alert := range p.data.Alerts {
			fmt.Fprintf(w, "- [%s] value=%.2f threshold=%.2f summary=%s\n", alert.Type, alert.Value, alert.Threshold, alert.Summary)
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

func (p *watchPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "log watch 输出结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"row_type", "timestamp", "type", "value", "threshold", "summary"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, sample := range p.data.Samples {
		row := []string{
			"sample",
			formatTime(sample.Timestamp),
			"error_rate",
			fmt.Sprintf("%.2f", sample.ErrorRate),
			fmt.Sprintf("%.2f", p.data.ThresholdError),
			fmt.Sprintf("growth=%d new=%d error=%d", sample.GrowthBytes, sample.NewLines, sample.ErrorLines),
		}
		if err := writer.Write(row); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	for _, alert := range p.data.Alerts {
		row := []string{
			"alert",
			formatTime(alert.Timestamp),
			alert.Type,
			fmt.Sprintf("%.2f", alert.Value),
			fmt.Sprintf("%.2f", alert.Threshold),
			alert.Summary,
		}
		if err := writer.Write(row); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func renderWarnings(w io.Writer, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w, "\n提示")
	for _, warning := range warnings {
		fmt.Fprintf(w, "- %s\n", warning)
	}
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

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

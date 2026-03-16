package fix

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	cleanupcollector "syskit/internal/collectors/cleanup"
	"syskit/internal/errs"
	"syskit/pkg/utils"
	"time"
)

type cleanupPresenter struct {
	data *cleanupOutputData
}

func newCleanupPresenter(data *cleanupOutputData) *cleanupPresenter {
	return &cleanupPresenter{data: data}
}

func (p *cleanupPresenter) RenderTable(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return emptyResultError("cleanup 结果为空")
	}

	plan := p.data.Plan
	fmt.Fprintf(w, "清理模式: %s\n", p.data.Mode)
	fmt.Fprintf(w, "目标: %s\n", joinTargets(plan.Targets))
	fmt.Fprintf(w, "older-than: %ds\n", plan.OlderThanSec)
	fmt.Fprintf(w, "扫描根目录: %d\n", len(plan.ScanRoots))
	fmt.Fprintf(w, "扫描文件数: %d\n", plan.ScannedFiles)
	fmt.Fprintf(w, "候选文件: %d\n", plan.CandidateCount)
	fmt.Fprintf(w, "候选大小: %s\n", formatBytes(uint64(max64(plan.CandidateBytes, 0))))

	fmt.Fprintln(w, "\n候选清单")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	renderCandidatesTable(w, plan.Candidates)

	if p.data.Result != nil {
		result := p.data.Result
		fmt.Fprintln(w, "\n执行结果")
		fmt.Fprintln(w, strings.Repeat("-", 100))
		fmt.Fprintf(w, "已删除: %d (%s)\n", result.DeletedCount, formatBytes(uint64(max64(result.DeletedBytes, 0))))
		fmt.Fprintf(w, "删除失败: %d\n", len(result.Failed))
		fmt.Fprintf(w, "剩余文件: %d (%s)\n", result.RemainingCount, formatBytes(uint64(max64(result.RemainingBytes, 0))))
		if len(result.Failed) > 0 {
			fmt.Fprintln(w, "\n失败列表")
			fmt.Fprintln(w, strings.Repeat("-", 100))
			for _, item := range result.Failed {
				fmt.Fprintf(w, "- %s (%s): %s\n", item.Path, formatBytes(uint64(max64(item.SizeBytes, 0))), item.Message)
			}
		}
	}

	renderWarningsTable(w, plan.Warnings)
	if p.data.Result != nil {
		renderWarningsTable(w, p.data.Result.Warnings)
	}
	return nil
}

func (p *cleanupPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return emptyResultError("cleanup 结果为空")
	}

	plan := p.data.Plan
	fmt.Fprintln(w, "# Fix Cleanup")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- mode: `%s`\n", p.data.Mode)
	fmt.Fprintf(w, "- targets: `%s`\n", joinTargets(plan.Targets))
	fmt.Fprintf(w, "- older_than_sec: `%d`\n", plan.OlderThanSec)
	fmt.Fprintf(w, "- scan_roots: `%d`\n", len(plan.ScanRoots))
	fmt.Fprintf(w, "- scanned_files: `%d`\n", plan.ScannedFiles)
	fmt.Fprintf(w, "- candidate_count: `%d`\n", plan.CandidateCount)
	fmt.Fprintf(w, "- candidate_bytes: `%d`\n", plan.CandidateBytes)

	fmt.Fprintln(w, "\n## 候选清单")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| TARGET | SIZE_BYTES | AGE_SEC | MOD_TIME | PATH |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	for _, item := range plan.Candidates {
		fmt.Fprintf(
			w,
			"| %s | %d | %d | %s | %s |\n",
			item.Target,
			item.SizeBytes,
			item.AgeSec,
			item.ModTime.Format("2006-01-02T15:04:05Z07:00"),
			mdCell(item.Path),
		)
	}

	if p.data.Result != nil {
		result := p.data.Result
		fmt.Fprintln(w, "\n## 执行结果")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "- deleted_count: `%d`\n", result.DeletedCount)
		fmt.Fprintf(w, "- deleted_bytes: `%d`\n", result.DeletedBytes)
		fmt.Fprintf(w, "- failed_count: `%d`\n", len(result.Failed))
		fmt.Fprintf(w, "- remaining_count: `%d`\n", result.RemainingCount)
		fmt.Fprintf(w, "- remaining_bytes: `%d`\n", result.RemainingBytes)
	}

	renderWarningsMarkdown(w, plan.Warnings)
	if p.data.Result != nil {
		renderWarningsMarkdown(w, p.data.Result.Warnings)
	}
	return nil
}

func (p *cleanupPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.data == nil || p.data.Plan == nil {
		return emptyResultError("cleanup 结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"row_type",
		"mode",
		"target",
		"path",
		"size_bytes",
		"age_sec",
		"mod_time",
		"status",
		"message",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	plan := p.data.Plan
	if err := writer.Write([]string{
		"summary",
		p.data.Mode,
		joinTargets(plan.Targets),
		"",
		strconv.FormatInt(plan.CandidateBytes, 10),
		strconv.FormatInt(plan.OlderThanSec, 10),
		"",
		"",
		"",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 内容失败", err)
	}

	statusByPath := make(map[string]cleanupcollector.FailedItem)
	if p.data.Result != nil {
		for _, item := range p.data.Result.Failed {
			statusByPath[item.Path] = item
		}
	}

	for _, item := range plan.Candidates {
		status := "planned"
		message := ""
		if p.data.Result != nil {
			status = "deleted"
			if failed, ok := statusByPath[item.Path]; ok {
				status = "failed"
				message = failed.Message
			}
		}
		if err := writer.Write([]string{
			"candidate",
			p.data.Mode,
			string(item.Target),
			item.Path,
			strconv.FormatInt(item.SizeBytes, 10),
			strconv.FormatInt(item.AgeSec, 10),
			item.ModTime.Format("2006-01-02T15:04:05Z07:00"),
			status,
			message,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func renderCandidatesTable(w io.Writer, items []cleanupcollector.Candidate) {
	if len(items) == 0 {
		fmt.Fprintln(w, "(无候选文件)")
		return
	}
	fmt.Fprintf(w, "%-8s %-12s %-10s %-22s %s\n", "TARGET", "SIZE", "AGE", "MOD_TIME", "PATH")
	for _, item := range items {
		fmt.Fprintf(
			w,
			"%-8s %-12s %-10s %-22s %s\n",
			item.Target,
			formatBytes(uint64(max64(item.SizeBytes, 0))),
			formatAge(item.AgeSec),
			item.ModTime.Format("2006-01-02 15:04:05"),
			compact(item.Path, 120),
		)
	}
}

func renderWarningsTable(w io.Writer, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w, "\n提示")
	fmt.Fprintln(w, strings.Repeat("-", 100))
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

func joinTargets(targets []cleanupcollector.Target) string {
	if len(targets) == 0 {
		return ""
	}
	items := make([]string, 0, len(targets))
	for _, target := range targets {
		items = append(items, string(target))
	}
	return strings.Join(items, ",")
}

func formatAge(ageSec int64) string {
	if ageSec <= 0 {
		return "0s"
	}
	duration := time.Duration(ageSec) * time.Second
	return duration.String()
}

func formatBytes(value uint64) string {
	if value > math.MaxInt64 {
		return fmt.Sprintf("%d B", value)
	}
	return utils.FormatBytes(int64(value))
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

func max64(value int64, fallback int64) int64 {
	if value < 0 {
		return fallback
	}
	return value
}

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

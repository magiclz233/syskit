package doctor

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
)

type doctorPresenter struct {
	data *doctorOutput
}

func newDoctorPresenter(data *doctorOutput) *doctorPresenter {
	return &doctorPresenter{data: data}
}

func (p *doctorPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "doctor 输出结果为空")
	}

	fmt.Fprintln(w, "Doctor 诊断结果")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	if p.data.Scope == "all" {
		fmt.Fprintf(w, "范围: all (%s)\n", strings.Join(p.data.Modules, ","))
		fmt.Fprintf(w, "模式: %s\n", p.data.Mode)
	} else {
		fmt.Fprintf(w, "范围: module (%s)\n", p.data.Module)
	}
	fmt.Fprintf(w, "健康分: %d\n", p.data.HealthScore)
	fmt.Fprintf(w, "健康等级: %s\n", p.data.HealthLevel)
	fmt.Fprintf(w, "覆盖率: %.1f%%\n", p.data.Coverage)
	fmt.Fprintf(w, "fail-on: %s (matched=%t)\n", p.data.FailOn, p.data.FailOnMatched)
	fmt.Fprintf(w, "问题数: %d\n", len(p.data.Issues))
	fmt.Fprintf(w, "跳过模块: %d\n", len(p.data.Skipped))

	renderIssuesTable(w, p.data.Issues)
	renderSkippedTable(w, p.data.Skipped)
	renderWarningsTable(w, p.data.Warnings)
	return nil
}

func (p *doctorPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "doctor 输出结果为空")
	}

	fmt.Fprintln(w, "# Doctor 诊断结果")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- scope: `%s`\n", p.data.Scope)
	if p.data.Scope == "all" {
		fmt.Fprintf(w, "- mode: `%s`\n", p.data.Mode)
		fmt.Fprintf(w, "- modules: `%s`\n", strings.Join(p.data.Modules, ","))
	} else {
		fmt.Fprintf(w, "- module: `%s`\n", p.data.Module)
	}
	fmt.Fprintf(w, "- health_score: `%d`\n", p.data.HealthScore)
	fmt.Fprintf(w, "- health_level: `%s`\n", p.data.HealthLevel)
	fmt.Fprintf(w, "- coverage: `%.1f`\n", p.data.Coverage)
	fmt.Fprintf(w, "- fail_on: `%s`\n", p.data.FailOn)
	fmt.Fprintf(w, "- fail_on_matched: `%t`\n", p.data.FailOnMatched)

	fmt.Fprintln(w, "\n## Issues")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| RULE_ID | SEVERITY | SUMMARY | FIX_COMMAND |")
	fmt.Fprintln(w, "|---|---|---|---|")
	for _, issue := range p.data.Issues {
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n", issue.RuleID, issue.Severity, mdCell(issue.Summary), mdCell(issue.FixCommand))
	}
	if len(p.data.Issues) == 0 {
		fmt.Fprintln(w, "| - | - | 无问题 | - |")
	}

	if len(p.data.Skipped) > 0 {
		fmt.Fprintln(w, "\n## Skipped")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| MODULE | REASON | IMPACT | SUGGESTION |")
		fmt.Fprintln(w, "|---|---|---|---|")
		for _, item := range p.data.Skipped {
			fmt.Fprintf(w, "| %s | %s | %s | %s |\n", item.Module, item.Reason, mdCell(item.Impact), mdCell(item.Suggestion))
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

func (p *doctorPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "doctor 输出结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"row_type", "scope", "module", "health_score", "health_level", "coverage", "rule_id", "severity", "summary", "fix_command", "skip_module", "skip_reason", "impact", "suggestion", "warning"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	module := p.data.Module
	if p.data.Scope == "all" {
		module = strings.Join(p.data.Modules, ",")
	}
	if err := writer.Write([]string{"summary", p.data.Scope, module, strconv.Itoa(p.data.HealthScore), p.data.HealthLevel, fmt.Sprintf("%.1f", p.data.Coverage), "", "", "", "", "", "", "", "", ""}); err != nil {
		return errs.ExecutionFailed("写入 CSV 内容失败", err)
	}

	for _, issue := range p.data.Issues {
		if err := writer.Write([]string{"issue", p.data.Scope, p.data.Module, "", "", "", issue.RuleID, issue.Severity, issue.Summary, issue.FixCommand, "", "", "", "", ""}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	for _, item := range p.data.Skipped {
		if err := writer.Write([]string{"skipped", p.data.Scope, p.data.Module, "", "", "", "", "", "", "", item.Module, item.Reason, item.Impact, item.Suggestion, ""}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	for _, warning := range p.data.Warnings {
		if err := writer.Write([]string{"warning", p.data.Scope, p.data.Module, "", "", "", "", "", "", "", "", "", "", "", warning}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func renderIssuesTable(w io.Writer, issues []model.Issue) {
	fmt.Fprintln(w, "\n问题清单")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	if len(issues) == 0 {
		fmt.Fprintln(w, "(无问题)")
		return
	}
	fmt.Fprintf(w, "%-10s %-9s %-46s %s\n", "RULE_ID", "SEVERITY", "SUMMARY", "FIX_COMMAND")
	for _, issue := range issues {
		fmt.Fprintf(w, "%-10s %-9s %-46s %s\n", issue.RuleID, issue.Severity, compact(issue.Summary, 46), compact(issue.FixCommand, 48))
	}
}

func renderSkippedTable(w io.Writer, skipped []model.SkippedModule) {
	if len(skipped) == 0 {
		return
	}
	fmt.Fprintln(w, "\n跳过模块")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	fmt.Fprintf(w, "%-10s %-20s %-28s %s\n", "MODULE", "REASON", "IMPACT", "SUGGESTION")
	for _, item := range skipped {
		fmt.Fprintf(w, "%-10s %-20s %-28s %s\n", item.Module, item.Reason, compact(item.Impact, 28), compact(item.Suggestion, 40))
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

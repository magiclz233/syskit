package startup

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	startupcollector "syskit/internal/collectors/startup"
	"syskit/internal/errs"
)

type listPresenter struct {
	data *startupcollector.ListResult
}

func newListPresenter(data *startupcollector.ListResult) *listPresenter {
	return &listPresenter{data: data}
}

func (p *listPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "startup list 输出结果为空")
	}

	fmt.Fprintf(w, "Startup 列表（platform=%s）\n", p.data.Platform)
	fmt.Fprintf(w, "总数: %d\n", p.data.Total)
	if p.data.OnlyRisk {
		fmt.Fprintln(w, "过滤: only-risk=true")
	}
	if strings.TrimSpace(p.data.User) != "" {
		fmt.Fprintf(w, "过滤: user=%s\n", p.data.User)
	}
	fmt.Fprintf(w, "\n%-14s %-8s %-6s %-12s %-24s %s\n", "ID", "ENABLED", "RISK", "USER", "NAME", "PATH")
	for _, item := range p.data.Items {
		fmt.Fprintf(
			w,
			"%-14s %-8t %-6t %-12s %-24s %s\n",
			item.ID,
			item.Enabled,
			item.Risk,
			compact(item.User, 12),
			compact(item.Name, 24),
			item.SourcePath,
		)
	}
	renderWarnings(w, p.data.Warnings)
	return nil
}

func (p *listPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "startup list 输出结果为空")
	}
	fmt.Fprintln(w, "# Startup List")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- platform: `%s`\n", p.data.Platform)
	fmt.Fprintf(w, "- total: `%d`\n", p.data.Total)
	fmt.Fprintf(w, "- only_risk: `%t`\n", p.data.OnlyRisk)
	if p.data.User != "" {
		fmt.Fprintf(w, "- user: `%s`\n", mdCell(p.data.User))
	}
	fmt.Fprintln(w, "\n| ID | ENABLED | RISK | NAME | USER | PATH |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|")
	for _, item := range p.data.Items {
		fmt.Fprintf(
			w,
			"| %s | %t | %t | %s | %s | %s |\n",
			mdCell(item.ID),
			item.Enabled,
			item.Risk,
			mdCell(item.Name),
			mdCell(item.User),
			mdCell(item.SourcePath),
		)
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

func (p *listPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "startup list 输出结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"id", "name", "enabled", "risk", "risk_reason", "user", "location", "platform", "source_path", "command"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.data.Items {
		row := []string{
			item.ID,
			item.Name,
			strconv.FormatBool(item.Enabled),
			strconv.FormatBool(item.Risk),
			item.RiskReason,
			item.User,
			item.Location,
			item.Platform,
			item.SourcePath,
			item.Command,
		}
		if err := writer.Write(row); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

type actionPresenter struct {
	data *actionOutputData
}

func newActionPresenter(data *actionOutputData) *actionPresenter {
	return &actionPresenter{data: data}
}

func (p *actionPresenter) RenderTable(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "startup action 输出结果为空")
	}
	fmt.Fprintf(w, "Startup 动作计划（mode=%s）\n", p.data.Mode)
	fmt.Fprintf(w, "动作: %s\n", p.data.Plan.Action)
	fmt.Fprintf(w, "目标: %s\n", p.data.Plan.ID)
	fmt.Fprintf(w, "命中: %t\n", p.data.Plan.Found)
	if p.data.Plan.Found {
		fmt.Fprintf(w, "当前状态: enabled=%t risk=%t name=%s\n", p.data.Plan.Current.Enabled, p.data.Plan.Current.Risk, p.data.Plan.Current.Name)
	}
	fmt.Fprintln(w, "\n执行步骤")
	for idx, step := range p.data.Plan.Steps {
		fmt.Fprintf(w, "%d. %s\n", idx+1, step)
	}
	if p.data.Result != nil {
		fmt.Fprintln(w, "\n执行结果")
		fmt.Fprintf(w, "success: %t\n", p.data.Result.Success)
		fmt.Fprintf(w, "summary: %s\n", p.data.Result.Summary)
		fmt.Fprintf(w, "before_enabled: %t\n", p.data.Result.Before.Enabled)
		fmt.Fprintf(w, "after_enabled: %t\n", p.data.Result.After.Enabled)
		renderWarnings(w, p.data.Result.Warnings)
		return nil
	}
	renderWarnings(w, p.data.Plan.Warnings)
	return nil
}

func (p *actionPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "startup action 输出结果为空")
	}
	fmt.Fprintln(w, "# Startup Action")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- mode: `%s`\n", p.data.Mode)
	fmt.Fprintf(w, "- action: `%s`\n", p.data.Plan.Action)
	fmt.Fprintf(w, "- id: `%s`\n", mdCell(p.data.Plan.ID))
	fmt.Fprintf(w, "- found: `%t`\n", p.data.Plan.Found)
	if p.data.Plan.Found {
		fmt.Fprintf(w, "- current_enabled: `%t`\n", p.data.Plan.Current.Enabled)
	}
	fmt.Fprintln(w, "\n## Steps")
	fmt.Fprintln(w)
	for _, step := range p.data.Plan.Steps {
		fmt.Fprintf(w, "- %s\n", step)
	}
	if p.data.Result != nil {
		fmt.Fprintln(w, "\n## Result")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "- success: `%t`\n", p.data.Result.Success)
		fmt.Fprintf(w, "- summary: `%s`\n", mdCell(p.data.Result.Summary))
		fmt.Fprintf(w, "- before_enabled: `%t`\n", p.data.Result.Before.Enabled)
		fmt.Fprintf(w, "- after_enabled: `%t`\n", p.data.Result.After.Enabled)
		if len(p.data.Result.Warnings) > 0 {
			fmt.Fprintln(w, "\n### Warnings")
			fmt.Fprintln(w)
			for _, warning := range p.data.Result.Warnings {
				fmt.Fprintf(w, "- %s\n", warning)
			}
		}
		return nil
	}
	if len(p.data.Plan.Warnings) > 0 {
		fmt.Fprintln(w, "\n## Warnings")
		fmt.Fprintln(w)
		for _, warning := range p.data.Plan.Warnings {
			fmt.Fprintf(w, "- %s\n", warning)
		}
	}
	return nil
}

func (p *actionPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "startup action 输出结果为空")
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

	if err := write("summary", "mode", p.data.Mode, ""); err != nil {
		return err
	}
	if err := write("summary", "action", string(p.data.Plan.Action), ""); err != nil {
		return err
	}
	if err := write("summary", "id", p.data.Plan.ID, ""); err != nil {
		return err
	}
	for _, step := range p.data.Plan.Steps {
		if err := write("step", "plan", step, ""); err != nil {
			return err
		}
	}
	if p.data.Result != nil {
		if err := write("result", "success", strconv.FormatBool(p.data.Result.Success), p.data.Result.Summary); err != nil {
			return err
		}
		if err := write("result", "before_enabled", strconv.FormatBool(p.data.Result.Before.Enabled), ""); err != nil {
			return err
		}
		if err := write("result", "after_enabled", strconv.FormatBool(p.data.Result.After.Enabled), ""); err != nil {
			return err
		}
		for _, warning := range p.data.Result.Warnings {
			if err := write("warning", "message", warning, ""); err != nil {
				return err
			}
		}
		return nil
	}
	for _, warning := range p.data.Plan.Warnings {
		if err := write("warning", "message", warning, ""); err != nil {
			return err
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

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
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

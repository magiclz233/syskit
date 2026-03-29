package service

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	servicecollector "syskit/internal/collectors/service"
	"syskit/internal/errs"
)

type listPresenter struct {
	data *servicecollector.ListResult
}

func newListPresenter(data *servicecollector.ListResult) *listPresenter {
	return &listPresenter{data: data}
}

func (p *listPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "service list 输出结果为空")
	}

	fmt.Fprintf(w, "Service 列表（platform=%s）\n", p.data.Platform)
	if len(p.data.StateFilter) > 0 {
		fmt.Fprintf(w, "状态过滤: %s\n", strings.Join(p.data.StateFilter, ","))
	}
	if len(p.data.StartupFilter) > 0 {
		fmt.Fprintf(w, "启动类型过滤: %s\n", strings.Join(p.data.StartupFilter, ","))
	}
	if strings.TrimSpace(p.data.NameFilter) != "" {
		fmt.Fprintf(w, "名称过滤: %s\n", p.data.NameFilter)
	}
	fmt.Fprintf(w, "总数: %d\n", p.data.Total)
	fmt.Fprintf(w, "\n%-36s %-12s %-10s %-8s %s\n", "NAME", "STATE", "STARTUP", "PID", "DISPLAY")
	for _, item := range p.data.Services {
		pid := "-"
		if item.PID > 0 {
			pid = strconv.Itoa(int(item.PID))
		}
		display := item.DisplayName
		if strings.TrimSpace(display) == "" {
			display = item.Name
		}
		fmt.Fprintf(w, "%-36s %-12s %-10s %-8s %s\n", item.Name, item.State, item.Startup, pid, display)
	}
	renderWarnings(w, p.data.Warnings)
	return nil
}

func (p *listPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "service list 输出结果为空")
	}

	fmt.Fprintln(w, "# Service List")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- platform: `%s`\n", p.data.Platform)
	fmt.Fprintf(w, "- total: `%d`\n", p.data.Total)
	if len(p.data.StateFilter) > 0 {
		fmt.Fprintf(w, "- state_filter: `%s`\n", strings.Join(p.data.StateFilter, ","))
	}
	if len(p.data.StartupFilter) > 0 {
		fmt.Fprintf(w, "- startup_filter: `%s`\n", strings.Join(p.data.StartupFilter, ","))
	}
	if strings.TrimSpace(p.data.NameFilter) != "" {
		fmt.Fprintf(w, "- name_filter: `%s`\n", mdCell(p.data.NameFilter))
	}

	fmt.Fprintln(w, "\n| NAME | STATE | STARTUP | PID | DISPLAY |")
	fmt.Fprintln(w, "|---|---|---|---:|---|")
	for _, item := range p.data.Services {
		fmt.Fprintf(
			w,
			"| %s | %s | %s | %d | %s |\n",
			mdCell(item.Name),
			mdCell(item.State),
			mdCell(item.Startup),
			item.PID,
			mdCell(item.DisplayName),
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
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "service list 输出结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"name", "display_name", "state", "startup", "pid", "platform", "description"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.data.Services {
		row := []string{
			item.Name,
			item.DisplayName,
			item.State,
			item.Startup,
			strconv.Itoa(int(item.PID)),
			item.Platform,
			item.Description,
		}
		if err := writer.Write(row); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

type checkPresenter struct {
	data *servicecollector.CheckResult
}

func newCheckPresenter(data *servicecollector.CheckResult) *checkPresenter {
	return &checkPresenter{data: data}
}

func (p *checkPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "service check 输出结果为空")
	}

	fmt.Fprintf(w, "Service 检查结果（platform=%s）\n", p.data.Platform)
	fmt.Fprintf(w, "目标: %s\n", p.data.Name)
	fmt.Fprintf(w, "匹配模式: %s\n", checkModeText(p.data.All))
	fmt.Fprintf(w, "详情模式: %t\n", p.data.Detail)
	fmt.Fprintf(w, "命中: %d，running: %d，healthy: %t\n", p.data.Matched, p.data.Running, p.data.Healthy)
	fmt.Fprintf(w, "结论: %s\n", p.data.Summary)

	if len(p.data.Services) > 0 {
		fmt.Fprintf(w, "\n%-36s %-12s %-10s %-8s %s\n", "NAME", "STATE", "STARTUP", "PID", "DISPLAY")
		for _, item := range p.data.Services {
			pid := "-"
			if item.PID > 0 {
				pid = strconv.Itoa(int(item.PID))
			}
			display := item.DisplayName
			if strings.TrimSpace(display) == "" {
				display = item.Name
			}
			fmt.Fprintf(w, "%-36s %-12s %-10s %-8s %s\n", item.Name, item.State, item.Startup, pid, display)
		}
	}
	renderWarnings(w, p.data.Warnings)
	return nil
}

func (p *checkPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "service check 输出结果为空")
	}

	fmt.Fprintln(w, "# Service Check")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- platform: `%s`\n", p.data.Platform)
	fmt.Fprintf(w, "- name: `%s`\n", mdCell(p.data.Name))
	fmt.Fprintf(w, "- all: `%t`\n", p.data.All)
	fmt.Fprintf(w, "- detail: `%t`\n", p.data.Detail)
	fmt.Fprintf(w, "- found: `%t`\n", p.data.Found)
	fmt.Fprintf(w, "- healthy: `%t`\n", p.data.Healthy)
	fmt.Fprintf(w, "- matched: `%d`\n", p.data.Matched)
	fmt.Fprintf(w, "- running: `%d`\n", p.data.Running)
	fmt.Fprintf(w, "- summary: `%s`\n", mdCell(p.data.Summary))

	if len(p.data.Services) > 0 {
		fmt.Fprintln(w, "\n| NAME | STATE | STARTUP | PID | DISPLAY |")
		fmt.Fprintln(w, "|---|---|---|---:|---|")
		for _, item := range p.data.Services {
			fmt.Fprintf(
				w,
				"| %s | %s | %s | %d | %s |\n",
				mdCell(item.Name),
				mdCell(item.State),
				mdCell(item.Startup),
				item.PID,
				mdCell(item.DisplayName),
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

func (p *checkPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "service check 输出结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"name", "display_name", "state", "startup", "pid", "platform", "description"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.data.Services {
		row := []string{
			item.Name,
			item.DisplayName,
			item.State,
			item.Startup,
			strconv.Itoa(int(item.PID)),
			item.Platform,
			item.Description,
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
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "service action 输出结果为空")
	}

	fmt.Fprintf(w, "Service 动作计划（mode=%s）\n", p.data.Mode)
	fmt.Fprintf(w, "动作: %s\n", p.data.Plan.Action)
	fmt.Fprintf(w, "目标: %s\n", p.data.Plan.Name)
	fmt.Fprintf(w, "平台: %s\n", p.data.Plan.Platform)
	fmt.Fprintf(w, "命中服务: %t\n", p.data.Plan.Found)
	if p.data.Plan.Found {
		fmt.Fprintf(w, "当前状态: state=%s startup=%s pid=%d\n", p.data.Plan.Current.State, p.data.Plan.Current.Startup, p.data.Plan.Current.PID)
	}
	fmt.Fprintln(w, "\n执行步骤")
	for idx, step := range p.data.Plan.Steps {
		fmt.Fprintf(w, "%d. %s\n", idx+1, step)
	}

	if p.data.Result != nil {
		fmt.Fprintln(w, "\n执行结果")
		fmt.Fprintf(w, "success: %t\n", p.data.Result.Success)
		fmt.Fprintf(w, "summary: %s\n", p.data.Result.Summary)
		if p.data.Result.Before.Name != "" || p.data.Result.After.Name != "" {
			fmt.Fprintf(w, "before: state=%s startup=%s\n", p.data.Result.Before.State, p.data.Result.Before.Startup)
			fmt.Fprintf(w, "after : state=%s startup=%s\n", p.data.Result.After.State, p.data.Result.After.Startup)
		}
		renderWarnings(w, p.data.Result.Warnings)
		return nil
	}

	renderWarnings(w, p.data.Plan.Warnings)
	return nil
}

func (p *actionPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "service action 输出结果为空")
	}

	fmt.Fprintln(w, "# Service Action")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- mode: `%s`\n", p.data.Mode)
	fmt.Fprintf(w, "- action: `%s`\n", p.data.Plan.Action)
	fmt.Fprintf(w, "- name: `%s`\n", mdCell(p.data.Plan.Name))
	fmt.Fprintf(w, "- platform: `%s`\n", p.data.Plan.Platform)
	fmt.Fprintf(w, "- found: `%t`\n", p.data.Plan.Found)
	if p.data.Plan.Found {
		fmt.Fprintf(w, "- current_state: `%s`\n", p.data.Plan.Current.State)
		fmt.Fprintf(w, "- current_startup: `%s`\n", p.data.Plan.Current.Startup)
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
		fmt.Fprintf(w, "- before_state: `%s`\n", p.data.Result.Before.State)
		fmt.Fprintf(w, "- after_state: `%s`\n", p.data.Result.After.State)
		fmt.Fprintf(w, "- before_startup: `%s`\n", p.data.Result.Before.Startup)
		fmt.Fprintf(w, "- after_startup: `%s`\n", p.data.Result.After.Startup)
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
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "service action 输出结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"row_type", "key", "value", "extra"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	writeRow := func(rowType string, key string, value string, extra string) error {
		if err := writer.Write([]string{rowType, key, value, extra}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
		return nil
	}

	if err := writeRow("summary", "mode", p.data.Mode, ""); err != nil {
		return err
	}
	if err := writeRow("summary", "action", string(p.data.Plan.Action), ""); err != nil {
		return err
	}
	if err := writeRow("summary", "name", p.data.Plan.Name, ""); err != nil {
		return err
	}
	for _, step := range p.data.Plan.Steps {
		if err := writeRow("step", "plan", step, ""); err != nil {
			return err
		}
	}
	if p.data.Result != nil {
		if err := writeRow("result", "success", strconv.FormatBool(p.data.Result.Success), p.data.Result.Summary); err != nil {
			return err
		}
		if err := writeRow("result", "before_state", p.data.Result.Before.State, p.data.Result.Before.Startup); err != nil {
			return err
		}
		if err := writeRow("result", "after_state", p.data.Result.After.State, p.data.Result.After.Startup); err != nil {
			return err
		}
		for _, warning := range p.data.Result.Warnings {
			if err := writeRow("warning", "message", warning, ""); err != nil {
				return err
			}
		}
		return nil
	}
	for _, warning := range p.data.Plan.Warnings {
		if err := writeRow("warning", "message", warning, ""); err != nil {
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
	for _, item := range warnings {
		fmt.Fprintf(w, "- %s\n", item)
	}
}

func checkModeText(all bool) string {
	if all {
		return "模糊匹配"
	}
	return "精确匹配"
}

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

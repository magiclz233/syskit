package snapshot

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"syskit/internal/errs"
	"syskit/pkg/utils"
)

type createPresenter struct {
	data *createOutputData
}

type listPresenter struct {
	data *listOutputData
}

type showPresenter struct {
	data *showOutputData
}

type diffPresenter struct {
	data *diffOutputData
}

type deletePresenter struct {
	data *deleteOutputData
}

func newCreatePresenter(data *createOutputData) *createPresenter {
	return &createPresenter{data: data}
}

func newListPresenter(data *listOutputData) *listPresenter {
	return &listPresenter{data: data}
}

func newShowPresenter(data *showOutputData) *showPresenter {
	return &showPresenter{data: data}
}

func newDiffPresenter(data *diffOutputData) *diffPresenter {
	return &diffPresenter{data: data}
}

func newDeletePresenter(data *deleteOutputData) *deletePresenter {
	return &deletePresenter{data: data}
}

func (p *createPresenter) RenderTable(w io.Writer) error {
	if p.data == nil || p.data.Snapshot == nil {
		return emptyResultError("snapshot create 结果为空")
	}
	s := p.data.Snapshot
	fmt.Fprintf(w, "快照 ID: %s\n", s.ID)
	fmt.Fprintf(w, "名称: %s\n", s.Name)
	if strings.TrimSpace(s.Description) != "" {
		fmt.Fprintf(w, "描述: %s\n", s.Description)
	}
	fmt.Fprintf(w, "创建时间: %s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "主机: %s\n", displayValue(s.Host, "-"))
	fmt.Fprintf(w, "平台: %s\n", displayValue(s.Platform, "-"))
	fmt.Fprintf(w, "模块: %s\n", joinModules(s.Modules))
	fmt.Fprintf(w, "大小: %s\n", utils.FormatBytes(s.SizeBytes))
	renderLines(w, "告警", p.data.Warnings)
	renderLines(w, "跳过模块", p.data.SkippedModules)
	return nil
}

func (p *createPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Snapshot == nil {
		return emptyResultError("snapshot create 结果为空")
	}
	s := p.data.Snapshot
	fmt.Fprintln(w, "# Snapshot Create")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- id: `%s`\n", s.ID)
	fmt.Fprintf(w, "- name: `%s`\n", mdCell(s.Name))
	if strings.TrimSpace(s.Description) != "" {
		fmt.Fprintf(w, "- description: `%s`\n", mdCell(s.Description))
	}
	fmt.Fprintf(w, "- created_at: `%s`\n", s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(w, "- host: `%s`\n", mdCell(displayValue(s.Host, "-")))
	fmt.Fprintf(w, "- platform: `%s`\n", mdCell(displayValue(s.Platform, "-")))
	fmt.Fprintf(w, "- modules: `%s`\n", mdCell(joinModules(s.Modules)))
	fmt.Fprintf(w, "- size_bytes: `%d`\n", s.SizeBytes)
	renderMarkdownLines(w, "告警", p.data.Warnings)
	renderMarkdownLines(w, "跳过模块", p.data.SkippedModules)
	return nil
}

func (p *createPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.data == nil || p.data.Snapshot == nil {
		return emptyResultError("snapshot create 结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	s := p.data.Snapshot
	if err := writer.Write([]string{"id", "name", "description", "created_at", "host", "platform", "modules", "size_bytes"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	if err := writer.Write([]string{s.ID, s.Name, s.Description, s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), s.Host, s.Platform, joinModules(s.Modules), strconv.FormatInt(s.SizeBytes, 10)}); err != nil {
		return errs.ExecutionFailed("写入 CSV 内容失败", err)
	}
	return nil
}

func (p *listPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return emptyResultError("snapshot list 结果为空")
	}
	fmt.Fprintf(w, "返回条数: %d (limit=%d)\n\n", p.data.Count, p.data.Limit)
	if len(p.data.Snapshots) == 0 {
		fmt.Fprintln(w, "(无快照)")
		return nil
	}
	fmt.Fprintf(w, "%-24s %-20s %-19s %-8s %-10s %s\n", "ID", "NAME", "CREATED_AT", "MODULES", "SIZE", "HOST")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	for _, item := range p.data.Snapshots {
		fmt.Fprintf(w, "%-24s %-20s %-19s %-8d %-10s %s\n",
			compact(item.ID, 24),
			compact(item.Name, 20),
			item.CreatedAt.Format("2006-01-02 15:04:05"),
			len(item.Modules),
			utils.FormatBytes(item.SizeBytes),
			compact(displayValue(item.Host, "-"), 16),
		)
	}
	return nil
}

func (p *listPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return emptyResultError("snapshot list 结果为空")
	}
	fmt.Fprintln(w, "# Snapshot List")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- count: `%d`\n", p.data.Count)
	fmt.Fprintf(w, "- limit: `%d`\n", p.data.Limit)
	fmt.Fprintln(w)
	if len(p.data.Snapshots) == 0 {
		fmt.Fprintln(w, "(无快照)")
		return nil
	}
	fmt.Fprintln(w, "| ID | NAME | CREATED_AT | MODULES | SIZE_BYTES | HOST |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|")
	for _, item := range p.data.Snapshots {
		fmt.Fprintf(w, "| %s | %s | %s | %d | %d | %s |\n",
			mdCell(item.ID),
			mdCell(item.Name),
			item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			len(item.Modules),
			item.SizeBytes,
			mdCell(displayValue(item.Host, "-")),
		)
	}
	return nil
}

func (p *listPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.data == nil {
		return emptyResultError("snapshot list 结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"id", "name", "description", "created_at", "host", "platform", "module_count", "modules", "size_bytes"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.data.Snapshots {
		if err := writer.Write([]string{
			item.ID,
			item.Name,
			item.Description,
			item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			item.Host,
			item.Platform,
			strconv.Itoa(len(item.Modules)),
			joinModules(item.Modules),
			strconv.FormatInt(item.SizeBytes, 10),
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *showPresenter) RenderTable(w io.Writer) error {
	if p.data == nil || p.data.Snapshot == nil {
		return emptyResultError("snapshot show 结果为空")
	}
	s := p.data.Snapshot
	fmt.Fprintf(w, "快照 ID: %s\n", s.ID)
	fmt.Fprintf(w, "名称: %s\n", s.Name)
	if strings.TrimSpace(s.Description) != "" {
		fmt.Fprintf(w, "描述: %s\n", s.Description)
	}
	fmt.Fprintf(w, "创建时间: %s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "模块数: %d\n", len(s.Modules))
	if len(p.data.SelectedModules) > 0 {
		fmt.Fprintf(w, "筛选模块: %s\n", strings.Join(p.data.SelectedModules, ","))
	}
	if len(p.data.MissingModules) > 0 {
		fmt.Fprintf(w, "未命中模块: %s\n", strings.Join(p.data.MissingModules, ","))
	}
	fmt.Fprintln(w)
	for _, module := range sortedModuleKeys(s.Modules) {
		fmt.Fprintf(w, "- %s\n", module)
	}
	renderLines(w, "告警", s.Warnings)
	return nil
}

func (p *showPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Snapshot == nil {
		return emptyResultError("snapshot show 结果为空")
	}
	s := p.data.Snapshot
	fmt.Fprintln(w, "# Snapshot Show")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- id: `%s`\n", s.ID)
	fmt.Fprintf(w, "- name: `%s`\n", mdCell(s.Name))
	if strings.TrimSpace(s.Description) != "" {
		fmt.Fprintf(w, "- description: `%s`\n", mdCell(s.Description))
	}
	fmt.Fprintf(w, "- created_at: `%s`\n", s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(w, "- module_count: `%d`\n", len(s.Modules))
	if len(p.data.SelectedModules) > 0 {
		fmt.Fprintf(w, "- selected_modules: `%s`\n", mdCell(strings.Join(p.data.SelectedModules, ",")))
	}
	if len(p.data.MissingModules) > 0 {
		fmt.Fprintf(w, "- missing_modules: `%s`\n", mdCell(strings.Join(p.data.MissingModules, ",")))
	}
	fmt.Fprintln(w, "\n## Modules")
	fmt.Fprintln(w)
	for _, module := range sortedModuleKeys(s.Modules) {
		fmt.Fprintf(w, "- `%s`\n", module)
	}
	renderMarkdownLines(w, "告警", s.Warnings)
	return nil
}

func (p *showPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.data == nil || p.data.Snapshot == nil {
		return emptyResultError("snapshot show 结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	s := p.data.Snapshot
	if err := writer.Write([]string{"id", "name", "module", "warning_count"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	modules := sortedModuleKeys(s.Modules)
	if len(modules) == 0 {
		if err := writer.Write([]string{s.ID, s.Name, "", strconv.Itoa(len(s.Warnings))}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
		return nil
	}
	for _, module := range modules {
		if err := writer.Write([]string{s.ID, s.Name, module, strconv.Itoa(len(s.Warnings))}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *diffPresenter) RenderTable(w io.Writer) error {
	if p.data == nil || p.data.Diff == nil {
		return emptyResultError("snapshot diff 结果为空")
	}
	d := p.data.Diff
	fmt.Fprintf(w, "基线快照: %s\n", p.data.BaseID)
	fmt.Fprintf(w, "目标快照: %s\n", p.data.TargetID)
	if p.data.AutoSelectedTarget != "" {
		fmt.Fprintf(w, "自动选择目标: %s\n", p.data.AutoSelectedTarget)
	}
	fmt.Fprintf(w, "仅变化项: %t\n", p.data.OnlyChange)
	fmt.Fprintf(w, "比较模块数: %d\n", len(d.ComparedModules))
	fmt.Fprintf(w, "变化统计: added=%d removed=%d changed=%d unchanged=%d\n\n", len(d.Added), len(d.Removed), len(d.Changed), len(d.Unchanged))

	renderModuleDiffTable(w, "新增模块", d.Added)
	renderModuleDiffTable(w, "删除模块", d.Removed)
	renderModuleDiffTable(w, "变化模块", d.Changed)
	if len(d.Unchanged) > 0 {
		fmt.Fprintf(w, "无变化模块: %s\n", strings.Join(d.Unchanged, ","))
	}
	return nil
}

func (p *diffPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Diff == nil {
		return emptyResultError("snapshot diff 结果为空")
	}
	d := p.data.Diff
	fmt.Fprintln(w, "# Snapshot Diff")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- base_id: `%s`\n", p.data.BaseID)
	fmt.Fprintf(w, "- target_id: `%s`\n", p.data.TargetID)
	if p.data.AutoSelectedTarget != "" {
		fmt.Fprintf(w, "- auto_selected_target: `%s`\n", p.data.AutoSelectedTarget)
	}
	fmt.Fprintf(w, "- only_change: `%t`\n", p.data.OnlyChange)
	fmt.Fprintf(w, "- compared_modules: `%d`\n", len(d.ComparedModules))
	fmt.Fprintf(w, "- has_changes: `%t`\n", d.HasChanges)
	fmt.Fprintln(w)
	renderModuleDiffMarkdown(w, "新增模块", d.Added)
	renderModuleDiffMarkdown(w, "删除模块", d.Removed)
	renderModuleDiffMarkdown(w, "变化模块", d.Changed)
	if len(d.Unchanged) > 0 {
		fmt.Fprintln(w, "## 无变化模块")
		fmt.Fprintln(w)
		for _, item := range d.Unchanged {
			fmt.Fprintf(w, "- `%s`\n", item)
		}
	}
	return nil
}

func (p *diffPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.data == nil || p.data.Diff == nil {
		return emptyResultError("snapshot diff 结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"module", "change_type", "risk"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	writeRows := func(items []moduleDiff) error {
		for _, item := range items {
			if err := writer.Write([]string{item.Module, item.ChangeType, item.Risk}); err != nil {
				return errs.ExecutionFailed("写入 CSV 内容失败", err)
			}
		}
		return nil
	}
	if err := writeRows(p.data.Diff.Added); err != nil {
		return err
	}
	if err := writeRows(p.data.Diff.Removed); err != nil {
		return err
	}
	if err := writeRows(p.data.Diff.Changed); err != nil {
		return err
	}
	if !p.data.OnlyChange {
		for _, item := range p.data.Diff.Unchanged {
			if err := writer.Write([]string{item, "unchanged", ""}); err != nil {
				return errs.ExecutionFailed("写入 CSV 内容失败", err)
			}
		}
	}
	return nil
}

func (p *deletePresenter) RenderTable(w io.Writer) error {
	if p.data == nil || p.data.Snapshot == nil {
		return emptyResultError("snapshot delete 结果为空")
	}
	s := p.data.Snapshot
	fmt.Fprintf(w, "模式: %s\n", p.data.Mode)
	fmt.Fprintf(w, "apply: %t\n", p.data.Apply)
	fmt.Fprintf(w, "deleted: %t\n", p.data.Deleted)
	fmt.Fprintf(w, "快照 ID: %s\n", s.ID)
	fmt.Fprintf(w, "名称: %s\n", s.Name)
	fmt.Fprintf(w, "创建时间: %s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "模块: %s\n", joinModules(s.Modules))
	fmt.Fprintf(w, "大小: %s\n", utils.FormatBytes(s.SizeBytes))
	return nil
}

func (p *deletePresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Snapshot == nil {
		return emptyResultError("snapshot delete 结果为空")
	}
	s := p.data.Snapshot
	fmt.Fprintln(w, "# Snapshot Delete")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- mode: `%s`\n", p.data.Mode)
	fmt.Fprintf(w, "- apply: `%t`\n", p.data.Apply)
	fmt.Fprintf(w, "- deleted: `%t`\n", p.data.Deleted)
	fmt.Fprintf(w, "- id: `%s`\n", s.ID)
	fmt.Fprintf(w, "- name: `%s`\n", mdCell(s.Name))
	fmt.Fprintf(w, "- created_at: `%s`\n", s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(w, "- module_count: `%d`\n", len(s.Modules))
	fmt.Fprintf(w, "- size_bytes: `%d`\n", s.SizeBytes)
	return nil
}

func (p *deletePresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.data == nil || p.data.Snapshot == nil {
		return emptyResultError("snapshot delete 结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	s := p.data.Snapshot
	if err := writer.Write([]string{"mode", "apply", "deleted", "id", "name", "created_at", "module_count", "size_bytes"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	if err := writer.Write([]string{p.data.Mode, strconv.FormatBool(p.data.Apply), strconv.FormatBool(p.data.Deleted), s.ID, s.Name, s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), strconv.Itoa(len(s.Modules)), strconv.FormatInt(s.SizeBytes, 10)}); err != nil {
		return errs.ExecutionFailed("写入 CSV 内容失败", err)
	}
	return nil
}

func renderLines(w io.Writer, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", title)
	fmt.Fprintln(w, strings.Repeat("-", 80))
	for _, item := range items {
		fmt.Fprintf(w, "- %s\n", item)
	}
}

func renderMarkdownLines(w io.Writer, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "\n## %s\n\n", title)
	for _, item := range items {
		fmt.Fprintf(w, "- %s\n", mdCell(item))
	}
}

func renderModuleDiffTable(w io.Writer, title string, items []moduleDiff) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "%s\n", title)
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "%-12s %-10s %s\n", "MODULE", "TYPE", "RISK")
	for _, item := range items {
		fmt.Fprintf(w, "%-12s %-10s %s\n", item.Module, item.ChangeType, displayValue(item.Risk, "-"))
	}
	fmt.Fprintln(w)
}

func renderModuleDiffMarkdown(w io.Writer, title string, items []moduleDiff) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "## %s\n\n", title)
	fmt.Fprintln(w, "| MODULE | TYPE | RISK |")
	fmt.Fprintln(w, "|---|---|---|")
	for _, item := range items {
		fmt.Fprintf(w, "| %s | %s | %s |\n", mdCell(item.Module), mdCell(item.ChangeType), mdCell(displayValue(item.Risk, "-")))
	}
	fmt.Fprintln(w)
}

func joinModules(modules []string) string {
	if len(modules) == 0 {
		return "-"
	}
	items := append([]string(nil), modules...)
	sort.Strings(items)
	return strings.Join(items, ",")
}

func sortedModuleKeys(modules map[string]any) []string {
	if len(modules) == 0 {
		return nil
	}
	keys := make([]string, 0, len(modules))
	for key := range modules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

func displayValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

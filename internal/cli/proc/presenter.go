package proc

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	proccollector "syskit/internal/collectors/proc"
	"syskit/internal/errs"
	"syskit/pkg/utils"
)

type topPresenter struct {
	result *proccollector.TopResult
}

type treePresenter struct {
	result *proccollector.TreeResult
}

type infoPresenter struct {
	result *proccollector.InfoResult
}

type killPresenter struct {
	result *killOutputData
}

func newTopPresenter(result *proccollector.TopResult) *topPresenter {
	return &topPresenter{result: result}
}

func newTreePresenter(result *proccollector.TreeResult) *treePresenter {
	return &treePresenter{result: result}
}

func newInfoPresenter(result *proccollector.InfoResult) *infoPresenter {
	return &infoPresenter{result: result}
}

func newKillPresenter(result *killOutputData) *killPresenter {
	return &killPresenter{result: result}
}

func (p *topPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("进程排行结果为空")
	}

	fmt.Fprintf(w, "进程排行（by=%s, top=%d）\n", p.result.By, p.result.TopN)
	fmt.Fprintf(w, "命中进程数: %d\n", p.result.TotalMatched)
	fmt.Fprintf(w, "%-8s %-8s %-20s %-8s %-12s %-8s %-18s %s\n", "PID", "PPID", "USER", "CPU%", "RSS", "FD", "NAME", "COMMAND")
	fmt.Fprintln(w, strings.Repeat("-", 120))

	for _, item := range p.result.Processes {
		fmt.Fprintf(
			w,
			"%-8d %-8d %-20s %-8.2f %-12s %-8d %-18s %s\n",
			item.PID,
			item.PPID,
			displayValue(item.User, "-"),
			item.CPUPercent,
			formatBytes(item.RSSBytes),
			item.FDCount,
			displayValue(item.Name, "-"),
			compact(displayValue(item.Command, "-"), 80),
		)
	}

	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *topPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("进程排行结果为空")
	}

	fmt.Fprintln(w, "# 进程排行")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- 排序维度: `%s`\n", p.result.By)
	fmt.Fprintf(w, "- TopN: `%d`\n", p.result.TopN)
	fmt.Fprintf(w, "- 命中进程数: `%d`\n", p.result.TotalMatched)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "| PID | PPID | USER | CPU% | RSS | FD | NAME | COMMAND |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|---|---|")
	for _, item := range p.result.Processes {
		fmt.Fprintf(
			w,
			"| %d | %d | %s | %.2f | %s | %d | %s | %s |\n",
			item.PID,
			item.PPID,
			mdCell(displayValue(item.User, "-")),
			item.CPUPercent,
			mdCell(formatBytes(item.RSSBytes)),
			item.FDCount,
			mdCell(displayValue(item.Name, "-")),
			mdCell(compact(displayValue(item.Command, "-"), 120)),
		)
	}

	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *topPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("进程排行结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"pid",
		"ppid",
		"user",
		"cpu_percent",
		"cpu_seconds",
		"rss_bytes",
		"vms_bytes",
		"io_read_bytes",
		"io_write_bytes",
		"fd_count",
		"thread_count",
		"name",
		"command",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	for _, item := range p.result.Processes {
		if err := writer.Write([]string{
			strconv.FormatInt(int64(item.PID), 10),
			strconv.FormatInt(int64(item.PPID), 10),
			item.User,
			fmt.Sprintf("%.2f", item.CPUPercent),
			fmt.Sprintf("%.2f", item.CPUSeconds),
			strconv.FormatUint(item.RSSBytes, 10),
			strconv.FormatUint(item.VMSBytes, 10),
			strconv.FormatUint(item.IOReadBytes, 10),
			strconv.FormatUint(item.IOWriteBytes, 10),
			strconv.FormatInt(int64(item.FDCount), 10),
			strconv.FormatInt(int64(item.ThreadCount), 10),
			item.Name,
			item.Command,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}

	return nil
}

func (p *treePresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("进程树结果为空")
	}

	fmt.Fprintf(w, "进程树（detail=%t, full=%t）\n", p.result.Detail, p.result.Full)
	for _, node := range p.result.Nodes {
		renderTreeTableNode(w, node, 0, p.result.Detail)
	}
	if p.result.Truncated {
		fmt.Fprintln(w, "\n提示: 输出已截断，可通过 --full 查看完整层级")
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *treePresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("进程树结果为空")
	}

	fmt.Fprintln(w, "# 进程树")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- detail: `%t`\n", p.result.Detail)
	fmt.Fprintf(w, "- full: `%t`\n", p.result.Full)
	if p.result.RootPID != nil {
		fmt.Fprintf(w, "- root_pid: `%d`\n", *p.result.RootPID)
	}
	fmt.Fprintln(w)

	for _, node := range p.result.Nodes {
		renderTreeMarkdownNode(w, node, 0, p.result.Detail)
	}
	if p.result.Truncated {
		fmt.Fprintln(w, "\n- 提示: 输出已截断，可通过 `--full` 查看完整层级")
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *treePresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("进程树结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"depth", "pid", "ppid", "name", "user", "cpu_percent", "rss_bytes", "command"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	rows := make([][]string, 0, 256)
	for _, node := range p.result.Nodes {
		flattenTreeRows(node, 0, &rows)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *infoPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("进程详情结果为空")
	}

	proc := p.result.Process
	fmt.Fprintln(w, "进程详情")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "PID: %d\n", proc.PID)
	fmt.Fprintf(w, "PPID: %d\n", proc.PPID)
	fmt.Fprintf(w, "名称: %s\n", displayValue(proc.Name, "-"))
	fmt.Fprintf(w, "用户: %s\n", displayValue(proc.User, "-"))
	fmt.Fprintf(w, "CPU: %.2f%% (累计 %.2fs)\n", proc.CPUPercent, proc.CPUSeconds)
	fmt.Fprintf(w, "RSS: %s\n", formatBytes(proc.RSSBytes))
	fmt.Fprintf(w, "VMS: %s\n", formatBytes(proc.VMSBytes))
	fmt.Fprintf(w, "IO: 读 %s / 写 %s\n", formatBytes(proc.IOReadBytes), formatBytes(proc.IOWriteBytes))
	fmt.Fprintf(w, "FD: %d\n", proc.FDCount)
	fmt.Fprintf(w, "线程数: %d\n", proc.ThreadCount)
	if !proc.StartTime.IsZero() {
		fmt.Fprintf(w, "启动时间(UTC): %s\n", proc.StartTime.Format(timeLayout))
	}
	if proc.Executable != "" {
		fmt.Fprintf(w, "可执行文件: %s\n", proc.Executable)
	}
	if proc.Command != "" {
		fmt.Fprintf(w, "命令行: %s\n", proc.Command)
	}

	if p.result.Parent != nil {
		fmt.Fprintf(w, "父进程: %s(%d)\n", displayValue(p.result.Parent.Name, "-"), p.result.Parent.PID)
	}

	if len(p.result.Children) > 0 {
		fmt.Fprintln(w, "\n子进程")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		fmt.Fprintf(w, "%-8s %-20s %-8s %-12s %s\n", "PID", "NAME", "CPU%", "RSS", "COMMAND")
		for _, item := range p.result.Children {
			fmt.Fprintf(
				w,
				"%-8d %-20s %-8.2f %-12s %s\n",
				item.PID,
				displayValue(item.Name, "-"),
				item.CPUPercent,
				formatBytes(item.RSSBytes),
				compact(displayValue(item.Command, "-"), 80),
			)
		}
	}

	if len(p.result.Environment) > 0 {
		fmt.Fprintln(w, "\n环境变量")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		keys := sortedEnvKeys(p.result.Environment)
		for _, key := range keys {
			fmt.Fprintf(w, "%s=%s\n", key, p.result.Environment[key])
		}
	}

	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *infoPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("进程详情结果为空")
	}

	proc := p.result.Process
	fmt.Fprintln(w, "# 进程详情")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| 字段 | 值 |")
	fmt.Fprintln(w, "|---|---|")
	fmt.Fprintf(w, "| pid | %d |\n", proc.PID)
	fmt.Fprintf(w, "| ppid | %d |\n", proc.PPID)
	fmt.Fprintf(w, "| name | %s |\n", mdCell(displayValue(proc.Name, "-")))
	fmt.Fprintf(w, "| user | %s |\n", mdCell(displayValue(proc.User, "-")))
	fmt.Fprintf(w, "| cpu_percent | %.2f |\n", proc.CPUPercent)
	fmt.Fprintf(w, "| cpu_seconds | %.2f |\n", proc.CPUSeconds)
	fmt.Fprintf(w, "| rss_bytes | %d |\n", proc.RSSBytes)
	fmt.Fprintf(w, "| vms_bytes | %d |\n", proc.VMSBytes)
	fmt.Fprintf(w, "| io_read_bytes | %d |\n", proc.IOReadBytes)
	fmt.Fprintf(w, "| io_write_bytes | %d |\n", proc.IOWriteBytes)
	fmt.Fprintf(w, "| fd_count | %d |\n", proc.FDCount)
	fmt.Fprintf(w, "| thread_count | %d |\n", proc.ThreadCount)
	if !proc.StartTime.IsZero() {
		fmt.Fprintf(w, "| start_time | %s |\n", proc.StartTime.Format(timeLayout))
	}
	if proc.Executable != "" {
		fmt.Fprintf(w, "| executable | %s |\n", mdCell(proc.Executable))
	}
	if proc.Command != "" {
		fmt.Fprintf(w, "| command | %s |\n", mdCell(proc.Command))
	}

	if p.result.Parent != nil {
		fmt.Fprintln(w, "\n## 父进程")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "- %s(%d)\n", p.result.Parent.Name, p.result.Parent.PID)
	}

	if len(p.result.Children) > 0 {
		fmt.Fprintln(w, "\n## 子进程")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| PID | NAME | CPU% | RSS | COMMAND |")
		fmt.Fprintln(w, "|---|---|---|---|---|")
		for _, item := range p.result.Children {
			fmt.Fprintf(
				w,
				"| %d | %s | %.2f | %s | %s |\n",
				item.PID,
				mdCell(displayValue(item.Name, "-")),
				item.CPUPercent,
				mdCell(formatBytes(item.RSSBytes)),
				mdCell(compact(displayValue(item.Command, "-"), 120)),
			)
		}
	}

	if len(p.result.Environment) > 0 {
		fmt.Fprintln(w, "\n## 环境变量")
		fmt.Fprintln(w)
		keys := sortedEnvKeys(p.result.Environment)
		for _, key := range keys {
			fmt.Fprintf(w, "- `%s=%s`\n", key, p.result.Environment[key])
		}
	}

	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *infoPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("进程详情结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"section", "key", "value"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	proc := p.result.Process
	rows := [][]string{
		{"process", "pid", strconv.FormatInt(int64(proc.PID), 10)},
		{"process", "ppid", strconv.FormatInt(int64(proc.PPID), 10)},
		{"process", "name", proc.Name},
		{"process", "user", proc.User},
		{"process", "cpu_percent", fmt.Sprintf("%.2f", proc.CPUPercent)},
		{"process", "cpu_seconds", fmt.Sprintf("%.2f", proc.CPUSeconds)},
		{"process", "rss_bytes", strconv.FormatUint(proc.RSSBytes, 10)},
		{"process", "vms_bytes", strconv.FormatUint(proc.VMSBytes, 10)},
		{"process", "io_read_bytes", strconv.FormatUint(proc.IOReadBytes, 10)},
		{"process", "io_write_bytes", strconv.FormatUint(proc.IOWriteBytes, 10)},
		{"process", "fd_count", strconv.FormatInt(int64(proc.FDCount), 10)},
		{"process", "thread_count", strconv.FormatInt(int64(proc.ThreadCount), 10)},
		{"process", "executable", proc.Executable},
		{"process", "command", proc.Command},
	}
	if !proc.StartTime.IsZero() {
		rows = append(rows, []string{"process", "start_time", proc.StartTime.Format(timeLayout)})
	}
	if p.result.Parent != nil {
		rows = append(rows,
			[]string{"parent", "pid", strconv.FormatInt(int64(p.result.Parent.PID), 10)},
			[]string{"parent", "name", p.result.Parent.Name},
		)
	}
	for _, child := range p.result.Children {
		rows = append(rows,
			[]string{"child", "pid", strconv.FormatInt(int64(child.PID), 10)},
			[]string{"child", "name", child.Name},
		)
	}
	if len(p.result.Environment) > 0 {
		keys := sortedEnvKeys(p.result.Environment)
		for _, key := range keys {
			rows = append(rows, []string{"env", key, p.result.Environment[key]})
		}
	}

	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *killPresenter) RenderTable(w io.Writer) error {
	if p.result == nil || p.result.Plan == nil {
		return emptyResultError("进程终止结果为空")
	}

	fmt.Fprintln(w, "进程终止计划")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "模式: %s\n", p.result.Mode)
	fmt.Fprintf(w, "根进程: %s(%d)\n", displayValue(p.result.Plan.RootName, "-"), p.result.Plan.RootPID)
	fmt.Fprintf(w, "force: %t, tree: %t, apply: %t\n", p.result.Plan.Force, p.result.Plan.Tree, p.result.Apply)

	fmt.Fprintln(w, "\n目标列表")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "%-8s %-6s %-20s %-16s %s\n", "PID", "DEPTH", "NAME", "STATUS", "MESSAGE")

	statusByPID := map[int32]proccollector.KillTargetResult{}
	if p.result.Result != nil {
		for _, item := range p.result.Result.Results {
			statusByPID[item.PID] = item
		}
	}

	for _, target := range p.result.Plan.Targets {
		status := "planned"
		message := ""
		if item, ok := statusByPID[target.PID]; ok {
			status = item.Status
			message = item.Message
		}
		fmt.Fprintf(w, "%-8d %-6d %-20s %-16s %s\n", target.PID, target.Depth, displayValue(target.Name, "-"), status, message)
	}

	if len(p.result.Plan.Warnings) > 0 {
		renderWarningsTable(w, p.result.Plan.Warnings)
	}
	if p.result.Result != nil {
		renderWarningsTable(w, p.result.Result.Warnings)
	}
	return nil
}

func (p *killPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil || p.result.Plan == nil {
		return emptyResultError("进程终止结果为空")
	}

	fmt.Fprintln(w, "# 进程终止")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- mode: `%s`\n", p.result.Mode)
	fmt.Fprintf(w, "- root: `%s(%d)`\n", p.result.Plan.RootName, p.result.Plan.RootPID)
	fmt.Fprintf(w, "- force: `%t`\n", p.result.Plan.Force)
	fmt.Fprintf(w, "- tree: `%t`\n", p.result.Plan.Tree)
	fmt.Fprintf(w, "- apply: `%t`\n", p.result.Apply)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## 目标列表")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| PID | DEPTH | NAME | STATUS | MESSAGE |")
	fmt.Fprintln(w, "|---|---|---|---|---|")

	statusByPID := map[int32]proccollector.KillTargetResult{}
	if p.result.Result != nil {
		for _, item := range p.result.Result.Results {
			statusByPID[item.PID] = item
		}
	}

	for _, target := range p.result.Plan.Targets {
		status := "planned"
		message := ""
		if item, ok := statusByPID[target.PID]; ok {
			status = item.Status
			message = item.Message
		}
		fmt.Fprintf(
			w,
			"| %d | %d | %s | %s | %s |\n",
			target.PID,
			target.Depth,
			mdCell(displayValue(target.Name, "-")),
			mdCell(status),
			mdCell(message),
		)
	}

	renderWarningsMarkdown(w, p.result.Plan.Warnings)
	if p.result.Result != nil {
		renderWarningsMarkdown(w, p.result.Result.Warnings)
	}
	return nil
}

func (p *killPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil || p.result.Plan == nil {
		return emptyResultError("进程终止结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"mode", "pid", "depth", "name", "status", "message"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	statusByPID := map[int32]proccollector.KillTargetResult{}
	if p.result.Result != nil {
		for _, item := range p.result.Result.Results {
			statusByPID[item.PID] = item
		}
	}

	for _, target := range p.result.Plan.Targets {
		status := "planned"
		message := ""
		if item, ok := statusByPID[target.PID]; ok {
			status = item.Status
			message = item.Message
		}

		if err := writer.Write([]string{
			p.result.Mode,
			strconv.FormatInt(int64(target.PID), 10),
			strconv.Itoa(target.Depth),
			target.Name,
			status,
			message,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func renderTreeTableNode(w io.Writer, node *proccollector.TreeNode, depth int, detail bool) {
	if node == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	line := fmt.Sprintf("%s- %s(%d)", indent, displayValue(node.Name, "-"), node.PID)
	if detail {
		line = fmt.Sprintf("%s user=%s cpu=%.2f rss=%s", line, displayValue(node.User, "-"), node.CPUPercent, formatBytes(node.RSSBytes))
		if node.Command != "" {
			line += " cmd=" + compact(node.Command, 80)
		}
	}
	fmt.Fprintln(w, line)
	for _, child := range node.Children {
		renderTreeTableNode(w, child, depth+1, detail)
	}
}

func renderTreeMarkdownNode(w io.Writer, node *proccollector.TreeNode, depth int, detail bool) {
	if node == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	line := fmt.Sprintf("%s- `%s(%d)`", indent, displayValue(node.Name, "-"), node.PID)
	if detail {
		line += fmt.Sprintf(" user=%s cpu=%.2f rss=%s", mdCell(displayValue(node.User, "-")), node.CPUPercent, mdCell(formatBytes(node.RSSBytes)))
	}
	fmt.Fprintln(w, line)
	for _, child := range node.Children {
		renderTreeMarkdownNode(w, child, depth+1, detail)
	}
}

func flattenTreeRows(node *proccollector.TreeNode, depth int, rows *[][]string) {
	if node == nil {
		return
	}
	*rows = append(*rows, []string{
		strconv.Itoa(depth),
		strconv.FormatInt(int64(node.PID), 10),
		strconv.FormatInt(int64(node.PPID), 10),
		node.Name,
		node.User,
		fmt.Sprintf("%.2f", node.CPUPercent),
		strconv.FormatUint(node.RSSBytes, 10),
		node.Command,
	})
	for _, child := range node.Children {
		flattenTreeRows(child, depth+1, rows)
	}
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

func sortedEnvKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatBytes(value uint64) string {
	if value > math.MaxInt64 {
		return fmt.Sprintf("%d B", value)
	}
	return utils.FormatBytes(int64(value))
}

func displayValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func compact(text string, limit int) string {
	if limit <= 0 || len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "..."
}

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

const timeLayout = "2006-01-02T15:04:05Z07:00"

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

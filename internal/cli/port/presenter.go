package port

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	portcollector "syskit/internal/collectors/port"
	"syskit/internal/errs"
)

type queryPresenter struct {
	result *portcollector.QueryResult
	detail bool
}

type listPresenter struct {
	result *portcollector.ListResult
	detail bool
}

type killPresenter struct {
	result *killOutputData
}

type pingPresenter struct {
	result *portcollector.PingResult
}

type scanPresenter struct {
	result *portcollector.ScanResult
}

func newQueryPresenter(result *portcollector.QueryResult, detail bool) *queryPresenter {
	return &queryPresenter{result: result, detail: detail}
}

func newListPresenter(result *portcollector.ListResult, detail bool) *listPresenter {
	return &listPresenter{result: result, detail: detail}
}

func newKillPresenter(result *killOutputData) *killPresenter {
	return &killPresenter{result: result}
}

func newPingPresenter(result *portcollector.PingResult) *pingPresenter {
	return &pingPresenter{result: result}
}

func newScanPresenter(result *portcollector.ScanResult) *scanPresenter {
	return &scanPresenter{result: result}
}

func (p *queryPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("端口查询结果为空")
	}

	fmt.Fprintf(w, "查询端口: %s\n", formatIntList(p.result.RequestedPorts))
	fmt.Fprintf(w, "命中端口: %s\n", formatIntList(p.result.FoundPorts))
	if len(p.result.MissingPorts) > 0 {
		fmt.Fprintf(w, "未命中端口: %s\n", formatIntList(p.result.MissingPorts))
	}
	fmt.Fprintln(w)

	renderPortEntriesTable(w, p.result.Entries, p.detail)
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *queryPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("端口查询结果为空")
	}

	fmt.Fprintln(w, "# 端口查询")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- requested_ports: `%s`\n", formatIntList(p.result.RequestedPorts))
	fmt.Fprintf(w, "- found_ports: `%s`\n", formatIntList(p.result.FoundPorts))
	if len(p.result.MissingPorts) > 0 {
		fmt.Fprintf(w, "- missing_ports: `%s`\n", formatIntList(p.result.MissingPorts))
	}
	fmt.Fprintln(w)
	renderPortEntriesMarkdown(w, p.result.Entries, p.detail)
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *queryPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("端口查询结果为空")
	}
	return writePortEntriesCSV(w, p.result.Entries)
}

func (p *listPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("端口列表结果为空")
	}

	fmt.Fprintf(w, "监听端口总数: %d\n", p.result.Total)
	fmt.Fprintf(w, "排序: %s\n", p.result.By)
	if p.result.Protocol != "" {
		fmt.Fprintf(w, "协议过滤: %s\n", p.result.Protocol)
	}
	if strings.TrimSpace(p.result.Listen) != "" {
		fmt.Fprintf(w, "监听地址过滤: %s\n", p.result.Listen)
	}
	fmt.Fprintln(w)

	renderPortEntriesTable(w, p.result.Entries, p.detail)
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *listPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("端口列表结果为空")
	}

	fmt.Fprintln(w, "# 监听端口列表")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- total: `%d`\n", p.result.Total)
	fmt.Fprintf(w, "- by: `%s`\n", p.result.By)
	if p.result.Protocol != "" {
		fmt.Fprintf(w, "- protocol: `%s`\n", p.result.Protocol)
	}
	if strings.TrimSpace(p.result.Listen) != "" {
		fmt.Fprintf(w, "- listen: `%s`\n", mdCell(p.result.Listen))
	}
	fmt.Fprintln(w)
	renderPortEntriesMarkdown(w, p.result.Entries, p.detail)
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *listPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("端口列表结果为空")
	}
	return writePortEntriesCSV(w, p.result.Entries)
}

func (p *killPresenter) RenderTable(w io.Writer) error {
	if p.result == nil || p.result.Plan == nil {
		return emptyResultError("端口释放结果为空")
	}

	fmt.Fprintf(w, "端口释放计划 (port=%d)\n", p.result.Plan.Port)
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "模式: %s\n", p.result.Mode)
	fmt.Fprintf(w, "force: %t, kill-tree: %t, apply: %t\n", p.result.Plan.Force, p.result.Plan.KillTree, p.result.Apply)

	fmt.Fprintln(w, "\n占用连接")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	renderPortEntriesTable(w, p.result.Plan.Connection, true)

	fmt.Fprintln(w, "\n目标进程")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "%-8s %-20s %-16s %s\n", "PID", "NAME", "STATUS", "MESSAGE")

	statusByPID := make(map[int32]portcollector.KillProcessResult)
	if p.result.Result != nil {
		for _, item := range p.result.Result.ProcessResult {
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
		fmt.Fprintf(w, "%-8d %-20s %-16s %s\n", target.PID, displayValue(target.ProcessName, "-"), status, message)
	}

	if p.result.Result != nil {
		fmt.Fprintf(w, "\n端口释放状态: %t\n", p.result.Result.Released)
		if len(p.result.Result.Remaining) > 0 {
			fmt.Fprintln(w, "仍占用连接:")
			renderPortEntriesTable(w, p.result.Result.Remaining, true)
		}
	}

	renderWarningsTable(w, p.result.Plan.Warnings)
	if p.result.Result != nil {
		renderWarningsTable(w, p.result.Result.Warnings)
	}
	return nil
}

func (p *killPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil || p.result.Plan == nil {
		return emptyResultError("端口释放结果为空")
	}

	fmt.Fprintln(w, "# 端口释放")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- port: `%d`\n", p.result.Plan.Port)
	fmt.Fprintf(w, "- mode: `%s`\n", p.result.Mode)
	fmt.Fprintf(w, "- force: `%t`\n", p.result.Plan.Force)
	fmt.Fprintf(w, "- kill_tree: `%t`\n", p.result.Plan.KillTree)
	fmt.Fprintf(w, "- apply: `%t`\n", p.result.Apply)
	if p.result.Result != nil {
		fmt.Fprintf(w, "- released: `%t`\n", p.result.Result.Released)
	}

	fmt.Fprintln(w, "\n## 目标进程")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| PID | NAME | STATUS | MESSAGE |")
	fmt.Fprintln(w, "|---|---|---|---|")

	statusByPID := make(map[int32]portcollector.KillProcessResult)
	if p.result.Result != nil {
		for _, item := range p.result.Result.ProcessResult {
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
			"| %d | %s | %s | %s |\n",
			target.PID,
			mdCell(displayValue(target.ProcessName, "-")),
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
		return emptyResultError("端口释放结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"mode", "port", "pid", "name", "status", "message"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	statusByPID := make(map[int32]portcollector.KillProcessResult)
	if p.result.Result != nil {
		for _, item := range p.result.Result.ProcessResult {
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
			strconv.Itoa(p.result.Plan.Port),
			strconv.FormatInt(int64(target.PID), 10),
			target.ProcessName,
			status,
			message,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *pingPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("端口可达性结果为空")
	}

	fmt.Fprintf(w, "目标: %s:%d\n", p.result.Target, p.result.Port)
	fmt.Fprintf(w, "探测次数: %d, 成功: %d, 失败: %d, 成功率: %.1f%%\n", p.result.Count, p.result.SuccessCount, p.result.FailureCount, p.result.SuccessRate)
	fmt.Fprintf(w, "timeout: %dms, interval: %dms\n", p.result.TimeoutMs, p.result.IntervalMs)
	if p.result.SuccessCount > 0 {
		fmt.Fprintf(w, "时延统计(ms): min=%.2f avg=%.2f max=%.2f\n", p.result.MinLatencyMs, p.result.AvgLatencyMs, p.result.MaxLatencyMs)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-6s %-8s %-12s %s\n", "SEQ", "SUCCESS", "LATENCY_MS", "ERROR")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	for _, item := range p.result.Attempts {
		fmt.Fprintf(w, "%-6d %-8t %-12.2f %s\n", item.Seq, item.Success, item.LatencyMs, displayValue(item.Error, "-"))
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *pingPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("端口可达性结果为空")
	}
	fmt.Fprintln(w, "# TCP 端口可达性测试")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- target: `%s`\n", mdCell(p.result.Target))
	fmt.Fprintf(w, "- port: `%d`\n", p.result.Port)
	fmt.Fprintf(w, "- count: `%d`\n", p.result.Count)
	fmt.Fprintf(w, "- success_count: `%d`\n", p.result.SuccessCount)
	fmt.Fprintf(w, "- failure_count: `%d`\n", p.result.FailureCount)
	fmt.Fprintf(w, "- success_rate: `%.1f%%`\n", p.result.SuccessRate)
	fmt.Fprintf(w, "- timeout_ms: `%d`\n", p.result.TimeoutMs)
	fmt.Fprintf(w, "- interval_ms: `%d`\n", p.result.IntervalMs)
	if p.result.SuccessCount > 0 {
		fmt.Fprintf(w, "- latency_min_ms: `%.2f`\n", p.result.MinLatencyMs)
		fmt.Fprintf(w, "- latency_avg_ms: `%.2f`\n", p.result.AvgLatencyMs)
		fmt.Fprintf(w, "- latency_max_ms: `%.2f`\n", p.result.MaxLatencyMs)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| SEQ | SUCCESS | LATENCY_MS | ERROR |")
	fmt.Fprintln(w, "|---|---|---|---|")
	for _, item := range p.result.Attempts {
		fmt.Fprintf(w, "| %d | %t | %.2f | %s |\n", item.Seq, item.Success, item.LatencyMs, mdCell(displayValue(item.Error, "-")))
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *pingPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("端口可达性结果为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"target",
		"port",
		"seq",
		"success",
		"latency_ms",
		"error",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.result.Attempts {
		if err := writer.Write([]string{
			p.result.Target,
			strconv.Itoa(p.result.Port),
			strconv.Itoa(item.Seq),
			strconv.FormatBool(item.Success),
			fmt.Sprintf("%.2f", item.LatencyMs),
			item.Error,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *scanPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("端口扫描结果为空")
	}

	fmt.Fprintf(w, "目标: %s\n", p.result.Target)
	fmt.Fprintf(w, "模式: %s, 总端口: %d, 开放: %d, 关闭: %d\n", p.result.Mode, p.result.TotalPorts, p.result.OpenCount, p.result.ClosedCount)
	fmt.Fprintf(w, "timeout: %dms\n", p.result.TimeoutMs)
	fmt.Fprintf(w, "开放端口: %s\n", formatIntList(p.result.OpenPorts))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-8s %-8s %-12s %s\n", "PORT", "OPEN", "LATENCY_MS", "ERROR")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	for _, item := range p.result.Results {
		fmt.Fprintf(w, "%-8d %-8t %-12.2f %s\n", item.Port, item.Open, item.LatencyMs, displayValue(item.Error, "-"))
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *scanPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("端口扫描结果为空")
	}

	fmt.Fprintln(w, "# 端口扫描")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- target: `%s`\n", mdCell(p.result.Target))
	fmt.Fprintf(w, "- mode: `%s`\n", p.result.Mode)
	fmt.Fprintf(w, "- total_ports: `%d`\n", p.result.TotalPorts)
	fmt.Fprintf(w, "- open_count: `%d`\n", p.result.OpenCount)
	fmt.Fprintf(w, "- closed_count: `%d`\n", p.result.ClosedCount)
	fmt.Fprintf(w, "- timeout_ms: `%d`\n", p.result.TimeoutMs)
	fmt.Fprintf(w, "- open_ports: `%s`\n", formatIntList(p.result.OpenPorts))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| PORT | OPEN | LATENCY_MS | ERROR |")
	fmt.Fprintln(w, "|---|---|---|---|")
	for _, item := range p.result.Results {
		fmt.Fprintf(w, "| %d | %t | %.2f | %s |\n", item.Port, item.Open, item.LatencyMs, mdCell(displayValue(item.Error, "-")))
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *scanPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("端口扫描结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"target",
		"mode",
		"port",
		"open",
		"latency_ms",
		"error",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.result.Results {
		if err := writer.Write([]string{
			p.result.Target,
			p.result.Mode,
			strconv.Itoa(item.Port),
			strconv.FormatBool(item.Open),
			fmt.Sprintf("%.2f", item.LatencyMs),
			item.Error,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func renderPortEntriesTable(w io.Writer, entries []portcollector.PortEntry, detail bool) {
	if len(entries) == 0 {
		fmt.Fprintln(w, "(无结果)")
		return
	}

	if detail {
		fmt.Fprintf(w, "%-6s %-8s %-22s %-8s %-8s %-20s %-18s %s\n", "PORT", "PROTO", "LOCAL", "STATUS", "PID", "PROCESS", "USER", "COMMAND")
		fmt.Fprintln(w, strings.Repeat("-", 140))
		for _, entry := range entries {
			fmt.Fprintf(
				w,
				"%-6d %-8s %-22s %-8s %-8d %-20s %-18s %s\n",
				entry.Port,
				entry.Protocol,
				entry.LocalAddr,
				entry.Status,
				entry.PID,
				compact(displayValue(entry.ProcessName, "-"), 20),
				compact(displayValue(entry.User, "-"), 18),
				compact(displayValue(entry.Command, "-"), 60),
			)
		}
		return
	}

	fmt.Fprintf(w, "%-6s %-8s %-22s %-8s %-8s %-20s\n", "PORT", "PROTO", "LOCAL", "STATUS", "PID", "PROCESS")
	fmt.Fprintln(w, strings.Repeat("-", 90))
	for _, entry := range entries {
		fmt.Fprintf(
			w,
			"%-6d %-8s %-22s %-8s %-8d %-20s\n",
			entry.Port,
			entry.Protocol,
			entry.LocalAddr,
			entry.Status,
			entry.PID,
			compact(displayValue(entry.ProcessName, "-"), 20),
		)
	}
}

func renderPortEntriesMarkdown(w io.Writer, entries []portcollector.PortEntry, detail bool) {
	if len(entries) == 0 {
		fmt.Fprintln(w, "(无结果)")
		return
	}

	if detail {
		fmt.Fprintln(w, "| PORT | PROTO | LOCAL | STATUS | PID | PROCESS | USER | COMMAND |")
		fmt.Fprintln(w, "|---|---|---|---|---|---|---|---|")
		for _, entry := range entries {
			fmt.Fprintf(
				w,
				"| %d | %s | %s | %s | %d | %s | %s | %s |\n",
				entry.Port,
				entry.Protocol,
				mdCell(entry.LocalAddr),
				mdCell(entry.Status),
				entry.PID,
				mdCell(displayValue(entry.ProcessName, "-")),
				mdCell(displayValue(entry.User, "-")),
				mdCell(compact(displayValue(entry.Command, "-"), 120)),
			)
		}
		return
	}

	fmt.Fprintln(w, "| PORT | PROTO | LOCAL | STATUS | PID | PROCESS |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|")
	for _, entry := range entries {
		fmt.Fprintf(
			w,
			"| %d | %s | %s | %s | %d | %s |\n",
			entry.Port,
			entry.Protocol,
			mdCell(entry.LocalAddr),
			mdCell(entry.Status),
			entry.PID,
			mdCell(displayValue(entry.ProcessName, "-")),
		)
	}
}

func writePortEntriesCSV(w io.Writer, entries []portcollector.PortEntry) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{
		"port",
		"protocol",
		"local_addr",
		"status",
		"pid",
		"parent_pid",
		"process_name",
		"user",
		"command",
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}

	for _, entry := range entries {
		if err := writer.Write([]string{
			strconv.Itoa(entry.Port),
			string(entry.Protocol),
			entry.LocalAddr,
			entry.Status,
			strconv.FormatInt(int64(entry.PID), 10),
			strconv.FormatInt(int64(entry.ParentPID), 10),
			entry.ProcessName,
			entry.User,
			entry.Command,
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

func formatIntList(values []int) string {
	if len(values) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
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

func displayValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

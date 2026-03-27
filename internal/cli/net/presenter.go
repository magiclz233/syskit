package net

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	netcollector "syskit/internal/collectors/net"
	"syskit/internal/errs"
)

type connPresenter struct {
	result *netcollector.ConnResult
}

type listenPresenter struct {
	result *netcollector.ListenResult
}

func newConnPresenter(result *netcollector.ConnResult) *connPresenter {
	return &connPresenter{result: result}
}

func newListenPresenter(result *netcollector.ListenResult) *listenPresenter {
	return &listenPresenter{result: result}
}

func (p *connPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("网络连接结果为空")
	}
	fmt.Fprintf(w, "连接总数: %d\n", p.result.Total)
	if p.result.PID > 0 {
		fmt.Fprintf(w, "PID 过滤: %d\n", p.result.PID)
	}
	if len(p.result.States) > 0 {
		fmt.Fprintf(w, "状态过滤: %s\n", strings.Join(p.result.States, ","))
	}
	if strings.TrimSpace(p.result.Protocol) != "" {
		fmt.Fprintf(w, "协议过滤: %s\n", p.result.Protocol)
	}
	if strings.TrimSpace(p.result.Remote) != "" {
		fmt.Fprintf(w, "远端过滤: %s\n", p.result.Remote)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-6s %-12s %-24s %-24s %-8s %s\n", "PROTO", "STATE", "LOCAL", "REMOTE", "PID", "PROCESS")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	if len(p.result.Connections) == 0 {
		fmt.Fprintln(w, "(无结果)")
	} else {
		for _, item := range p.result.Connections {
			fmt.Fprintf(
				w,
				"%-6s %-12s %-24s %-24s %-8d %s\n",
				item.Protocol,
				item.State,
				compact(displayValue(item.LocalAddr, "-"), 24),
				compact(displayValue(item.RemoteAddr, "-"), 24),
				item.PID,
				compact(displayValue(item.ProcessName, "-"), 30),
			)
		}
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *connPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("网络连接结果为空")
	}
	fmt.Fprintln(w, "# 网络连接")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- total: `%d`\n", p.result.Total)
	if p.result.PID > 0 {
		fmt.Fprintf(w, "- pid: `%d`\n", p.result.PID)
	}
	if len(p.result.States) > 0 {
		fmt.Fprintf(w, "- states: `%s`\n", strings.Join(p.result.States, ","))
	}
	if strings.TrimSpace(p.result.Protocol) != "" {
		fmt.Fprintf(w, "- protocol: `%s`\n", p.result.Protocol)
	}
	if strings.TrimSpace(p.result.Remote) != "" {
		fmt.Fprintf(w, "- remote: `%s`\n", mdCell(p.result.Remote))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| PROTO | STATE | LOCAL | REMOTE | PID | PROCESS |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|")
	for _, item := range p.result.Connections {
		fmt.Fprintf(
			w,
			"| %s | %s | %s | %s | %d | %s |\n",
			mdCell(item.Protocol),
			mdCell(item.State),
			mdCell(displayValue(item.LocalAddr, "-")),
			mdCell(displayValue(item.RemoteAddr, "-")),
			item.PID,
			mdCell(displayValue(item.ProcessName, "-")),
		)
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *connPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("网络连接结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"protocol", "state", "local_addr", "remote_addr", "pid", "process_name", "user", "command"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.result.Connections {
		if err := writer.Write([]string{
			item.Protocol,
			item.State,
			item.LocalAddr,
			item.RemoteAddr,
			strconv.FormatInt(int64(item.PID), 10),
			item.ProcessName,
			item.User,
			item.Command,
		}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func (p *listenPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("监听列表结果为空")
	}
	fmt.Fprintf(w, "监听总数: %d\n", p.result.Total)
	if strings.TrimSpace(p.result.Protocol) != "" {
		fmt.Fprintf(w, "协议过滤: %s\n", p.result.Protocol)
	}
	if strings.TrimSpace(p.result.Addr) != "" {
		fmt.Fprintf(w, "地址过滤: %s\n", p.result.Addr)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-6s %-12s %-24s %-8s %s\n", "PROTO", "STATE", "LOCAL", "PID", "PROCESS")
	fmt.Fprintln(w, strings.Repeat("-", 90))
	if len(p.result.Listen) == 0 {
		fmt.Fprintln(w, "(无结果)")
	} else {
		for _, item := range p.result.Listen {
			fmt.Fprintf(
				w,
				"%-6s %-12s %-24s %-8d %s\n",
				item.Protocol,
				item.State,
				compact(displayValue(item.LocalAddr, "-"), 24),
				item.PID,
				compact(displayValue(item.ProcessName, "-"), 30),
			)
		}
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *listenPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("监听列表结果为空")
	}
	fmt.Fprintln(w, "# 监听列表")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- total: `%d`\n", p.result.Total)
	if strings.TrimSpace(p.result.Protocol) != "" {
		fmt.Fprintf(w, "- protocol: `%s`\n", p.result.Protocol)
	}
	if strings.TrimSpace(p.result.Addr) != "" {
		fmt.Fprintf(w, "- addr: `%s`\n", mdCell(p.result.Addr))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| PROTO | STATE | LOCAL | PID | PROCESS |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	for _, item := range p.result.Listen {
		fmt.Fprintf(
			w,
			"| %s | %s | %s | %d | %s |\n",
			mdCell(item.Protocol),
			mdCell(item.State),
			mdCell(displayValue(item.LocalAddr, "-")),
			item.PID,
			mdCell(displayValue(item.ProcessName, "-")),
		)
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *listenPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("监听列表结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"protocol", "state", "local_addr", "pid", "process_name", "user", "command"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, item := range p.result.Listen {
		if err := writer.Write([]string{
			item.Protocol,
			item.State,
			item.LocalAddr,
			strconv.FormatInt(int64(item.PID), 10),
			item.ProcessName,
			item.User,
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

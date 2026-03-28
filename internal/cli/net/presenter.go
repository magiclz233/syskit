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

type speedPresenter struct {
	result *netcollector.SpeedResult
}

func newConnPresenter(result *netcollector.ConnResult) *connPresenter {
	return &connPresenter{result: result}
}

func newListenPresenter(result *netcollector.ListenResult) *listenPresenter {
	return &listenPresenter{result: result}
}

func newSpeedPresenter(result *netcollector.SpeedResult) *speedPresenter {
	return &speedPresenter{result: result}
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

func (p *speedPresenter) RenderTable(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("测速结果为空")
	}
	fmt.Fprintf(w, "测速服务: %s\n", p.result.Server)
	fmt.Fprintf(w, "模式: %s, 总耗时: %.2fms\n", p.result.Mode, p.result.DurationMs)
	if strings.TrimSpace(p.result.PublicIP) != "" {
		fmt.Fprintf(w, "公网 IP: %s\n", p.result.PublicIP)
	}
	if p.result.Trace != nil {
		if strings.TrimSpace(p.result.Trace.Operator) != "" {
			fmt.Fprintf(w, "运营商: %s\n", p.result.Trace.Operator)
		}
		if strings.TrimSpace(p.result.Trace.Location) != "" || strings.TrimSpace(p.result.Trace.Colo) != "" {
			fmt.Fprintf(w, "出口位置: loc=%s colo=%s\n", displayValue(p.result.Trace.Location, "-"), displayValue(p.result.Trace.Colo, "-"))
		}
	}
	if p.result.Ping != nil {
		fmt.Fprintf(
			w,
			"延迟(ms): min=%.2f avg=%.2f max=%.2f jitter=%.2f (loss=%.1f%%)\n",
			p.result.Ping.MinMs,
			p.result.Ping.AvgMs,
			p.result.Ping.MaxMs,
			p.result.Ping.JitterMs,
			p.result.Ping.LossRate,
		)
	}
	if p.result.Download != nil {
		fmt.Fprintf(w, "下载: %.2f Mbps (bytes=%d, duration=%.2fms)\n", p.result.Download.Mbps, p.result.Download.Bytes, p.result.Download.DurationMs)
	}
	if p.result.Upload != nil {
		fmt.Fprintf(w, "上传: %.2f Mbps (bytes=%d, duration=%.2fms)\n", p.result.Upload.Mbps, p.result.Upload.Bytes, p.result.Upload.DurationMs)
	}
	if p.result.Assessment != nil {
		fmt.Fprintln(w, "\n结论")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		fmt.Fprintln(w, displayValue(p.result.Assessment.Summary, "已完成测速"))
		for _, item := range p.result.Assessment.Highlights {
			fmt.Fprintf(w, "- %s\n", item)
		}
	}
	if len(p.result.Phases) > 0 {
		fmt.Fprintln(w, "\n阶段")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		fmt.Fprintf(w, "%-10s %-8s %-12s %-12s %s\n", "STAGE", "STATUS", "DURATION", "MBPS", "DETAIL")
		for _, item := range p.result.Phases {
			fmt.Fprintf(
				w,
				"%-10s %-8s %-12s %-12s %s\n",
				item.Name,
				item.Status,
				fmt.Sprintf("%.2fms", item.DurationMs),
				displaySpeedValue(item.Mbps),
				compact(displayValue(item.Detail, "-"), 60),
			)
		}
	}
	renderWarningsTable(w, p.result.Warnings)
	return nil
}

func (p *speedPresenter) RenderMarkdown(w io.Writer) error {
	if p.result == nil {
		return emptyResultError("测速结果为空")
	}
	fmt.Fprintln(w, "# 带宽测速")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- server: `%s`\n", mdCell(p.result.Server))
	fmt.Fprintf(w, "- mode: `%s`\n", p.result.Mode)
	fmt.Fprintf(w, "- duration_ms: `%.2f`\n", p.result.DurationMs)
	if strings.TrimSpace(p.result.PublicIP) != "" {
		fmt.Fprintf(w, "- public_ip: `%s`\n", mdCell(p.result.PublicIP))
	}
	if p.result.Trace != nil {
		if strings.TrimSpace(p.result.Trace.Operator) != "" {
			fmt.Fprintf(w, "- operator: `%s`\n", mdCell(p.result.Trace.Operator))
		}
		if strings.TrimSpace(p.result.Trace.Location) != "" {
			fmt.Fprintf(w, "- location: `%s`\n", mdCell(p.result.Trace.Location))
		}
		if strings.TrimSpace(p.result.Trace.Colo) != "" {
			fmt.Fprintf(w, "- colo: `%s`\n", mdCell(p.result.Trace.Colo))
		}
	}
	if p.result.Ping != nil {
		fmt.Fprintf(w, "- ping_avg_ms: `%.2f`\n", p.result.Ping.AvgMs)
		fmt.Fprintf(w, "- ping_loss_rate: `%.1f%%`\n", p.result.Ping.LossRate)
	}
	if p.result.Download != nil {
		fmt.Fprintf(w, "- download_mbps: `%.2f`\n", p.result.Download.Mbps)
	}
	if p.result.Upload != nil {
		fmt.Fprintf(w, "- upload_mbps: `%.2f`\n", p.result.Upload.Mbps)
	}
	if p.result.Assessment != nil {
		fmt.Fprintf(w, "- assessment: `%s`\n", mdCell(displayValue(p.result.Assessment.Summary, "已完成测速")))
	}
	if len(p.result.Phases) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "## 阶段")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| STAGE | STATUS | DURATION_MS | MBPS | DETAIL |")
		fmt.Fprintln(w, "|---|---|---|---|---|")
		for _, item := range p.result.Phases {
			fmt.Fprintf(
				w,
				"| %s | %s | %.2f | %s | %s |\n",
				mdCell(item.Name),
				mdCell(item.Status),
				item.DurationMs,
				mdCell(displaySpeedValue(item.Mbps)),
				mdCell(displayValue(item.Detail, "-")),
			)
		}
	}
	if p.result.Assessment != nil && len(p.result.Assessment.Highlights) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "## 结论")
		fmt.Fprintln(w)
		for _, item := range p.result.Assessment.Highlights {
			fmt.Fprintf(w, "- %s\n", item)
		}
	}
	renderWarningsMarkdown(w, p.result.Warnings)
	return nil
}

func (p *speedPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.result == nil {
		return emptyResultError("测速结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"server", "mode", "public_ip", "operator", "location", "colo", "ping_avg_ms", "ping_loss_rate", "download_mbps", "download_bytes", "upload_mbps", "upload_bytes", "duration_ms", "assessment_summary", "phase_summary"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	pingAvg := ""
	pingLoss := ""
	if p.result.Ping != nil {
		pingAvg = fmt.Sprintf("%.2f", p.result.Ping.AvgMs)
		pingLoss = fmt.Sprintf("%.1f", p.result.Ping.LossRate)
	}
	downMbps := ""
	downBytes := ""
	if p.result.Download != nil {
		downMbps = fmt.Sprintf("%.2f", p.result.Download.Mbps)
		downBytes = strconv.FormatInt(p.result.Download.Bytes, 10)
	}
	upMbps := ""
	upBytes := ""
	if p.result.Upload != nil {
		upMbps = fmt.Sprintf("%.2f", p.result.Upload.Mbps)
		upBytes = strconv.FormatInt(p.result.Upload.Bytes, 10)
	}
	operator := ""
	location := ""
	colo := ""
	if p.result.Trace != nil {
		operator = p.result.Trace.Operator
		location = p.result.Trace.Location
		colo = p.result.Trace.Colo
	}
	assessmentSummary := ""
	if p.result.Assessment != nil {
		assessmentSummary = p.result.Assessment.Summary
	}

	if err := writer.Write([]string{
		p.result.Server,
		p.result.Mode,
		p.result.PublicIP,
		operator,
		location,
		colo,
		pingAvg,
		pingLoss,
		downMbps,
		downBytes,
		upMbps,
		upBytes,
		fmt.Sprintf("%.2f", p.result.DurationMs),
		assessmentSummary,
		joinPhaseSummary(p.result.Phases),
	}); err != nil {
		return errs.ExecutionFailed("写入 CSV 内容失败", err)
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

func displaySpeedValue(value float64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f", value)
}

func joinPhaseSummary(phases []netcollector.SpeedPhase) string {
	if len(phases) == 0 {
		return ""
	}
	items := make([]string, 0, len(phases))
	for _, item := range phases {
		items = append(items, fmt.Sprintf("%s:%s:%.2fms", item.Name, item.Status, item.DurationMs))
	}
	return strings.Join(items, ";")
}

func emptyResultError(message string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, message)
}

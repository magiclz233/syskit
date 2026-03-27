package net

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"syskit/internal/errs"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type processInfo struct {
	name    string
	user    string
	command string
}

// CollectConnections 采集并过滤当前主机网络连接。
func CollectConnections(ctx context.Context, opts ConnOptions) (*ConnResult, error) {
	if opts.PID < 0 {
		return nil, errs.InvalidArgument("--pid 不能小于 0")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	entries, warnings, err := collectEntries(ctx)
	if err != nil {
		return nil, err
	}

	stateSet := stringSet(opts.States)
	remoteFilter := strings.ToLower(strings.TrimSpace(opts.Remote))
	filtered := make([]ConnectionEntry, 0, len(entries))
	for _, item := range entries {
		if opts.PID > 0 && item.PID != opts.PID {
			continue
		}
		if opts.Protocol != "" && item.Protocol != string(opts.Protocol) {
			continue
		}
		if len(stateSet) > 0 {
			if _, ok := stateSet[item.State]; !ok {
				continue
			}
		}
		if remoteFilter != "" && !strings.Contains(strings.ToLower(item.RemoteAddr), remoteFilter) {
			continue
		}
		filtered = append(filtered, item)
	}

	return &ConnResult{
		PID:         opts.PID,
		States:      append([]string(nil), opts.States...),
		Protocol:    string(opts.Protocol),
		Remote:      opts.Remote,
		Total:       len(filtered),
		Connections: filtered,
		Warnings:    warnings,
	}, nil
}

// CollectListen 采集监听端口列表。
func CollectListen(ctx context.Context, opts ListenOptions) (*ListenResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	entries, warnings, err := collectEntries(ctx)
	if err != nil {
		return nil, err
	}

	addrFilter := strings.ToLower(strings.TrimSpace(opts.Addr))
	filtered := make([]ConnectionEntry, 0, len(entries))
	for _, item := range entries {
		if !isListenEntry(item) {
			continue
		}
		if opts.Protocol != "" && item.Protocol != string(opts.Protocol) {
			continue
		}
		if addrFilter != "" && !strings.Contains(strings.ToLower(item.LocalAddr), addrFilter) {
			continue
		}
		filtered = append(filtered, item)
	}

	return &ListenResult{
		Protocol: string(opts.Protocol),
		Addr:     opts.Addr,
		Total:    len(filtered),
		Listen:   filtered,
		Warnings: warnings,
	}, nil
}

func collectEntries(ctx context.Context) ([]ConnectionEntry, []string, error) {
	connections, err := gnet.ConnectionsWithContext(ctx, "inet")
	if err != nil {
		return nil, nil, mapCollectionError("读取网络连接信息失败", err)
	}

	entries := make([]ConnectionEntry, 0, len(connections))
	warningSet := make(map[string]struct{})
	processCache := make(map[int32]processInfo)
	seen := make(map[string]struct{})

	for _, conn := range connections {
		if err := ctx.Err(); err != nil {
			if timeoutErr := mapTimeoutError(err); timeoutErr != nil {
				return nil, nil, timeoutErr
			}
			return nil, nil, errs.ExecutionFailed("网络连接采集被取消", err)
		}

		protocol := resolveProtocol(conn.Type)
		if protocol == "" {
			continue
		}

		entry := ConnectionEntry{
			Protocol:   protocol,
			State:      normalizeState(conn.Status),
			LocalAddr:  formatAddr(conn.Laddr),
			RemoteAddr: formatAddr(conn.Raddr),
			PID:        conn.Pid,
		}
		if entry.RemoteAddr == "*:0" {
			entry.RemoteAddr = ""
		}

		if conn.Pid > 0 {
			info := resolveProcessInfo(ctx, conn.Pid, processCache, warningSet)
			entry.ProcessName = info.name
			entry.User = info.user
			entry.Command = info.command
		}

		key := fmt.Sprintf("%s|%s|%s|%s|%d", entry.Protocol, entry.State, entry.LocalAddr, entry.RemoteAddr, entry.PID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i int, j int) bool {
		if entries[i].Protocol != entries[j].Protocol {
			return entries[i].Protocol < entries[j].Protocol
		}
		if entries[i].LocalAddr != entries[j].LocalAddr {
			return entries[i].LocalAddr < entries[j].LocalAddr
		}
		if entries[i].RemoteAddr != entries[j].RemoteAddr {
			return entries[i].RemoteAddr < entries[j].RemoteAddr
		}
		if entries[i].State != entries[j].State {
			return entries[i].State < entries[j].State
		}
		return entries[i].PID < entries[j].PID
	})

	return entries, warningSlice(warningSet), nil
}

func resolveProcessInfo(ctx context.Context, pid int32, cache map[int32]processInfo, warningSet map[string]struct{}) processInfo {
	if cached, ok := cache[pid]; ok {
		return cached
	}

	info := processInfo{}
	procRef, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		addWarning(warningSet, fmt.Sprintf("读取 PID=%d 进程信息失败: %v", pid, err))
		cache[pid] = info
		return info
	}
	if name, err := procRef.NameWithContext(ctx); err == nil {
		info.name = strings.TrimSpace(name)
	}
	if user, err := procRef.UsernameWithContext(ctx); err == nil {
		info.user = strings.TrimSpace(user)
	}
	if command, err := procRef.CmdlineWithContext(ctx); err == nil {
		info.command = strings.TrimSpace(command)
	}

	cache[pid] = info
	return info
}

func resolveProtocol(sockType uint32) string {
	switch sockType {
	case 1:
		return "tcp"
	case 2:
		return "udp"
	default:
		return ""
	}
}

func normalizeState(raw string) string {
	state := strings.ToLower(strings.TrimSpace(raw))
	if state == "" {
		return "none"
	}
	return state
}

func formatAddr(addr gnet.Addr) string {
	ip := strings.TrimSpace(addr.IP)
	if ip == "" {
		ip = "*"
	}
	return fmt.Sprintf("%s:%d", ip, addr.Port)
}

func isListenEntry(item ConnectionEntry) bool {
	if item.Protocol == "tcp" {
		return item.State == "listen" || item.State == "none"
	}
	if item.Protocol == "udp" {
		return strings.TrimSpace(item.LocalAddr) != ""
	}
	return false
}

func mapCollectionError(message string, err error) error {
	if err == nil {
		return nil
	}
	if timeoutErr := mapTimeoutError(err); timeoutErr != nil {
		return timeoutErr
	}
	if permissionErr := mapPermissionError(message, err); permissionErr != nil {
		return permissionErr
	}
	return errs.ExecutionFailed(message, err)
}

func mapTimeoutError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "命令执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
	}
	return nil
}

func mapPermissionError(message string, err error) error {
	if err == nil {
		return nil
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "permission denied") ||
		strings.Contains(text, "access denied") ||
		strings.Contains(text, "operation not permitted") {
		return errs.PermissionDenied(message, "请提升权限后重试")
	}
	return nil
}

func warningSlice(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	items := make([]string, 0, len(set))
	for item := range set {
		items = append(items, item)
	}
	slices.Sort(items)
	return items
}

func addWarning(set map[string]struct{}, message string) {
	if set == nil {
		return
	}
	set[message] = struct{}{}
}

func stringSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	return set
}

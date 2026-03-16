// Package mem 提供内存总览和进程内存排行采集能力。
package mem

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"syskit/internal/errs"

	gmem "github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

// SortBy 表示 `mem top` 排序维度。
type SortBy string

const (
	// SortByRSS 按 RSS 排序。
	SortByRSS SortBy = "rss"
	// SortByVMS 按 VMS 排序。
	SortByVMS SortBy = "vms"
	// SortBySwap 按 swap 排序。
	SortBySwap SortBy = "swap"
)

const defaultTopN = 5

// TopOptions 是 `mem top` 的输入参数。
type TopOptions struct {
	By   SortBy
	TopN int
	User string
	Name string
}

// ProcessEntry 表示单个进程的内存信息。
type ProcessEntry struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	User       string  `json:"user,omitempty"`
	Command    string  `json:"command,omitempty"`
	RSSBytes   uint64  `json:"rss_bytes"`
	VMSBytes   uint64  `json:"vms_bytes"`
	SwapBytes  uint64  `json:"swap_bytes"`
	MemPercent float32 `json:"mem_percent"`
}

// TopResult 是 `mem top` 的结构化输出。
type TopResult struct {
	By           SortBy         `json:"by"`
	TopN         int            `json:"top_n"`
	TotalMatched int            `json:"total_matched"`
	Processes    []ProcessEntry `json:"processes"`
	Warnings     []string       `json:"warnings,omitempty"`
}

// Overview 是 `mem` 命令总览输出。
type Overview struct {
	TotalBytes       uint64         `json:"total_bytes"`
	AvailableBytes   uint64         `json:"available_bytes"`
	UsedBytes        uint64         `json:"used_bytes"`
	FreeBytes        uint64         `json:"free_bytes"`
	UsagePercent     float64        `json:"usage_percent"`
	SwapTotalBytes   uint64         `json:"swap_total_bytes"`
	SwapUsedBytes    uint64         `json:"swap_used_bytes"`
	SwapFreeBytes    uint64         `json:"swap_free_bytes"`
	SwapUsagePercent float64        `json:"swap_usage_percent"`
	CachedBytes      uint64         `json:"cached_bytes,omitempty"`
	BuffersBytes     uint64         `json:"buffers_bytes,omitempty"`
	TopProcesses     []ProcessEntry `json:"top_processes"`
	Warnings         []string       `json:"warnings,omitempty"`
}

// ParseSortBy 解析 `--by` 参数。
func ParseSortBy(raw string) (SortBy, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return SortByRSS, nil
	}

	switch SortBy(normalized) {
	case SortByRSS, SortByVMS, SortBySwap:
		return SortBy(normalized), nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--by 仅支持 rss/vms/swap，当前为: %s", raw))
	}
}

// CollectOverview 采集系统内存总览，并附带高内存进程概览。
func CollectOverview(ctx context.Context, detail bool, topN int) (*Overview, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if topN <= 0 {
		topN = defaultTopN
	}

	virtual, err := gmem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, mapCollectionError("读取内存总览失败", err)
	}

	swap, swapErr := gmem.SwapMemoryWithContext(ctx)
	warnings := make([]string, 0, 2)
	if swapErr != nil {
		warnings = append(warnings, "读取 swap 信息失败，已按 0 处理")
	}

	overview := &Overview{
		TotalBytes:       virtual.Total,
		AvailableBytes:   virtual.Available,
		UsedBytes:        virtual.Used,
		FreeBytes:        virtual.Free,
		UsagePercent:     virtual.UsedPercent,
		CachedBytes:      virtual.Cached,
		BuffersBytes:     virtual.Buffers,
		SwapTotalBytes:   swap.Total,
		SwapUsedBytes:    swap.Used,
		SwapFreeBytes:    swap.Free,
		SwapUsagePercent: swap.UsedPercent,
		Warnings:         warnings,
		TopProcesses:     make([]ProcessEntry, 0, topN),
	}

	top, topErr := CollectTop(ctx, TopOptions{
		By:   SortByRSS,
		TopN: topN,
	})
	if topErr != nil {
		overview.Warnings = appendUnique(overview.Warnings, fmt.Sprintf("读取高内存进程失败: %s", errs.Message(topErr)))
		return overview, nil
	}

	overview.TopProcesses = top.Processes
	overview.Warnings = appendUnique(overview.Warnings, top.Warnings...)
	if !detail {
		overview.CachedBytes = 0
		overview.BuffersBytes = 0
	}
	return overview, nil
}

// CollectTop 采集并返回进程内存排行。
func CollectTop(ctx context.Context, opts TopOptions) (*TopResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.TopN <= 0 {
		return nil, errs.InvalidArgument("--top 必须大于 0")
	}
	if opts.By == "" {
		opts.By = SortByRSS
	}

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, mapCollectionError("读取进程列表失败", err)
	}

	warningSet := make(map[string]struct{})
	entries := make([]ProcessEntry, 0, len(procs))

	for _, procRef := range procs {
		if err := ctx.Err(); err != nil {
			return nil, mapCollectionError("读取进程信息失败", err)
		}

		entry, ok := collectProcessEntry(ctx, procRef)
		if !ok {
			continue
		}
		if !matchFilter(entry.User, opts.User) {
			continue
		}
		if opts.Name != "" && !matchesName(entry, opts.Name) {
			continue
		}
		entries = append(entries, entry)
	}

	sortEntries(entries, opts.By)
	limited := entries
	if len(limited) > opts.TopN {
		limited = limited[:opts.TopN]
	}
	if opts.By == SortBySwap && len(entries) > 0 {
		allZero := true
		for _, item := range entries {
			if item.SwapBytes > 0 {
				allZero = false
				break
			}
		}
		if allZero {
			addWarning(warningSet, "swap", "当前环境进程 swap 指标不可用或均为 0")
		}
	}

	return &TopResult{
		By:           opts.By,
		TopN:         opts.TopN,
		TotalMatched: len(entries),
		Processes:    limited,
		Warnings:     warningSlice(warningSet),
	}, nil
}

func collectProcessEntry(ctx context.Context, procRef *process.Process) (ProcessEntry, bool) {
	entry := ProcessEntry{
		PID: procRef.Pid,
	}

	if name, err := procRef.NameWithContext(ctx); err == nil {
		entry.Name = strings.TrimSpace(name)
	}
	if user, err := procRef.UsernameWithContext(ctx); err == nil {
		entry.User = strings.TrimSpace(user)
	}
	if command, err := procRef.CmdlineWithContext(ctx); err == nil {
		entry.Command = strings.TrimSpace(command)
	}
	if memPercent, err := procRef.MemoryPercentWithContext(ctx); err == nil {
		entry.MemPercent = memPercent
	}

	memInfo, memErr := procRef.MemoryInfoWithContext(ctx)
	if memErr != nil || memInfo == nil {
		return ProcessEntry{}, false
	}
	entry.RSSBytes = memInfo.RSS
	entry.VMSBytes = memInfo.VMS
	entry.SwapBytes = memInfo.Swap
	return entry, true
}

func sortEntries(entries []ProcessEntry, by SortBy) {
	sort.Slice(entries, func(i int, j int) bool {
		left := entries[i]
		right := entries[j]
		switch by {
		case SortByVMS:
			if left.VMSBytes != right.VMSBytes {
				return left.VMSBytes > right.VMSBytes
			}
		case SortBySwap:
			if left.SwapBytes != right.SwapBytes {
				return left.SwapBytes > right.SwapBytes
			}
		default:
			if left.RSSBytes != right.RSSBytes {
				return left.RSSBytes > right.RSSBytes
			}
		}
		return left.PID < right.PID
	})
}

func matchesName(entry ProcessEntry, filter string) bool {
	return matchFilter(entry.Name, filter) || matchFilter(entry.Command, filter)
}

func matchFilter(value string, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), filter)
}

func mapCollectionError(message string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "命令执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "permission denied") || strings.Contains(text, "access denied") || strings.Contains(text, "operation not permitted") {
		return errs.PermissionDenied(message, "请提升权限后重试")
	}
	return errs.ExecutionFailed(message, err)
}

func appendUnique(items []string, values ...string) []string {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		exists := false
		for _, item := range items {
			if item == value {
				exists = true
				break
			}
		}
		if !exists {
			items = append(items, value)
		}
	}
	return items
}

func addWarning(set map[string]struct{}, key string, message string) {
	if set == nil {
		return
	}
	set[key+": "+message] = struct{}{}
}

func warningSlice(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	items := make([]string, 0, len(set))
	for key := range set {
		items = append(items, key)
	}
	slices.Sort(items)
	return items
}

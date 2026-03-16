package proc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"syskit/internal/errs"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

const defaultTreeMaxDepth = 3

type snapshotOptions struct {
	withCommand bool
	withCPU     bool
	withIO      bool
	withFD      bool
}

// CollectTop 采集进程并按指定维度排序，供 `proc top` 使用。
func CollectTop(ctx context.Context, opts TopOptions) (*TopResult, error) {
	if opts.TopN <= 0 {
		return nil, errs.InvalidArgument("--top 必须大于 0")
	}
	if opts.By == "" {
		opts.By = SortByCPU
	}

	snapshots, warnings, err := collectSnapshots(ctx, snapshotOptions{
		withCommand: true,
		withCPU:     true,
		withIO:      opts.By == SortByIO,
		withFD:      opts.By == SortByFD,
	})
	if err != nil {
		return nil, err
	}

	filtered := make([]ProcessSnapshot, 0, len(snapshots))
	for _, item := range snapshots {
		if !matchFilter(item.User, opts.User) {
			continue
		}
		if opts.Name != "" && !matchesName(item, opts.Name) {
			continue
		}
		filtered = append(filtered, item)
	}

	sortTopSnapshots(filtered, opts.By)

	limited := filtered
	if len(limited) > opts.TopN {
		limited = limited[:opts.TopN]
	}

	if opts.Watch {
		warnings = appendUnique(warnings, "--watch 当前为单次采样输出，持续监控将在后续迭代增强")
	}

	return &TopResult{
		By:           opts.By,
		TopN:         opts.TopN,
		Watch:        opts.Watch,
		TotalMatched: len(filtered),
		Processes:    limited,
		Warnings:     warnings,
	}, nil
}

// CollectTree 构建进程树，供 `proc tree` 使用。
func CollectTree(ctx context.Context, opts TreeOptions) (*TreeResult, error) {
	snapshots, warnings, err := collectSnapshots(ctx, snapshotOptions{
		withCommand: opts.Detail,
		withCPU:     opts.Detail,
	})
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return &TreeResult{
			RootPID:  opts.RootPID,
			Detail:   opts.Detail,
			Full:     opts.Full,
			Nodes:    nil,
			Warnings: warnings,
		}, nil
	}

	snapshotByPID := make(map[int32]ProcessSnapshot, len(snapshots))
	childrenByPID := make(map[int32][]int32, len(snapshots))
	for _, item := range snapshots {
		snapshotByPID[item.PID] = item
		childrenByPID[item.PPID] = append(childrenByPID[item.PPID], item.PID)
	}
	for parent, children := range childrenByPID {
		sort.Slice(children, func(i int, j int) bool {
			return children[i] < children[j]
		})
		childrenByPID[parent] = children
	}

	roots, err := selectRoots(opts.RootPID, snapshotByPID)
	if err != nil {
		return nil, err
	}

	maxDepth := defaultTreeMaxDepth
	if opts.Full {
		maxDepth = -1
	}

	result := &TreeResult{
		RootPID:  opts.RootPID,
		Detail:   opts.Detail,
		Full:     opts.Full,
		Nodes:    make([]*TreeNode, 0, len(roots)),
		Warnings: warnings,
	}

	for _, root := range roots {
		path := map[int32]bool{root: true}
		node, truncated := buildTreeNode(root, 0, maxDepth, opts.Detail, childrenByPID, snapshotByPID, path)
		if node != nil {
			result.Nodes = append(result.Nodes, node)
		}
		if truncated {
			result.Truncated = true
		}
	}

	return result, nil
}

// CollectInfo 采集单进程详情，供 `proc info` 使用。
func CollectInfo(ctx context.Context, pid int32, includeEnv bool) (*InfoResult, error) {
	proc, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return nil, mapProcessLookupError(pid, err)
	}

	warningSet := make(map[string]struct{})
	self, err := snapshotFromProcess(ctx, proc, snapshotOptions{
		withCommand: true,
		withCPU:     true,
		withIO:      true,
		withFD:      true,
	}, warningSet)
	if err != nil {
		return nil, err
	}

	result := &InfoResult{
		Process:  self,
		Warnings: warningSlice(warningSet),
	}

	if ppid := self.PPID; ppid > 0 {
		parentProc, parentErr := process.NewProcessWithContext(ctx, ppid)
		if parentErr == nil {
			parent, snapErr := snapshotFromProcess(ctx, parentProc, snapshotOptions{
				withCommand: true,
				withCPU:     true,
			}, warningSet)
			if snapErr == nil {
				result.Parent = &parent
			}
		}
	}

	children, childErr := proc.ChildrenWithContext(ctx)
	if childErr == nil {
		result.Children = make([]ProcessSnapshot, 0, len(children))
		for _, child := range children {
			childItem, snapErr := snapshotFromProcess(ctx, child, snapshotOptions{
				withCommand: true,
				withCPU:     true,
			}, warningSet)
			if snapErr == nil {
				result.Children = append(result.Children, childItem)
			}
		}
		sort.Slice(result.Children, func(i int, j int) bool {
			return result.Children[i].PID < result.Children[j].PID
		})
	}

	if includeEnv {
		envList, envErr := proc.EnvironWithContext(ctx)
		if envErr != nil {
			addWarning(warningSet, "env", fmt.Sprintf("读取环境变量失败: %v", envErr))
		} else {
			result.Environment = parseEnvMap(envList)
		}
	}

	result.Warnings = warningSlice(warningSet)
	return result, nil
}

// BuildProcessMap 返回当前进程快照映射，供 `proc kill` 规划阶段复用。
func BuildProcessMap(ctx context.Context) (map[int32]ProcessSnapshot, map[int32][]int32, []string, error) {
	snapshots, warnings, err := collectSnapshots(ctx, snapshotOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	snapshotByPID := make(map[int32]ProcessSnapshot, len(snapshots))
	childrenByPID := make(map[int32][]int32, len(snapshots))
	for _, item := range snapshots {
		snapshotByPID[item.PID] = item
		childrenByPID[item.PPID] = append(childrenByPID[item.PPID], item.PID)
	}
	for parent, children := range childrenByPID {
		sort.Slice(children, func(i int, j int) bool {
			return children[i] < children[j]
		})
		childrenByPID[parent] = children
	}

	return snapshotByPID, childrenByPID, warnings, nil
}

func collectSnapshots(ctx context.Context, opts snapshotOptions) ([]ProcessSnapshot, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, nil, mapContextOrExecutionError("读取进程列表失败", err)
	}

	result := make([]ProcessSnapshot, 0, len(procs))
	warningSet := make(map[string]struct{})

	for _, item := range procs {
		if err := ctx.Err(); err != nil {
			return nil, nil, timeoutError(err)
		}
		snapshot, snapErr := snapshotFromProcess(ctx, item, opts, warningSet)
		if snapErr != nil {
			// 单个进程读取失败不阻断整体，保证只读命令的可用性。
			continue
		}
		result = append(result, snapshot)
	}

	return result, warningSlice(warningSet), nil
}

func snapshotFromProcess(ctx context.Context, proc *process.Process, opts snapshotOptions, warningSet map[string]struct{}) (ProcessSnapshot, error) {
	item := ProcessSnapshot{PID: proc.Pid}

	if ppid, err := proc.PpidWithContext(ctx); err == nil {
		item.PPID = ppid
	}
	if name, err := proc.NameWithContext(ctx); err == nil {
		item.Name = name
	}
	if user, err := proc.UsernameWithContext(ctx); err == nil {
		item.User = strings.TrimSpace(user)
	}
	if opts.withCommand {
		if command, err := proc.CmdlineWithContext(ctx); err == nil {
			item.Command = strings.TrimSpace(command)
		}
		if exe, err := proc.ExeWithContext(ctx); err == nil {
			item.Executable = strings.TrimSpace(exe)
		}
	}
	if startedAt, err := proc.CreateTimeWithContext(ctx); err == nil && startedAt > 0 {
		item.StartTime = time.UnixMilli(startedAt).UTC()
	}

	if opts.withCPU {
		if cpuPercent, err := proc.CPUPercentWithContext(ctx); err == nil {
			item.CPUPercent = cpuPercent
		}
		if cpuTimes, err := proc.TimesWithContext(ctx); err == nil && cpuTimes != nil {
			item.CPUSeconds = cpuTimes.User + cpuTimes.System
		}
	}

	if memInfo, err := proc.MemoryInfoWithContext(ctx); err == nil && memInfo != nil {
		item.RSSBytes = memInfo.RSS
		item.VMSBytes = memInfo.VMS
	}
	if threadCount, err := proc.NumThreadsWithContext(ctx); err == nil {
		item.ThreadCount = threadCount
	}
	if opts.withIO {
		ioCounters, ioErr := proc.IOCountersWithContext(ctx)
		if ioErr == nil && ioCounters != nil {
			item.IOReadBytes = ioCounters.ReadBytes
			item.IOWriteBytes = ioCounters.WriteBytes
		} else if ioErr != nil {
			maybeAddUnsupportedWarning(warningSet, "io", "io", ioErr)
		}
	}
	if opts.withFD {
		fdCount, fdErr := proc.NumFDsWithContext(ctx)
		if fdErr == nil {
			item.FDCount = fdCount
		} else {
			maybeAddUnsupportedWarning(warningSet, "fd", "fd", fdErr)
		}
	}

	return item, nil
}

func sortTopSnapshots(items []ProcessSnapshot, by SortBy) {
	sort.Slice(items, func(i int, j int) bool {
		left := items[i]
		right := items[j]
		switch by {
		case SortByMem:
			if left.RSSBytes != right.RSSBytes {
				return left.RSSBytes > right.RSSBytes
			}
		case SortByIO:
			if left.IOBytes() != right.IOBytes() {
				return left.IOBytes() > right.IOBytes()
			}
		case SortByFD:
			if left.FDCount != right.FDCount {
				return left.FDCount > right.FDCount
			}
		default:
			if left.CPUPercent != right.CPUPercent {
				return left.CPUPercent > right.CPUPercent
			}
		}
		return left.PID < right.PID
	})
}

func selectRoots(rootPID *int32, snapshotByPID map[int32]ProcessSnapshot) ([]int32, error) {
	if rootPID != nil {
		if _, ok := snapshotByPID[*rootPID]; !ok {
			return nil, errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, fmt.Sprintf("未找到 PID=%d 的进程", *rootPID))
		}
		return []int32{*rootPID}, nil
	}

	roots := make([]int32, 0, len(snapshotByPID))
	for pid, item := range snapshotByPID {
		if item.PPID == item.PID {
			roots = append(roots, pid)
			continue
		}
		if _, ok := snapshotByPID[item.PPID]; !ok {
			roots = append(roots, pid)
		}
	}
	sort.Slice(roots, func(i int, j int) bool {
		return roots[i] < roots[j]
	})
	return roots, nil
}

func buildTreeNode(
	pid int32,
	depth int,
	maxDepth int,
	detail bool,
	childrenByPID map[int32][]int32,
	snapshotByPID map[int32]ProcessSnapshot,
	path map[int32]bool,
) (*TreeNode, bool) {
	item, ok := snapshotByPID[pid]
	if !ok {
		return nil, false
	}

	node := &TreeNode{
		PID:  item.PID,
		PPID: item.PPID,
		Name: item.Name,
	}
	if detail {
		node.User = item.User
		node.Command = item.Command
		node.CPUPercent = item.CPUPercent
		node.RSSBytes = item.RSSBytes
	}

	if maxDepth >= 0 && depth >= maxDepth {
		return node, len(childrenByPID[pid]) > 0
	}

	children := childrenByPID[pid]
	if len(children) == 0 {
		return node, false
	}

	node.Children = make([]*TreeNode, 0, len(children))
	truncated := false
	for _, childPID := range children {
		if path[childPID] {
			truncated = true
			continue
		}
		childPath := clonePath(path)
		childPath[childPID] = true
		childNode, childTruncated := buildTreeNode(childPID, depth+1, maxDepth, detail, childrenByPID, snapshotByPID, childPath)
		if childNode != nil {
			node.Children = append(node.Children, childNode)
		}
		if childTruncated {
			truncated = true
		}
	}

	return node, truncated
}

func clonePath(path map[int32]bool) map[int32]bool {
	next := make(map[int32]bool, len(path)+1)
	for key, value := range path {
		next[key] = value
	}
	return next
}

func parseEnvMap(list []string) map[string]string {
	env := make(map[string]string, len(list))
	for _, item := range list {
		if strings.TrimSpace(item) == "" {
			continue
		}
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		// 重复 key 保留首次值，避免后置覆盖引入不确定性。
		if _, exists := env[key]; !exists {
			env[key] = value
		}
	}
	return env
}

func matchesName(item ProcessSnapshot, filter string) bool {
	if matchFilter(item.Name, filter) {
		return true
	}
	return matchFilter(item.Command, filter)
}

func matchFilter(value string, filter string) bool {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), filter)
}

func mapProcessLookupError(pid int32, err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "not found") || strings.Contains(message, "not exist") {
		return errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, fmt.Sprintf("未找到 PID=%d 的进程", pid))
	}
	return mapContextOrExecutionError(fmt.Sprintf("读取 PID=%d 进程信息失败", pid), err)
}

func mapContextOrExecutionError(message string, err error) error {
	if err == nil {
		return nil
	}
	if timeout := timeoutError(err); timeout != nil {
		return timeout
	}
	if permission := permissionError(message, err); permission != nil {
		return permission
	}
	return errs.ExecutionFailed(message, err)
}

func timeoutError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "命令执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
	}
	return nil
}

func permissionError(message string, err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "permission denied") || strings.Contains(lower, "access denied") || strings.Contains(lower, "operation not permitted") {
		return errs.PermissionDenied(message, "请提升权限后重试")
	}
	return nil
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

func addWarning(set map[string]struct{}, key string, message string) {
	if set == nil {
		return
	}
	set[key+": "+message] = struct{}{}
}

func maybeAddUnsupportedWarning(set map[string]struct{}, key string, field string, err error) {
	if err == nil {
		return
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "not implement") || strings.Contains(lower, "not supported") {
		addWarning(set, key, fmt.Sprintf("%s 指标当前平台不支持，已按 0 处理", field))
		return
	}
	if strings.Contains(lower, "permission denied") || strings.Contains(lower, "access denied") || strings.Contains(lower, "operation not permitted") {
		addWarning(set, key, fmt.Sprintf("%s 指标读取权限不足，已按 0 处理", field))
	}
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

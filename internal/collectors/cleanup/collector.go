// Package cleanup 提供 `fix cleanup` 的清理计划、执行和校验能力。
package cleanup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syskit/internal/errs"
	"time"
)

// Target 表示清理目标类别。
type Target string

const (
	// TargetTemp 表示临时文件清理。
	TargetTemp Target = "temp"
	// TargetLogs 表示日志文件清理。
	TargetLogs Target = "logs"
	// TargetCache 表示缓存文件清理。
	TargetCache Target = "cache"
)

var allTargets = []Target{TargetTemp, TargetLogs, TargetCache}

// PlanOptions 是构建清理计划时的输入参数。
type PlanOptions struct {
	Targets        []Target
	OlderThan      time.Duration
	StorageDataDir string
	LoggingFile    string
	Now            time.Time
}

// Candidate 是一个待清理文件条目。
type Candidate struct {
	Target    Target    `json:"target"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	ModTime   time.Time `json:"mod_time"`
	AgeSec    int64     `json:"age_sec"`
}

// Plan 表示 dry-run 阶段输出的清理计划。
type Plan struct {
	Targets        []Target    `json:"targets"`
	OlderThanSec   int64       `json:"older_than_sec"`
	ScanRoots      []string    `json:"scan_roots"`
	ScannedFiles   int         `json:"scanned_files"`
	CandidateCount int         `json:"candidate_count"`
	CandidateBytes int64       `json:"candidate_bytes"`
	Candidates     []Candidate `json:"candidates"`
	Warnings       []string    `json:"warnings,omitempty"`
}

// FailedItem 表示执行阶段删除失败的文件。
type FailedItem struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Message   string `json:"message"`
}

// ApplyResult 表示 apply 阶段执行和校验结果。
type ApplyResult struct {
	Plan           *Plan        `json:"plan"`
	Applied        bool         `json:"applied"`
	DeletedCount   int          `json:"deleted_count"`
	DeletedBytes   int64        `json:"deleted_bytes"`
	Failed         []FailedItem `json:"failed,omitempty"`
	RemainingCount int          `json:"remaining_count"`
	RemainingBytes int64        `json:"remaining_bytes"`
	Warnings       []string     `json:"warnings,omitempty"`
}

// ParseTargets 解析 `--target` 参数。
func ParseTargets(raw string) ([]Target, error) {
	if strings.TrimSpace(raw) == "" {
		return append([]Target(nil), allTargets...), nil
	}

	parts := strings.Split(raw, ",")
	set := make(map[Target]struct{}, len(parts))
	for _, part := range parts {
		name := Target(strings.ToLower(strings.TrimSpace(part)))
		switch name {
		case TargetTemp, TargetLogs, TargetCache:
			set[name] = struct{}{}
		default:
			return nil, errs.InvalidArgument(fmt.Sprintf("--target 仅支持 temp/logs/cache，当前为: %s", part))
		}
	}

	if len(set) == 0 {
		return nil, errs.InvalidArgument("--target 不能为空")
	}

	targets := make([]Target, 0, len(set))
	for _, target := range allTargets {
		if _, ok := set[target]; ok {
			targets = append(targets, target)
		}
	}
	return targets, nil
}

// ParseOlderThan 解析 `--older-than` 参数，支持 Go duration 以及 d/w 后缀。
func ParseOlderThan(raw string) (time.Duration, error) {
	text := strings.TrimSpace(strings.ToLower(raw))
	if text == "" {
		return 7 * 24 * time.Hour, nil
	}
	if duration, err := time.ParseDuration(text); err == nil {
		if duration <= 0 {
			return 0, errs.InvalidArgument("--older-than 必须大于 0")
		}
		return duration, nil
	}

	if strings.HasSuffix(text, "d") || strings.HasSuffix(text, "w") {
		unit := text[len(text)-1]
		valuePart := strings.TrimSpace(text[:len(text)-1])
		if valuePart == "" {
			return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --older-than: %s", raw))
		}
		value, err := parsePositiveInt(valuePart)
		if err != nil {
			return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --older-than: %s", raw))
		}
		switch unit {
		case 'd':
			return time.Duration(value) * 24 * time.Hour, nil
		case 'w':
			return time.Duration(value) * 7 * 24 * time.Hour, nil
		}
	}

	return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --older-than: %s（示例: 72h, 7d, 2w）", raw))
}

// BuildPlan 扫描目标目录并生成清理计划（dry-run）。
func BuildPlan(ctx context.Context, opts PlanOptions) (*Plan, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(opts.Targets) == 0 {
		opts.Targets = append([]Target(nil), allTargets...)
	}
	if opts.OlderThan <= 0 {
		return nil, errs.InvalidArgument("older-than 必须大于 0")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	rootsByTarget := buildRootsByTarget(opts.StorageDataDir, opts.LoggingFile)
	plan := &Plan{
		Targets:      append([]Target(nil), opts.Targets...),
		OlderThanSec: int64(opts.OlderThan.Seconds()),
		Candidates:   make([]Candidate, 0, 128),
		Warnings:     make([]string, 0, 8),
	}
	warningSet := make(map[string]struct{})

	for _, target := range opts.Targets {
		roots := rootsByTarget[target]
		if len(roots) == 0 {
			addWarning(warningSet, fmt.Sprintf("目标 %s 没有可扫描路径", target))
			continue
		}
		for _, root := range roots {
			plan.ScanRoots = append(plan.ScanRoots, root)
			stat, err := os.Stat(root)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					addWarning(warningSet, fmt.Sprintf("路径不存在，已跳过: %s", root))
					continue
				}
				if permission := permissionError("读取清理路径失败", err); permission != nil {
					return nil, permission
				}
				return nil, errs.ExecutionFailed("读取清理路径失败", err)
			}
			if !stat.IsDir() {
				addWarning(warningSet, fmt.Sprintf("路径不是目录，已跳过: %s", root))
				continue
			}

			err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					// 单路径失败不阻断整体，记录告警继续扫描。
					addWarning(warningSet, fmt.Sprintf("扫描失败，已跳过: %s (%v)", path, walkErr))
					return nil
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if d.Type()&os.ModeSymlink != 0 {
					return nil
				}

				info, err := d.Info()
				if err != nil {
					addWarning(warningSet, fmt.Sprintf("读取文件信息失败，已跳过: %s (%v)", path, err))
					return nil
				}
				plan.ScannedFiles++

				age := now.Sub(info.ModTime())
				if age < opts.OlderThan {
					return nil
				}

				candidate := Candidate{
					Target:    target,
					Path:      path,
					SizeBytes: info.Size(),
					ModTime:   info.ModTime().UTC(),
					AgeSec:    int64(age.Seconds()),
				}
				plan.Candidates = append(plan.Candidates, candidate)
				plan.CandidateBytes += info.Size()
				return nil
			})
			if err != nil {
				if timeout := timeoutError(err); timeout != nil {
					return nil, timeout
				}
				return nil, errs.ExecutionFailed("扫描清理路径失败", err)
			}
		}
	}

	plan.CandidateCount = len(plan.Candidates)
	sort.Slice(plan.Candidates, func(i int, j int) bool {
		if plan.Candidates[i].SizeBytes != plan.Candidates[j].SizeBytes {
			return plan.Candidates[i].SizeBytes > plan.Candidates[j].SizeBytes
		}
		return plan.Candidates[i].Path < plan.Candidates[j].Path
	})
	plan.Warnings = warningSlice(warningSet)
	return plan, nil
}

// ApplyPlan 执行清理计划并做基础校验。
func ApplyPlan(ctx context.Context, plan *Plan) (*ApplyResult, error) {
	if plan == nil {
		return nil, errs.InvalidArgument("清理计划不能为空")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	result := &ApplyResult{
		Plan:    plan,
		Applied: true,
		Failed:  make([]FailedItem, 0, 8),
	}
	warningSet := make(map[string]struct{})

	for _, item := range plan.Candidates {
		if err := ctx.Err(); err != nil {
			return nil, timeoutError(err)
		}

		if err := os.Remove(item.Path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// 文件已不存在，视为已清理。
				result.DeletedCount++
				result.DeletedBytes += item.SizeBytes
				continue
			}
			if permission := permissionError("删除文件失败", err); permission != nil {
				return nil, permission
			}
			result.Failed = append(result.Failed, FailedItem{
				Path:      item.Path,
				SizeBytes: item.SizeBytes,
				Message:   err.Error(),
			})
			continue
		}

		result.DeletedCount++
		result.DeletedBytes += item.SizeBytes
	}

	// verify: 校验计划中文件是否仍存在，给出 remaining 结果。
	for _, item := range plan.Candidates {
		if err := ctx.Err(); err != nil {
			return nil, timeoutError(err)
		}
		stat, err := os.Stat(item.Path)
		if err == nil {
			result.RemainingCount++
			result.RemainingBytes += stat.Size()
			addWarning(warningSet, fmt.Sprintf("文件仍存在: %s", item.Path))
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		addWarning(warningSet, fmt.Sprintf("校验失败: %s (%v)", item.Path, err))
	}

	if len(result.Failed) > 0 {
		addWarning(warningSet, fmt.Sprintf("共有 %d 个文件删除失败", len(result.Failed)))
	}
	if result.RemainingCount > 0 {
		addWarning(warningSet, fmt.Sprintf("共有 %d 个文件仍未清理", result.RemainingCount))
	}

	result.Warnings = warningSlice(warningSet)
	return result, nil
}

func buildRootsByTarget(storageDataDir string, loggingFile string) map[Target][]string {
	dataDir := strings.TrimSpace(storageDataDir)
	dataRoot := strings.TrimSpace(filepath.Dir(dataDir))

	roots := map[Target][]string{
		TargetTemp: {
			filepath.Join(os.TempDir(), "syskit"),
			filepath.Join(dataRoot, "temp"),
			filepath.Join(dataDir, "temp"),
		},
		TargetLogs: {
			filepath.Dir(strings.TrimSpace(loggingFile)),
			filepath.Join(dataRoot, "logs"),
			filepath.Join(dataDir, "logs"),
		},
		TargetCache: {
			filepath.Join(dataRoot, "cache"),
			filepath.Join(dataDir, "cache"),
		},
	}

	for target, items := range roots {
		roots[target] = dedupePaths(items)
	}
	return roots
}

func dedupePaths(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || item == "." {
			continue
		}
		clean := filepath.Clean(item)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	slices.Sort(result)
	return result
}

func parsePositiveInt(raw string) (int, error) {
	value := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, errs.InvalidArgument("无效数字")
		}
		value = value*10 + int(ch-'0')
	}
	if value <= 0 {
		return 0, errs.InvalidArgument("数值必须大于 0")
	}
	return value, nil
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
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "permission denied") || strings.Contains(text, "access denied") || strings.Contains(text, "operation not permitted") {
		return errs.PermissionDenied(message, "请提升权限后重试")
	}
	return nil
}

func addWarning(set map[string]struct{}, message string) {
	if set == nil {
		return
	}
	set[message] = struct{}{}
}

func warningSlice(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	items := make([]string, 0, len(set))
	for item := range set {
		items = append(items, item)
	}
	sort.Strings(items)
	return items
}

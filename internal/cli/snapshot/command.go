// Package snapshot 负责快照相关命令组。
package snapshot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"syskit/internal/audit"
	"syskit/internal/cliutil"
	cpucollector "syskit/internal/collectors/cpu"
	diskcollector "syskit/internal/collectors/disk"
	memcollector "syskit/internal/collectors/mem"
	portcollector "syskit/internal/collectors/port"
	proccollector "syskit/internal/collectors/proc"
	"syskit/internal/config"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"syskit/internal/output"
	"syskit/internal/storage"
	"time"

	"github.com/spf13/cobra"
)

var snapshotSupportedModules = []string{"port", "cpu", "mem", "disk", "proc"}

var snapshotModuleSet = map[string]struct{}{
	"port": {},
	"cpu":  {},
	"mem":  {},
	"disk": {},
	"proc": {},
}

type createOptions struct {
	name        string
	description string
	module      string
}

type listOptions struct {
	limit int
}

type showOptions struct {
	module string
}

type diffOptions struct {
	onlyChange bool
	module     string
}

type deleteOptions struct{}

// createOutputData 表示 `snapshot create` 输出数据。
type createOutputData struct {
	Snapshot       *model.SnapshotSummary `json:"snapshot"`
	Warnings       []string               `json:"warnings,omitempty"`
	SkippedModules []string               `json:"skipped_modules,omitempty"`
}

// listOutputData 表示 `snapshot list` 输出数据。
type listOutputData struct {
	Snapshots []model.SnapshotSummary `json:"snapshots"`
	Count     int                     `json:"count"`
	Limit     int                     `json:"limit"`
}

// showOutputData 表示 `snapshot show` 输出数据。
type showOutputData struct {
	Snapshot        *model.Snapshot `json:"snapshot"`
	SelectedModules []string        `json:"selected_modules,omitempty"`
	MissingModules  []string        `json:"missing_modules,omitempty"`
}

// moduleDiff 表示单模块差异。
type moduleDiff struct {
	Module     string `json:"module"`
	ChangeType string `json:"change_type"`
	Risk       string `json:"risk,omitempty"`
	Before     any    `json:"before,omitempty"`
	After      any    `json:"after,omitempty"`
}

// snapshotDiffResult 表示快照对比结果。
type snapshotDiffResult struct {
	ComparedModules []string     `json:"compared_modules"`
	Added           []moduleDiff `json:"added,omitempty"`
	Removed         []moduleDiff `json:"removed,omitempty"`
	Changed         []moduleDiff `json:"changed,omitempty"`
	Unchanged       []string     `json:"unchanged,omitempty"`
	HasChanges      bool         `json:"has_changes"`
}

// diffOutputData 表示 `snapshot diff` 输出数据。
type diffOutputData struct {
	BaseID             string              `json:"base_id"`
	TargetID           string              `json:"target_id"`
	SelectedModules    []string            `json:"selected_modules,omitempty"`
	OnlyChange         bool                `json:"only_change"`
	AutoSelectedTarget string              `json:"auto_selected_target,omitempty"`
	Diff               *snapshotDiffResult `json:"diff"`
}

// deleteOutputData 表示 `snapshot delete` 输出数据。
type deleteOutputData struct {
	Mode     string                 `json:"mode"`
	Apply    bool                   `json:"apply"`
	Deleted  bool                   `json:"deleted"`
	Snapshot *model.SnapshotSummary `json:"snapshot"`
}

// NewCommand 创建 `snapshot` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "快照管理命令",
		Long: "snapshot 用于创建、查看、对比和删除系统快照，支撑报告生成和状态回溯。" +
			"\n\n删除快照属于写操作，默认仍会先以 dry-run 方式输出计划。",
		Example: "  syskit snapshot create --module port,cpu\n" +
			"  syskit snapshot list --limit 10\n" +
			"  syskit snapshot diff snap-a snap-b",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newCreateCommand(),
		newListCommand(),
		newShowCommand(),
		newDiffCommand(),
		newDeleteCommand(),
	)

	return cmd
}

func newCreateCommand() *cobra.Command {
	opts := &createOptions{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "创建系统快照",
		Long:  "snapshot create 用于采集当前系统指定模块的数据并落盘为可追踪快照。",
		Example: "  syskit snapshot create\n" +
			"  syskit snapshot create --name nightly --module port,cpu\n" +
			"  syskit snapshot create --description \"发布前基线\"",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.name, "name", "", "快照名称")
	flags.StringVar(&opts.description, "description", "", "快照描述")
	flags.StringVar(&opts.module, "module", "", "指定快照模块（逗号分隔）：port,cpu,mem,disk,proc")
	return cmd
}

func newListCommand() *cobra.Command {
	opts := &listOptions{limit: 20}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出已保存快照",
		Long:  "snapshot list 用于查看本地已保存的快照摘要。",
		Example: "  syskit snapshot list\n" +
			"  syskit snapshot list --limit 5\n" +
			"  syskit snapshot list --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}
	cmd.Flags().IntVar(&opts.limit, "limit", 20, "返回最近 N 条快照")
	return cmd
}

func newShowCommand() *cobra.Command {
	opts := &showOptions{}
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "查看快照详情",
		Long:  "snapshot show 用于查看单个快照的完整内容，也可按模块过滤。",
		Example: "  syskit snapshot show snap-a\n" +
			"  syskit snapshot show snap-a --module port,cpu\n" +
			"  syskit snapshot show snap-a --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(cmd, args[0], opts)
		},
	}
	cmd.Flags().StringVar(&opts.module, "module", "", "仅展示指定模块（逗号分隔）")
	return cmd
}

func newDiffCommand() *cobra.Command {
	opts := &diffOptions{}
	cmd := &cobra.Command{
		Use:   "diff <idA> [idB]",
		Short: "比较两个快照",
		Long:  "snapshot diff 用于比较两个快照的模块差异；只传一个 ID 时会自动选择最近的另一个快照。",
		Example: "  syskit snapshot diff snap-a snap-b\n" +
			"  syskit snapshot diff snap-a --only-change\n" +
			"  syskit snapshot diff snap-a snap-b --module port,cpu",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd, args, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.onlyChange, "only-change", false, "仅展示变化项")
	flags.StringVar(&opts.module, "module", "", "仅对比指定模块（逗号分隔）")
	return cmd
}

func newDeleteCommand() *cobra.Command {
	opts := &deleteOptions{}
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "删除指定快照",
		Long:  "snapshot delete 默认只输出删除计划；真实删除必须显式传入 `--apply --yes`，并会写入审计日志。",
		Example: "  syskit snapshot delete snap-a\n" +
			"  syskit snapshot delete snap-a --apply --yes\n" +
			"  syskit snapshot delete snap-a --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args[0], opts)
		},
	}
	return cmd
}

func runCreate(cmd *cobra.Command, opts *createOptions) error {
	startedAt := time.Now()
	modules, err := parseSnapshotModules(opts.module, true)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return err
	}
	store, err := storage.NewSnapshotStore(cfg.Storage.DataDir)
	if err != nil {
		return err
	}

	moduleData, warnings, skipped := collectSnapshotData(ctx, modules)
	now := time.Now().UTC()
	id := newSnapshotID(now)
	name := strings.TrimSpace(opts.name)
	if name == "" {
		name = "snapshot-" + id
	}

	host, _ := os.Hostname()
	snapshot := &model.Snapshot{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(opts.description),
		CreatedAt:   now,
		Host:        host,
		Platform:    runtime.GOOS,
		Modules:     moduleData,
		Warnings:    warnings,
	}

	summary, err := store.Save(ctx, snapshot)
	if err != nil {
		return err
	}

	data := &createOutputData{
		Snapshot:       summary,
		Warnings:       warnings,
		SkippedModules: skipped,
	}
	msg := fmt.Sprintf("快照创建完成（id=%s）", summary.ID)
	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newCreatePresenter(data))
}

func runList(cmd *cobra.Command, opts *listOptions) error {
	if opts.limit <= 0 {
		return errs.InvalidArgument("--limit 必须大于 0")
	}
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return err
	}
	store, err := storage.NewSnapshotStore(cfg.Storage.DataDir)
	if err != nil {
		return err
	}

	items, err := store.List(ctx, opts.limit)
	if err != nil {
		return err
	}
	data := &listOutputData{
		Snapshots: items,
		Count:     len(items),
		Limit:     opts.limit,
	}
	result := output.NewSuccessResult(fmt.Sprintf("快照列表查询完成（%d 条）", len(items)), data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newListPresenter(data))
}

func runShow(cmd *cobra.Command, id string, opts *showOptions) error {
	startedAt := time.Now()
	selectedModules, err := parseSnapshotModules(opts.module, false)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return err
	}
	store, err := storage.NewSnapshotStore(cfg.Storage.DataDir)
	if err != nil {
		return err
	}

	snapshot, err := store.Load(ctx, id)
	if err != nil {
		return err
	}
	view, missing := filterSnapshotModules(snapshot, selectedModules)

	data := &showOutputData{
		Snapshot:        view,
		SelectedModules: selectedModules,
		MissingModules:  missing,
	}
	result := output.NewSuccessResult("快照详情查询完成", data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newShowPresenter(data))
}

func runDiff(cmd *cobra.Command, args []string, opts *diffOptions) error {
	startedAt := time.Now()
	selectedModules, err := parseSnapshotModules(opts.module, false)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return err
	}
	store, err := storage.NewSnapshotStore(cfg.Storage.DataDir)
	if err != nil {
		return err
	}

	baseSnapshot, err := store.Load(ctx, args[0])
	if err != nil {
		return err
	}

	targetID := ""
	autoSelected := ""
	if len(args) == 2 {
		targetID = args[1]
	} else {
		items, listErr := store.List(ctx, 0)
		if listErr != nil {
			return listErr
		}
		for _, item := range items {
			if item.ID != baseSnapshot.ID {
				targetID = item.ID
				autoSelected = item.ID
				break
			}
		}
		if targetID == "" {
			return errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, "缺少可对比快照，请补充 <idB> 或先创建更多快照")
		}
	}

	if strings.EqualFold(baseSnapshot.ID, targetID) {
		return errs.InvalidArgument("snapshot diff 的两个快照 ID 不能相同")
	}
	targetSnapshot, err := store.Load(ctx, targetID)
	if err != nil {
		return err
	}

	diff := buildSnapshotDiff(baseSnapshot, targetSnapshot, selectedModules, opts.onlyChange)
	data := &diffOutputData{
		BaseID:             baseSnapshot.ID,
		TargetID:           targetSnapshot.ID,
		SelectedModules:    selectedModules,
		OnlyChange:         opts.onlyChange,
		AutoSelectedTarget: autoSelected,
		Diff:               diff,
	}

	result := output.NewSuccessResult("快照对比完成", data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newDiffPresenter(data))
}

func runDelete(cmd *cobra.Command, id string, _ *deleteOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return err
	}
	store, err := storage.NewSnapshotStore(cfg.Storage.DataDir)
	if err != nil {
		return err
	}

	summary, err := store.GetSummary(ctx, id)
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	yes := cliutil.ResolveBoolFlag(cmd, "yes")
	if apply && !yes {
		return errs.InvalidArgument("真实执行 snapshot delete 需要同时传入 --yes")
	}

	if !apply {
		data := &deleteOutputData{
			Mode:     "dry-run",
			Apply:    false,
			Deleted:  false,
			Snapshot: summary,
		}
		result := output.NewSuccessResult("已生成快照删除计划（dry-run）", data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newDeletePresenter(data))
	}

	deleted, err := store.Delete(ctx, id)
	if err != nil {
		auditErr := writeAuditEvent(cmd, ctx, audit.Event{
			Command:    cmd.CommandPath(),
			Action:     "snapshot.delete",
			Target:     fmt.Sprintf("snapshot:%s", id),
			Before:     summary,
			Result:     "failed",
			ErrorMsg:   errs.Message(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Metadata: map[string]any{
				"apply": true,
			},
		})
		if auditErr != nil {
			return errs.ExecutionFailed(
				fmt.Sprintf("snapshot delete 执行失败且审计写入失败: %s", errs.Message(err)),
				auditErr,
			)
		}
		return err
	}
	data := &deleteOutputData{
		Mode:     "apply",
		Apply:    true,
		Deleted:  true,
		Snapshot: deleted,
	}
	if err := writeAuditEvent(cmd, ctx, audit.Event{
		Command:    cmd.CommandPath(),
		Action:     "snapshot.delete",
		Target:     fmt.Sprintf("snapshot:%s", deleted.ID),
		Before:     summary,
		After:      deleted,
		Result:     "success",
		DurationMs: time.Since(startedAt).Milliseconds(),
		Metadata: map[string]any{
			"apply": true,
		},
	}); err != nil {
		return err
	}
	result := output.NewSuccessResult(fmt.Sprintf("快照删除完成（id=%s）", deleted.ID), data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newDeletePresenter(data))
}

func collectSnapshotData(ctx context.Context, modules []string) (map[string]any, []string, []string) {
	data := make(map[string]any, len(modules))
	warnings := make([]string, 0, 8)
	skipped := make([]string, 0, len(modules))

	for _, module := range modules {
		switch module {
		case "port":
			result, err := portcollector.ListPorts(ctx, portcollector.ListOptions{By: "port"}, true)
			if err != nil {
				skipped = append(skipped, module)
				warnings = append(warnings, fmt.Sprintf("%s 模块采集失败: %s", module, errs.Message(err)))
				continue
			}
			data[module] = result
			warnings = appendPrefixedWarnings(warnings, module, result.Warnings)
		case "cpu":
			result, err := cpucollector.CollectOverview(ctx, cpucollector.CollectOptions{Detail: true, TopN: 20})
			if err != nil {
				skipped = append(skipped, module)
				warnings = append(warnings, fmt.Sprintf("%s 模块采集失败: %s", module, errs.Message(err)))
				continue
			}
			data[module] = result
			warnings = appendPrefixedWarnings(warnings, module, result.Warnings)
		case "mem":
			result, err := memcollector.CollectOverview(ctx, true, 20)
			if err != nil {
				skipped = append(skipped, module)
				warnings = append(warnings, fmt.Sprintf("%s 模块采集失败: %s", module, errs.Message(err)))
				continue
			}
			data[module] = result
			warnings = appendPrefixedWarnings(warnings, module, result.Warnings)
		case "disk":
			result, err := diskcollector.CollectOverview()
			if err != nil {
				skipped = append(skipped, module)
				warnings = append(warnings, fmt.Sprintf("%s 模块采集失败: %s", module, errs.Message(errs.ExecutionFailed("采集磁盘总览失败", err))))
				continue
			}
			data[module] = result
			warnings = appendPrefixedWarnings(warnings, module, result.Warnings)
		case "proc":
			result, err := proccollector.CollectTop(ctx, proccollector.TopOptions{
				By:   proccollector.SortByCPU,
				TopN: 50,
			})
			if err != nil {
				skipped = append(skipped, module)
				warnings = append(warnings, fmt.Sprintf("%s 模块采集失败: %s", module, errs.Message(err)))
				continue
			}
			data[module] = result
			warnings = appendPrefixedWarnings(warnings, module, result.Warnings)
		}
	}

	return data, dedupeStrings(warnings), dedupeStrings(skipped)
}

func appendPrefixedWarnings(dst []string, module string, values []string) []string {
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		dst = append(dst, fmt.Sprintf("%s: %s", module, item))
	}
	return dst
}

func parseSnapshotModules(raw string, defaultAll bool) ([]string, error) {
	items := cliutil.SplitCSV(raw)
	if len(items) == 0 {
		if defaultAll {
			return append([]string(nil), snapshotSupportedModules...), nil
		}
		return nil, nil
	}

	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if _, ok := snapshotModuleSet[normalized]; !ok {
			return nil, errs.InvalidArgument(fmt.Sprintf("--module 包含不支持模块: %s", item))
		}
		set[normalized] = struct{}{}
	}

	result := make([]string, 0, len(set))
	for _, module := range snapshotSupportedModules {
		if _, ok := set[module]; ok {
			result = append(result, module)
		}
	}
	return result, nil
}

func filterSnapshotModules(snapshot *model.Snapshot, selected []string) (*model.Snapshot, []string) {
	if snapshot == nil {
		return nil, nil
	}
	copySnapshot := *snapshot
	copySnapshot.Modules = make(map[string]any, len(snapshot.Modules))

	if len(selected) == 0 {
		for key, value := range snapshot.Modules {
			copySnapshot.Modules[key] = value
		}
		return &copySnapshot, nil
	}

	missing := make([]string, 0, len(selected))
	for _, module := range selected {
		if value, ok := snapshot.Modules[module]; ok {
			copySnapshot.Modules[module] = value
			continue
		}
		missing = append(missing, module)
	}
	return &copySnapshot, missing
}

func buildSnapshotDiff(base *model.Snapshot, target *model.Snapshot, selectedModules []string, onlyChange bool) *snapshotDiffResult {
	modules := comparedModules(base, target, selectedModules)
	result := &snapshotDiffResult{
		ComparedModules: modules,
		Added:           make([]moduleDiff, 0),
		Removed:         make([]moduleDiff, 0),
		Changed:         make([]moduleDiff, 0),
		Unchanged:       make([]string, 0),
	}

	for _, module := range modules {
		before, hasBefore := base.Modules[module]
		after, hasAfter := target.Modules[module]

		switch {
		case !hasBefore && hasAfter:
			result.Added = append(result.Added, moduleDiff{
				Module:     module,
				ChangeType: "added",
				Risk:       moduleRisk(module),
				After:      after,
			})
		case hasBefore && !hasAfter:
			result.Removed = append(result.Removed, moduleDiff{
				Module:     module,
				ChangeType: "removed",
				Risk:       moduleRisk(module),
				Before:     before,
			})
		case !reflect.DeepEqual(before, after):
			result.Changed = append(result.Changed, moduleDiff{
				Module:     module,
				ChangeType: "changed",
				Risk:       moduleRisk(module),
				Before:     before,
				After:      after,
			})
		default:
			if !onlyChange {
				result.Unchanged = append(result.Unchanged, module)
			}
		}
	}

	result.HasChanges = len(result.Added)+len(result.Removed)+len(result.Changed) > 0
	return result
}

func comparedModules(base *model.Snapshot, target *model.Snapshot, selectedModules []string) []string {
	if len(selectedModules) > 0 {
		return append([]string(nil), selectedModules...)
	}

	set := make(map[string]struct{}, len(base.Modules)+len(target.Modules))
	for module := range base.Modules {
		set[module] = struct{}{}
	}
	for module := range target.Modules {
		set[module] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for module := range set {
		result = append(result, module)
	}
	sort.Strings(result)
	return result
}

func moduleRisk(module string) string {
	switch module {
	case "port", "proc":
		return "high"
	default:
		return "medium"
	}
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func newSnapshotID(now time.Time) string {
	now = now.UTC()
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return strings.ToLower(now.Format("20060102t150405"))
	}
	return strings.ToLower(now.Format("20060102t150405") + "-" + hex.EncodeToString(random))
}

func loadRuntimeConfig(cmd *cobra.Command) (*config.Config, error) {
	loadResult, err := config.Load(config.LoadOptions{
		ExplicitPath: strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "config")),
	})
	if err != nil {
		return nil, err
	}
	return loadResult.Config, nil
}

func writeAuditEvent(cmd *cobra.Command, ctx context.Context, event audit.Event) error {
	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return err
	}
	logger, err := audit.NewLogger(cfg.Storage.DataDir)
	if err != nil {
		return err
	}
	return logger.Log(ctx, event)
}

// Package filecmd 负责文件治理命令组。
package filecmd

import (
	"context"
	"fmt"
	"strings"
	"syskit/internal/audit"
	"syskit/internal/cliutil"
	filecollector "syskit/internal/collectors/file"
	"syskit/internal/config"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type dupOptions struct {
	minSize string
	exclude string
	hash    string
}

type archiveOptions struct {
	olderThan   string
	archivePath string
	compress    string
	retention   string
	exclude     string
}

type emptyOptions struct {
	exclude string
}

type dedupOptions struct {
	minSize string
	exclude string
	hash    string
}

type archiveOutputData struct {
	Mode   string                       `json:"mode"`
	Apply  bool                         `json:"apply"`
	Plan   *filecollector.ArchivePlan   `json:"plan"`
	Result *filecollector.ArchiveResult `json:"result,omitempty"`
}

type emptyOutputData struct {
	Mode   string                     `json:"mode"`
	Apply  bool                       `json:"apply"`
	Plan   *filecollector.EmptyPlan   `json:"plan"`
	Result *filecollector.EmptyResult `json:"result,omitempty"`
}

type dedupOutputData struct {
	Mode   string                     `json:"mode"`
	Apply  bool                       `json:"apply"`
	Plan   *filecollector.DedupPlan   `json:"plan"`
	Result *filecollector.DedupResult `json:"result,omitempty"`
}

// NewCommand 创建 `file` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "文件治理命令",
		Long: "file 提供重复文件检测、归档、空目录清理和去重能力。" +
			"\n\narchive/empty/dedup 默认仅输出 dry-run 计划，真实执行会写入审计日志。",
		Example: "  syskit file dup ./logs --min-size 1MB\n" +
			"  syskit file archive ./logs --older-than 7d\n" +
			"  syskit file empty ./workspace\n" +
			"  syskit file dedup ./data --apply --yes",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newDupCommand(),
		newArchiveCommand(),
		newEmptyCommand(),
		newDedupCommand(),
	)
	return cmd
}

func newDupCommand() *cobra.Command {
	opts := &dupOptions{
		minSize: "1B",
		hash:    "sha256",
	}
	cmd := &cobra.Command{
		Use:   "dup <path>",
		Short: "检测重复文件",
		Long:  "file dup 会扫描指定目录并按 hash 分组输出重复文件清单。",
		Example: "  syskit file dup ./data\n" +
			"  syskit file dup ./data --min-size 1MB --hash sha256\n" +
			"  syskit file dup ./data --exclude node_modules,.git --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDup(cmd, args[0], opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.minSize, "min-size", "1B", "最小文件大小（支持 B/KB/MB/GB）")
	flags.StringVar(&opts.exclude, "exclude", "", "排除路径关键字（逗号分隔）")
	flags.StringVar(&opts.hash, "hash", "sha256", "hash 算法: md5/sha256")
	return cmd
}

func newArchiveCommand() *cobra.Command {
	opts := &archiveOptions{
		olderThan: "7d",
		compress:  "gzip",
		retention: "30d",
	}
	cmd := &cobra.Command{
		Use:   "archive <path>",
		Short: "归档旧文件",
		Long: "file archive 会扫描历史文件并生成归档计划，默认仅 dry-run。" +
			"\n\n真实执行请显式传入 --apply；执行结果会写入审计日志。",
		Example: "  syskit file archive ./logs --older-than 7d\n" +
			"  syskit file archive ./logs --archive-path ./backup --compress zip --apply",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArchive(cmd, args[0], opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.olderThan, "older-than", "7d", "仅归档超过该时长的文件")
	flags.StringVar(&opts.archivePath, "archive-path", "", "归档目录（默认 <path>/.archive）")
	flags.StringVar(&opts.compress, "compress", "gzip", "压缩方式: gzip/zip")
	flags.StringVar(&opts.retention, "retention", "30d", "归档保留时长（示例: 30d）")
	flags.StringVar(&opts.exclude, "exclude", "", "排除路径关键字（逗号分隔）")
	return cmd
}

func newEmptyCommand() *cobra.Command {
	opts := &emptyOptions{}
	cmd := &cobra.Command{
		Use:   "empty <path>",
		Short: "清理空目录",
		Long: "file empty 会扫描空目录并输出清理计划，默认仅 dry-run。" +
			"\n\n真实执行必须显式传入 `--apply --yes`，执行后会写入审计日志。",
		Example: "  syskit file empty ./workspace\n" +
			"  syskit file empty ./workspace --apply --yes",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEmpty(cmd, args[0], opts)
		},
	}
	cmd.Flags().StringVar(&opts.exclude, "exclude", "", "排除路径关键字（逗号分隔）")
	return cmd
}

func newDedupCommand() *cobra.Command {
	opts := &dedupOptions{
		minSize: "1B",
		hash:    "sha256",
	}
	cmd := &cobra.Command{
		Use:   "dedup <path>",
		Short: "清理重复文件",
		Long: "file dedup 会先生成重复文件清理计划，默认仅 dry-run。" +
			"\n\n真实执行必须显式传入 `--apply --yes`，执行后会写入审计日志。",
		Example: "  syskit file dedup ./data\n" +
			"  syskit file dedup ./data --min-size 1MB --apply --yes",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDedup(cmd, args[0], opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.minSize, "min-size", "1B", "最小文件大小（支持 B/KB/MB/GB）")
	flags.StringVar(&opts.exclude, "exclude", "", "排除路径关键字（逗号分隔）")
	flags.StringVar(&opts.hash, "hash", "sha256", "hash 算法: md5/sha256")
	return cmd
}

func runDup(cmd *cobra.Command, path string, opts *dupOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	minSize, err := parseSize(opts.minSize)
	if err != nil {
		return err
	}
	hashMode, err := filecollector.ParseHashMode(opts.hash)
	if err != nil {
		return err
	}
	resultData, err := filecollector.FindDuplicates(ctx, filecollector.DupOptions{
		Path:    path,
		MinSize: minSize,
		Exclude: splitCSV(opts.exclude),
		Hash:    hashMode,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("重复文件检测完成（groups=%d duplicates=%d）", resultData.GroupCount, resultData.DuplicateCount)
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newDupPresenter(resultData))
}

func runArchive(cmd *cobra.Command, path string, opts *archiveOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	olderThan, err := parseSince(opts.olderThan, "--older-than")
	if err != nil {
		return err
	}
	retention, err := parseSince(opts.retention, "--retention")
	if err != nil {
		return err
	}
	plan, err := filecollector.BuildArchivePlan(ctx, filecollector.ArchiveOptions{
		Path:        path,
		OlderThan:   olderThan,
		ArchivePath: opts.archivePath,
		Compress:    opts.compress,
		Retention:   retention,
		Exclude:     splitCSV(opts.exclude),
	})
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	if !apply {
		data := &archiveOutputData{
			Mode:  "dry-run",
			Apply: false,
			Plan:  plan,
		}
		result := output.NewSuccessResult("归档计划已生成（dry-run）", data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newArchivePresenter(data))
	}

	execResult, err := filecollector.ExecuteArchivePlan(ctx, plan)
	if err != nil {
		auditResult := auditResultFromError(err)
		if auditErr := writeAuditEvent(cmd, ctx, audit.Event{
			Command:    cmd.CommandPath(),
			Action:     "file.archive",
			Target:     path,
			Before:     plan,
			Result:     auditResult,
			ErrorMsg:   errs.Message(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Metadata: map[string]any{
				"apply":      true,
				"error_code": errs.ErrorCode(err),
			},
		}); auditErr != nil {
			return errs.ExecutionFailed("file archive 执行失败且审计写入失败", auditErr)
		}
		return err
	}

	data := &archiveOutputData{
		Mode:   "apply",
		Apply:  true,
		Plan:   plan,
		Result: execResult,
	}
	auditResult := auditResultFromFailures(execResult.Failed)
	if err := writeAuditEvent(cmd, ctx, audit.Event{
		Command:    cmd.CommandPath(),
		Action:     "file.archive",
		Target:     path,
		Before:     plan,
		After:      execResult,
		Result:     auditResult,
		DurationMs: time.Since(startedAt).Milliseconds(),
		Metadata: map[string]any{
			"apply":        true,
			"failed_count": len(execResult.Failed),
		},
	}); err != nil {
		return err
	}

	msg := fmt.Sprintf("文件归档完成（archived=%d）", execResult.ArchivedCount)
	if len(execResult.Failed) > 0 {
		msg = fmt.Sprintf("文件归档完成（archived=%d failed=%d）", execResult.ArchivedCount, len(execResult.Failed))
	}
	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newArchivePresenter(data))
}

func runEmpty(cmd *cobra.Command, path string, opts *emptyOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	plan, err := filecollector.BuildEmptyPlan(ctx, path, splitCSV(opts.exclude))
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	yes := cliutil.ResolveBoolFlag(cmd, "yes")
	if apply && !yes {
		return errs.InvalidArgument("真实执行 file empty 需要同时传入 --yes")
	}
	if !apply {
		data := &emptyOutputData{
			Mode:  "dry-run",
			Apply: false,
			Plan:  plan,
		}
		result := output.NewSuccessResult("空目录清理计划已生成（dry-run）", data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newEmptyPresenter(data))
	}

	execResult, err := filecollector.ExecuteEmptyPlan(ctx, plan)
	if err != nil {
		auditResult := auditResultFromError(err)
		if auditErr := writeAuditEvent(cmd, ctx, audit.Event{
			Command:    cmd.CommandPath(),
			Action:     "file.empty",
			Target:     path,
			Before:     plan,
			Result:     auditResult,
			ErrorMsg:   errs.Message(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Metadata: map[string]any{
				"apply":      true,
				"error_code": errs.ErrorCode(err),
			},
		}); auditErr != nil {
			return errs.ExecutionFailed("file empty 执行失败且审计写入失败", auditErr)
		}
		return err
	}

	data := &emptyOutputData{
		Mode:   "apply",
		Apply:  true,
		Plan:   plan,
		Result: execResult,
	}
	auditResult := auditResultFromFailures(execResult.Failed)
	if err := writeAuditEvent(cmd, ctx, audit.Event{
		Command:    cmd.CommandPath(),
		Action:     "file.empty",
		Target:     path,
		Before:     plan,
		After:      execResult,
		Result:     auditResult,
		DurationMs: time.Since(startedAt).Milliseconds(),
		Metadata: map[string]any{
			"apply":        true,
			"failed_count": len(execResult.Failed),
		},
	}); err != nil {
		return err
	}

	msg := fmt.Sprintf("空目录清理完成（deleted=%d）", execResult.DeletedDirs)
	if len(execResult.Failed) > 0 {
		msg = fmt.Sprintf("空目录清理完成（deleted=%d failed=%d）", execResult.DeletedDirs, len(execResult.Failed))
	}
	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newEmptyPresenter(data))
}

func runDedup(cmd *cobra.Command, path string, opts *dedupOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	minSize, err := parseSize(opts.minSize)
	if err != nil {
		return err
	}
	hashMode, err := filecollector.ParseHashMode(opts.hash)
	if err != nil {
		return err
	}
	plan, err := filecollector.BuildDedupPlan(ctx, filecollector.DupOptions{
		Path:    path,
		MinSize: minSize,
		Exclude: splitCSV(opts.exclude),
		Hash:    hashMode,
	})
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	yes := cliutil.ResolveBoolFlag(cmd, "yes")
	if apply && !yes {
		return errs.InvalidArgument("真实执行 file dedup 需要同时传入 --yes")
	}
	if !apply {
		data := &dedupOutputData{
			Mode:  "dry-run",
			Apply: false,
			Plan:  plan,
		}
		result := output.NewSuccessResult("重复文件清理计划已生成（dry-run）", data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newDedupPresenter(data))
	}

	execResult, err := filecollector.ExecuteDedupPlan(ctx, plan)
	if err != nil {
		auditResult := auditResultFromError(err)
		if auditErr := writeAuditEvent(cmd, ctx, audit.Event{
			Command:    cmd.CommandPath(),
			Action:     "file.dedup",
			Target:     path,
			Before:     plan,
			Result:     auditResult,
			ErrorMsg:   errs.Message(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Metadata: map[string]any{
				"apply":      true,
				"error_code": errs.ErrorCode(err),
			},
		}); auditErr != nil {
			return errs.ExecutionFailed("file dedup 执行失败且审计写入失败", auditErr)
		}
		return err
	}

	data := &dedupOutputData{
		Mode:   "apply",
		Apply:  true,
		Plan:   plan,
		Result: execResult,
	}
	auditResult := auditResultFromFailures(execResult.Failed)
	if err := writeAuditEvent(cmd, ctx, audit.Event{
		Command:    cmd.CommandPath(),
		Action:     "file.dedup",
		Target:     path,
		Before:     plan,
		After:      execResult,
		Result:     auditResult,
		DurationMs: time.Since(startedAt).Milliseconds(),
		Metadata: map[string]any{
			"apply":        true,
			"failed_count": len(execResult.Failed),
		},
	}); err != nil {
		return err
	}

	msg := fmt.Sprintf("重复文件清理完成（deleted=%d）", execResult.DeletedFiles)
	if len(execResult.Failed) > 0 {
		msg = fmt.Sprintf("重复文件清理完成（deleted=%d failed=%d）", execResult.DeletedFiles, len(execResult.Failed))
	}
	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newDedupPresenter(data))
}

func splitCSV(raw string) []string {
	return cliutil.SplitCSV(raw)
}

func parseSince(raw string, flagName string) (time.Duration, error) {
	text := strings.ToLower(strings.TrimSpace(raw))
	if text == "" {
		return 0, nil
	}
	if duration, err := time.ParseDuration(text); err == nil {
		if duration <= 0 {
			return 0, errs.InvalidArgument(flagName + " 必须大于 0")
		}
		return duration, nil
	}
	if strings.HasSuffix(text, "d") || strings.HasSuffix(text, "w") {
		unit := text[len(text)-1]
		value := strings.TrimSpace(text[:len(text)-1])
		n, err := parsePositiveInt(value)
		if err != nil {
			return 0, errs.InvalidArgument(fmt.Sprintf("无效的 %s: %s", flagName, raw))
		}
		if unit == 'd' {
			return time.Duration(n) * 24 * time.Hour, nil
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return 0, errs.InvalidArgument(fmt.Sprintf("无效的 %s: %s", flagName, raw))
}

func parseSize(raw string) (int64, error) {
	text := strings.ToUpper(strings.TrimSpace(raw))
	if text == "" {
		return 0, errs.InvalidArgument("--min-size 不能为空")
	}
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(text, "KB"):
		multiplier = 1024
		text = strings.TrimSuffix(text, "KB")
	case strings.HasSuffix(text, "MB"):
		multiplier = 1024 * 1024
		text = strings.TrimSuffix(text, "MB")
	case strings.HasSuffix(text, "GB"):
		multiplier = 1024 * 1024 * 1024
		text = strings.TrimSuffix(text, "GB")
	case strings.HasSuffix(text, "B"):
		text = strings.TrimSuffix(text, "B")
	}
	value, err := parsePositiveInt(text)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的大小值: %s", raw))
	}
	return int64(value) * multiplier, nil
}

func parsePositiveInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errs.InvalidArgument("数值不能为空")
	}
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

func auditResultFromError(err error) string {
	if errs.Code(err) == errs.ExitPermissionDenied {
		return "skipped"
	}
	return "failed"
}

func auditResultFromFailures(failed []string) string {
	if len(failed) == 0 {
		return "success"
	}
	for _, item := range failed {
		if isPermissionFailure(item) {
			return "skipped"
		}
	}
	return "partial"
}

func isPermissionFailure(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(normalized, "permission denied") ||
		strings.Contains(normalized, "access denied") ||
		strings.Contains(normalized, "operation not permitted")
}

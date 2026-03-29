package doctor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"syskit/internal/cliutil"
	"syskit/internal/domain/rules"
	"syskit/internal/errs"
	"syskit/internal/scanner"
	"syskit/pkg/utils"

	"github.com/spf13/cobra"
)

const (
	defaultDiskFullTopN = 20
)

type diskFullOptions struct {
	path string
	top  int
}

type slownessOptions struct {
	mode string
}

// newDiskFullCommand 创建 `doctor disk-full` 命令。
func newDiskFullCommand() *cobra.Command {
	opts := &diskFullOptions{
		path: ".",
		top:  defaultDiskFullTopN,
	}
	cmd := &cobra.Command{
		Use:   "disk-full",
		Short: "执行磁盘爆满场景诊断",
		Long: "doctor disk-full 用于面向“磁盘爆满”场景做专项诊断，综合分区容量、目录扫描和大文件线索输出可解释问题。" +
			"\n\n该命令为只读诊断，不会执行任何删除操作。",
		Example: "  syskit doctor disk-full\n" +
			"  syskit doctor disk-full --path /var/log --top 30\n" +
			"  syskit doctor disk-full --path . --top 10 --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorDiskFull(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.path, "path", ".", "磁盘爆满诊断根目录")
	flags.IntVar(&opts.top, "top", defaultDiskFullTopN, "目录和文件 Top 结果数量")
	return cmd
}

// newSlownessCommand 创建 `doctor slowness` 命令。
func newSlownessCommand() *cobra.Command {
	opts := &slownessOptions{mode: "quick"}
	cmd := &cobra.Command{
		Use:   "slowness",
		Short: "执行系统卡顿场景诊断",
		Long: "doctor slowness 用于面向“系统卡顿”场景做专项诊断，联合 CPU/内存/磁盘与高资源进程规则输出根因排序。" +
			"\n\n--mode deep 会启用更多规则并输出附加提示。",
		Example: "  syskit doctor slowness\n" +
			"  syskit doctor slowness --mode deep\n" +
			"  syskit doctor slowness --mode quick --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorSlowness(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.mode, "mode", "quick", "诊断模式: quick/deep")
	return cmd
}

func runDoctorDiskFull(cmd *cobra.Command, opts *diskFullOptions) error {
	path := filepath.Clean(strings.TrimSpace(opts.path))
	if path == "" || path == "." {
		if cwd, err := os.Getwd(); err == nil {
			path = cwd
		}
	}
	if strings.TrimSpace(path) == "" {
		return errs.InvalidArgument("--path 不能为空")
	}
	if opts.top <= 0 {
		return errs.InvalidArgument("--top 必须大于 0")
	}
	info, err := os.Stat(path)
	if err != nil {
		return errs.InvalidArgument(fmt.Sprintf("--path 不存在或不可访问: %s", path))
	}
	if !info.IsDir() {
		return errs.InvalidArgument("--path 必须是目录")
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	runtime, err := loadRuntime(cmd)
	if err != nil {
		return err
	}

	diagnoseInput, warnings := buildBaseInput(runtime)
	skippedCount := 0

	diskResult := collectDiskModule(ctx)
	warnings = append(warnings, diskResult.warnings...)
	if diskResult.skipped != nil {
		diagnoseInput.Skipped = append(diagnoseInput.Skipped, *diskResult.skipped)
		skippedCount++
	} else {
		mergeModuleResult(&diagnoseInput.Snapshots, diskResult)
	}

	scanResult, scanErr := scanDiskScenario(path, opts.top)
	if scanErr != nil {
		if degraded := rules.ClassifyModuleError("disk-scan", scanErr); degraded != nil {
			skipped := degraded.ToSkippedModule()
			diagnoseInput.Skipped = append(diagnoseInput.Skipped, skipped)
			skippedCount++
			warnings = append(warnings, fmt.Sprintf("目录扫描降级: %s", errs.Message(scanErr)))
		} else {
			return errs.ExecutionFailed("执行磁盘场景扫描失败", scanErr)
		}
	} else {
		diagnoseInput.Snapshots.Files = append(diagnoseInput.Snapshots.Files, toFileObservations(scanResult.TopFiles)...)
		warnings = append(warnings,
			fmt.Sprintf("扫描路径: %s", scanResult.ProcessedPath),
			fmt.Sprintf("扫描统计: files=%d dirs=%d total=%s", scanResult.TotalFiles, scanResult.TotalDirs, utils.FormatBytes(scanResult.TotalSize)),
			fmt.Sprintf("Top 结果: files=%d dirs=%d", len(scanResult.TopFiles), len(scanResult.TopDirs)),
		)
	}

	enabledRules := []string{"DISK-001", "FILE-001"}
	coverage := coverageOf(2, skippedCount)
	failOn := cliutil.ResolveStringFlag(cmd, "fail-on")

	resultData, exitCode, err := evaluateDoctor(ctx, diagnoseInput, enabledRules, coverage, failOn)
	if err != nil {
		return err
	}
	resultData.Scope = "module"
	resultData.Module = "disk-full"
	resultData.Warnings = append(resultData.Warnings, uniqueWarnings(warnings)...)
	return renderDoctorResult(cmd, startedAt, resultData, exitCode, "磁盘爆满场景诊断完成")
}

func runDoctorSlowness(cmd *cobra.Command, opts *slownessOptions) error {
	mode, err := normalizeMode(opts.mode)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	runtime, err := loadRuntime(cmd)
	if err != nil {
		return err
	}

	diagnoseInput, warnings := buildBaseInput(runtime)
	results := collectModulesConcurrently(ctx, runtime, []string{"cpu", "mem", "disk"})
	skippedCount := 0
	for _, result := range results {
		warnings = append(warnings, result.warnings...)
		if result.skipped != nil {
			diagnoseInput.Skipped = append(diagnoseInput.Skipped, *result.skipped)
			skippedCount++
			continue
		}
		mergeModuleResult(&diagnoseInput.Snapshots, result)
	}
	diagnoseInput.Snapshots.PathEntries = splitPathEntries(os.Getenv("PATH"))

	enabledRules := enabledRulesForSlowness(mode)
	if mode == "deep" {
		warnings = append(warnings, "deep 模式已启用扩展规则，建议结合 snapshot/disk scan 做趋势复核")
	}
	warnings = append(warnings, "当前版本未接入 startup 维度，后续版本会补齐启动项慢启动线索")

	coverage := coverageOf(3, skippedCount)
	failOn := cliutil.ResolveStringFlag(cmd, "fail-on")
	resultData, exitCode, err := evaluateDoctor(ctx, diagnoseInput, enabledRules, coverage, failOn)
	if err != nil {
		return err
	}
	resultData.Scope = "module"
	resultData.Module = "slowness"
	resultData.Mode = mode
	resultData.Warnings = append(resultData.Warnings, uniqueWarnings(warnings)...)
	return renderDoctorResult(cmd, startedAt, resultData, exitCode, "系统卡顿场景诊断完成")
}

func scanDiskScenario(path string, top int) (*scanner.ScanResult, error) {
	options := scanner.NewScanOptions(path)
	options.TopN = top
	options.IncludeFiles = true
	options.IncludeDirs = true
	options.ShowBanner = false
	options.ShowProgress = false
	options.LogOutput = io.Discard

	s := scanner.NewScanner(options)
	return s.Scan()
}

func toFileObservations(files []scanner.FileInfo) []rules.FileObservation {
	if len(files) == 0 {
		return nil
	}
	result := make([]rules.FileObservation, 0, len(files))
	for _, file := range files {
		result = append(result, rules.FileObservation{
			Path:             file.Path,
			SizeBytes:        file.Size,
			GrowthMBPerHour:  0,
			LastModifiedTime: file.ModTime,
		})
	}
	return result
}

func uniqueWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(warnings))
	for _, warning := range warnings {
		normalized := strings.TrimSpace(warning)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}

	result := make([]string, 0, len(set))
	for item := range set {
		result = append(result, item)
	}
	// 使用稳定顺序，避免测试和输出抖动。
	sort.Strings(result)
	return result
}

func enabledRulesForSlowness(mode string) []string {
	rulesList := []string{"CPU-001", "PROC-001", "MEM-001", "PROC-002", "DISK-001"}
	if mode == "deep" {
		rulesList = append(rulesList, "FILE-001", "ENV-001", "DISK-002")
	}
	return rulesList
}

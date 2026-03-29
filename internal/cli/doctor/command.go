// Package doctor 负责系统体检命令组。
package doctor

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"syskit/internal/cliutil"
	cpucollector "syskit/internal/collectors/cpu"
	diskcollector "syskit/internal/collectors/disk"
	memcollector "syskit/internal/collectors/mem"
	portcollector "syskit/internal/collectors/port"
	proccollector "syskit/internal/collectors/proc"
	"syskit/internal/config"
	"syskit/internal/domain/model"
	"syskit/internal/domain/rules"
	"syskit/internal/errs"
	"syskit/internal/output"
	"syskit/internal/policy"
	"time"

	"github.com/spf13/cobra"
)

type allOptions struct {
	mode    string
	exclude string
}

type portOptions struct {
	port        int
	commonPorts bool
}

type cpuOptions struct {
	threshold float64
	duration  int
}

type memOptions struct {
	threshold float64
}

type diskOptions struct {
	threshold     float64
	analyzeGrowth bool
}

// doctorOutput 是 doctor 命令输出数据结构。
type doctorOutput struct {
	Scope         string                `json:"scope"`
	Mode          string                `json:"mode,omitempty"`
	Module        string                `json:"module,omitempty"`
	Modules       []string              `json:"modules,omitempty"`
	HealthScore   int                   `json:"health_score"`
	HealthLevel   string                `json:"health_level"`
	Coverage      float64               `json:"coverage"`
	FailOn        string                `json:"fail_on"`
	FailOnMatched bool                  `json:"fail_on_matched"`
	Issues        []model.Issue         `json:"issues"`
	Skipped       []model.SkippedModule `json:"skipped,omitempty"`
	Warnings      []string              `json:"warnings,omitempty"`
}

type runtimeBundle struct {
	config *config.Config
	policy *policy.Policy
}

type moduleResult struct {
	name      string
	ports     []rules.PortSnapshot
	processes []rules.ProcessSnapshot
	cpu       *rules.CPUOverviewSnapshot
	memory    *rules.MemoryOverview
	memoryTop []rules.MemoryProcess
	disk      []rules.DiskPartition
	skipped   *model.SkippedModule
	warnings  []string
}

var doctorModules = []string{"port", "cpu", "mem", "disk"}
var doctorCommonPorts = []int{80, 443, 8080, 3306, 6379}

// NewCommand 创建 `doctor` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "系统体检与专项诊断",
		Long: "doctor 提供一键体检和专项诊断入口，用于把采集、规则判断、评分和退出码协议串成统一闭环。" +
			"\n\n当前已交付 all、port、cpu、mem、disk、network、disk-full、slowness 八个正式入口。",
		Example: "  syskit doctor all\n" +
			"  syskit doctor all --mode deep --fail-on medium\n" +
			"  syskit doctor network --target 1.1.1.1\n" +
			"  syskit doctor disk-full --path /var/log --top 20\n" +
			"  syskit doctor slowness --mode deep",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newAllCommand(),
		newPortCommand(),
		newCPUCommand(),
		newMemCommand(),
		newDiskCommand(),
		newNetworkCommand(),
		newDiskFullCommand(),
		newSlownessCommand(),
	)

	return cmd
}

func newAllCommand() *cobra.Command {
	opts := &allOptions{mode: "quick"}
	cmd := &cobra.Command{
		Use:   "all",
		Short: "执行全量体检",
		Long: "doctor all 会并发采集端口、CPU、内存和磁盘信息，统一输出健康分、问题清单、覆盖率和跳过项。" +
			"\n\n建议自动化场景配合 --fail-on 和 --format json 使用。",
		Example: "  syskit doctor all\n" +
			"  syskit doctor all --mode deep --fail-on never --format json\n" +
			"  syskit doctor all --exclude disk",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorAll(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.mode, "mode", "quick", "体检模式: quick/deep")
	flags.StringVar(&opts.exclude, "exclude", "", "排除模块（逗号分隔）: port,cpu,mem,disk")
	return cmd
}

func newPortCommand() *cobra.Command {
	opts := &portOptions{}
	cmd := &cobra.Command{
		Use:   "port",
		Short: "执行端口专项诊断",
		Long: "doctor port 用于诊断关键端口冲突和公网监听风险。" +
			"\n\n可以只针对指定端口，或只看常见关键端口集合。",
		Example: "  syskit doctor port\n" +
			"  syskit doctor port --common-ports\n" +
			"  syskit doctor port --port 8080 --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorPort(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.port, "port", 0, "仅诊断指定端口")
	flags.BoolVar(&opts.commonPorts, "common-ports", false, "仅聚焦常见关键端口")
	return cmd
}

func newCPUCommand() *cobra.Command {
	opts := &cpuOptions{duration: 3}
	cmd := &cobra.Command{
		Use:   "cpu",
		Short: "执行 CPU 专项诊断",
		Long: "doctor cpu 用于评估系统整体 CPU 使用率是否异常，并定位高 CPU 进程。" +
			"\n\n--duration 用于控制采样窗口，适合在短时波动场景下重复执行。",
		Example: "  syskit doctor cpu\n" +
			"  syskit doctor cpu --threshold 85\n" +
			"  syskit doctor cpu --duration 5 --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorCPU(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.Float64Var(&opts.threshold, "threshold", 0, "覆盖 CPU 阈值（百分比）")
	flags.IntVar(&opts.duration, "duration", 3, "采样窗口（秒），当前版本用于兼容占位")
	return cmd
}

func newMemCommand() *cobra.Command {
	opts := &memOptions{}
	cmd := &cobra.Command{
		Use:   "mem",
		Short: "执行内存专项诊断",
		Long: "doctor mem 用于评估系统可用内存和高内存进程是否越过阈值。" +
			"\n\n适合和 `mem`/`mem top` 配合，先做规则判断，再看详细排行。",
		Example: "  syskit doctor mem\n" +
			"  syskit doctor mem --threshold 90\n" +
			"  syskit doctor mem --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorMem(cmd, opts)
		},
	}

	cmd.Flags().Float64Var(&opts.threshold, "threshold", 0, "覆盖内存阈值（百分比）")
	return cmd
}

func newDiskCommand() *cobra.Command {
	opts := &diskOptions{}
	cmd := &cobra.Command{
		Use:   "disk",
		Short: "执行磁盘专项诊断",
		Long: "doctor disk 用于评估分区使用率、剩余空间和增长速率风险。" +
			"\n\n如果需要查看具体膨胀目录或大文件，请继续执行 `syskit disk scan <path>`。",
		Example: "  syskit doctor disk\n" +
			"  syskit doctor disk --threshold 90\n" +
			"  syskit doctor disk --analyze-growth --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorDisk(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.Float64Var(&opts.threshold, "threshold", 0, "覆盖磁盘阈值（百分比）")
	flags.BoolVar(&opts.analyzeGrowth, "analyze-growth", false, "启用增长速率诊断（DISK-002）")
	return cmd
}

func runDoctorAll(cmd *cobra.Command, opts *allOptions) error {
	startedAt := time.Now()
	mode, err := normalizeMode(opts.mode)
	if err != nil {
		return err
	}
	excludedModules, err := parseModules(opts.exclude)
	if err != nil {
		return err
	}
	selectedModules := selectModules(excludedModules)
	if len(selectedModules) == 0 {
		return errs.InvalidArgument("--exclude 不能排空所有模块")
	}

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
	collectResults := collectModulesConcurrently(ctx, runtime, selectedModules)
	for _, result := range collectResults {
		warnings = append(warnings, result.warnings...)
		if result.skipped != nil {
			diagnoseInput.Skipped = append(diagnoseInput.Skipped, *result.skipped)
			continue
		}
		mergeModuleResult(&diagnoseInput.Snapshots, result)
	}
	if mode == "deep" && containsModule(selectedModules, "disk") {
		warnings = append(warnings, "deep 模式已启用 DISK-002，但当前未接入历史快照，规则可能无法命中")
	}
	diagnoseInput.Snapshots.PathEntries = splitPathEntries(os.Getenv("PATH"))

	enabledRules := enabledRulesForAll(selectedModules, mode)
	coverage := coverageOf(len(selectedModules), len(diagnoseInput.Skipped))
	failOn := cliutil.ResolveStringFlag(cmd, "fail-on")

	resultData, exitCode, err := evaluateDoctor(ctx, diagnoseInput, enabledRules, coverage, failOn)
	if err != nil {
		return err
	}
	resultData.Scope = "all"
	resultData.Mode = mode
	resultData.Modules = selectedModules
	resultData.Warnings = append(resultData.Warnings, warnings...)

	return renderDoctorResult(cmd, startedAt, resultData, exitCode, "全量体检完成")
}

func runDoctorPort(cmd *cobra.Command, opts *portOptions) error {
	if opts.port < 0 {
		return errs.InvalidArgument("--port 不能小于 0")
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
	result := collectPortModule(ctx, runtime, opts)
	warnings = append(warnings, result.warnings...)
	if result.skipped != nil {
		diagnoseInput.Skipped = append(diagnoseInput.Skipped, *result.skipped)
	} else {
		mergeModuleResult(&diagnoseInput.Snapshots, result)
	}

	diagnoseInput.Snapshots.PathEntries = splitPathEntries(os.Getenv("PATH"))
	enabledRules := []string{"PORT-001", "PORT-002"}
	coverage := coverageOf(1, len(diagnoseInput.Skipped))
	failOn := cliutil.ResolveStringFlag(cmd, "fail-on")

	resultData, exitCode, err := evaluateDoctor(ctx, diagnoseInput, enabledRules, coverage, failOn)
	if err != nil {
		return err
	}
	resultData.Scope = "module"
	resultData.Module = "port"
	resultData.Warnings = append(resultData.Warnings, warnings...)

	return renderDoctorResult(cmd, startedAt, resultData, exitCode, "端口专项诊断完成")
}

func runDoctorCPU(cmd *cobra.Command, opts *cpuOptions) error {
	if opts.threshold < 0 {
		return errs.InvalidArgument("--threshold 不能小于 0")
	}
	if opts.duration <= 0 {
		return errs.InvalidArgument("--duration 必须大于 0")
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
	if opts.threshold > 0 {
		diagnoseInput.Options.Thresholds.CPUPercent = opts.threshold
	}
	warnings = append(warnings, fmt.Sprintf("CPU 采样窗口: %ds", opts.duration))

	result := collectCPUModule(ctx, runtime)
	warnings = append(warnings, result.warnings...)
	if result.skipped != nil {
		diagnoseInput.Skipped = append(diagnoseInput.Skipped, *result.skipped)
	} else {
		mergeModuleResult(&diagnoseInput.Snapshots, result)
	}

	diagnoseInput.Snapshots.PathEntries = splitPathEntries(os.Getenv("PATH"))
	enabledRules := []string{"CPU-001", "PROC-001"}
	coverage := coverageOf(1, len(diagnoseInput.Skipped))
	failOn := cliutil.ResolveStringFlag(cmd, "fail-on")

	resultData, exitCode, err := evaluateDoctor(ctx, diagnoseInput, enabledRules, coverage, failOn)
	if err != nil {
		return err
	}
	resultData.Scope = "module"
	resultData.Module = "cpu"
	resultData.Warnings = append(resultData.Warnings, warnings...)

	return renderDoctorResult(cmd, startedAt, resultData, exitCode, "CPU 专项诊断完成")
}

func runDoctorMem(cmd *cobra.Command, opts *memOptions) error {
	if opts.threshold < 0 {
		return errs.InvalidArgument("--threshold 不能小于 0")
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
	if opts.threshold > 0 {
		diagnoseInput.Options.Thresholds.MemPercent = opts.threshold
	}

	result := collectMemModule(ctx, runtime)
	warnings = append(warnings, result.warnings...)
	if result.skipped != nil {
		diagnoseInput.Skipped = append(diagnoseInput.Skipped, *result.skipped)
	} else {
		mergeModuleResult(&diagnoseInput.Snapshots, result)
	}

	diagnoseInput.Snapshots.PathEntries = splitPathEntries(os.Getenv("PATH"))
	enabledRules := []string{"MEM-001", "PROC-002"}
	coverage := coverageOf(1, len(diagnoseInput.Skipped))
	failOn := cliutil.ResolveStringFlag(cmd, "fail-on")

	resultData, exitCode, err := evaluateDoctor(ctx, diagnoseInput, enabledRules, coverage, failOn)
	if err != nil {
		return err
	}
	resultData.Scope = "module"
	resultData.Module = "mem"
	resultData.Warnings = append(resultData.Warnings, warnings...)

	return renderDoctorResult(cmd, startedAt, resultData, exitCode, "内存专项诊断完成")
}

func runDoctorDisk(cmd *cobra.Command, opts *diskOptions) error {
	if opts.threshold < 0 {
		return errs.InvalidArgument("--threshold 不能小于 0")
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
	if opts.threshold > 0 {
		diagnoseInput.Options.Thresholds.DiskPercent = opts.threshold
	}
	if opts.analyzeGrowth {
		warnings = append(warnings, "已启用 --analyze-growth，但当前未接入历史快照，DISK-002 可能无法命中")
	}

	result := collectDiskModule(ctx)
	warnings = append(warnings, result.warnings...)
	if result.skipped != nil {
		diagnoseInput.Skipped = append(diagnoseInput.Skipped, *result.skipped)
	} else {
		mergeModuleResult(&diagnoseInput.Snapshots, result)
	}

	diagnoseInput.Snapshots.PathEntries = splitPathEntries(os.Getenv("PATH"))
	enabledRules := enabledRulesForDisk(opts.analyzeGrowth)
	coverage := coverageOf(1, len(diagnoseInput.Skipped))
	failOn := cliutil.ResolveStringFlag(cmd, "fail-on")

	resultData, exitCode, err := evaluateDoctor(ctx, diagnoseInput, enabledRules, coverage, failOn)
	if err != nil {
		return err
	}
	resultData.Scope = "module"
	resultData.Module = "disk"
	resultData.Warnings = append(resultData.Warnings, warnings...)

	return renderDoctorResult(cmd, startedAt, resultData, exitCode, "磁盘专项诊断完成")
}

func renderDoctorResult(cmd *cobra.Command, startedAt time.Time, data *doctorOutput, exitCode int, msg string) error {
	if data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "doctor 结果为空")
	}
	result := output.NewSuccessResult(msg, data, startedAt)
	result.Code = exitCode

	if err := cliutil.RenderCommandResult(cmd, result, newDoctorPresenter(data)); err != nil {
		return err
	}

	switch exitCode {
	case errs.ExitSuccess:
		return nil
	case errs.ExitFailOnMatched:
		return errs.New(errs.ExitFailOnMatched, errs.CodeExecutionFailed, "命中 --fail-on 阈值")
	default:
		return errs.New(errs.ExitWarning, errs.CodeExecutionFailed, "检测到风险问题或跳过项")
	}
}

func evaluateDoctor(ctx context.Context, in rules.DiagnoseInput, enabledRules []string, coverage float64, failOn string) (*doctorOutput, int, error) {
	engine := rules.NewEngine(rules.NewP0Rules()...)
	evaluated, err := engine.Evaluate(ctx, in, enabledRules)
	if err != nil {
		return nil, errs.ExitExecutionFailed, errs.ExecutionFailed("规则评估失败", err)
	}

	scorer := rules.NewScorer()
	score, level := scorer.Score(evaluated.Issues, coverage)
	exitCode := deriveDoctorExitCode(evaluated.Issues, evaluated.Skipped, failOn)

	return &doctorOutput{
		HealthScore:   score,
		HealthLevel:   level,
		Coverage:      coverage,
		FailOn:        strings.ToLower(strings.TrimSpace(failOn)),
		FailOnMatched: exitCode == errs.ExitFailOnMatched,
		Issues:        evaluated.Issues,
		Skipped:       evaluated.Skipped,
	}, exitCode, nil
}

func deriveDoctorExitCode(issues []model.Issue, skipped []model.SkippedModule, failOn string) int {
	exitCode := rules.ResolveDoctorExitCode(issues, failOn)
	if exitCode == errs.ExitSuccess && len(skipped) > 0 {
		return errs.ExitWarning
	}
	return exitCode
}

func collectModulesConcurrently(ctx context.Context, runtime *runtimeBundle, modules []string) []moduleResult {
	results := make([]moduleResult, 0, len(modules))
	if len(modules) == 0 {
		return results
	}

	ch := make(chan moduleResult, len(modules))
	var wg sync.WaitGroup
	for _, module := range modules {
		module := module
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch module {
			case "port":
				ch <- collectPortModule(ctx, runtime, &portOptions{commonPorts: true})
			case "cpu":
				ch <- collectCPUModule(ctx, runtime)
			case "mem":
				ch <- collectMemModule(ctx, runtime)
			case "disk":
				ch <- collectDiskModule(ctx)
			default:
				ch <- moduleResult{
					name: module,
					skipped: &model.SkippedModule{
						Module:     module,
						Reason:     model.SkipReasonExecutionFailed,
						Impact:     "模块不在支持列表中，已跳过",
						Suggestion: "请检查 --exclude 配置",
					},
				}
			}
		}()
	}

	wg.Wait()
	close(ch)

	for item := range ch {
		results = append(results, item)
	}
	sort.Slice(results, func(i int, j int) bool {
		return results[i].name < results[j].name
	})
	return results
}

func collectPortModule(ctx context.Context, runtime *runtimeBundle, opts *portOptions) moduleResult {
	result := moduleResult{name: "port"}
	entries, warnings, err := collectPortEntries(ctx)
	if err != nil {
		result.skipped = skipFromError("port", err)
		result.warnings = append(result.warnings, fmt.Sprintf("端口采集失败: %s", errs.Message(err)))
		return result
	}
	result.warnings = append(result.warnings, warnings...)

	selected := make([]portcollector.PortEntry, 0, len(entries))
	commonPortSet := intSet(resolveCommonPorts(runtime))
	for _, entry := range entries {
		if opts != nil && opts.port > 0 && entry.Port != opts.port {
			continue
		}
		if opts != nil && opts.commonPorts {
			if _, ok := commonPortSet[entry.Port]; !ok {
				continue
			}
		}
		selected = append(selected, entry)
	}

	for _, entry := range selected {
		result.ports = append(result.ports, rules.PortSnapshot{
			Port:        entry.Port,
			LocalAddr:   entry.LocalAddr,
			PID:         entry.PID,
			ProcessName: entry.ProcessName,
			Command:     entry.Command,
			ParentPID:   entry.ParentPID,
		})
	}
	return result
}

func collectCPUModule(ctx context.Context, runtime *runtimeBundle) moduleResult {
	_ = runtime
	result := moduleResult{name: "cpu"}
	overview, err := cpucollector.CollectOverview(ctx, cpucollector.CollectOptions{Detail: false, TopN: 10})
	if err != nil {
		result.skipped = skipFromError("cpu", err)
		result.warnings = append(result.warnings, fmt.Sprintf("CPU 采集失败: %s", errs.Message(err)))
		return result
	}
	result.warnings = append(result.warnings, overview.Warnings...)

	cpuSnapshot := &rules.CPUOverviewSnapshot{
		CPUCores:     overview.CPUCores,
		UsagePercent: overview.UsagePercent,
		Load1:        overview.Load1,
		Load5:        overview.Load5,
		Load15:       overview.Load15,
		TopProcesses: make([]rules.ProcessSnapshot, 0, len(overview.TopProcesses)),
	}
	for _, proc := range overview.TopProcesses {
		cpuSnapshot.TopProcesses = append(cpuSnapshot.TopProcesses, rules.ProcessSnapshot{
			PID:        proc.PID,
			Name:       proc.Name,
			Command:    proc.Command,
			CPUPercent: proc.CPUPercent,
		})
	}
	result.cpu = cpuSnapshot

	topResult, topErr := proccollector.CollectTop(ctx, proccollector.TopOptions{
		By:   proccollector.SortByCPU,
		TopN: 30,
	})
	if topErr != nil {
		result.warnings = append(result.warnings, fmt.Sprintf("读取高 CPU 进程失败: %s", errs.Message(topErr)))
		return result
	}
	result.warnings = append(result.warnings, topResult.Warnings...)

	for _, proc := range topResult.Processes {
		result.processes = append(result.processes, rules.ProcessSnapshot{
			PID:        proc.PID,
			Name:       proc.Name,
			Command:    proc.Command,
			CPUPercent: proc.CPUPercent,
			RSSBytes:   proc.RSSBytes,
			VMSBytes:   proc.VMSBytes,
		})
	}
	return result
}

func collectMemModule(ctx context.Context, runtime *runtimeBundle) moduleResult {
	_ = runtime
	result := moduleResult{name: "mem"}
	overview, err := memcollector.CollectOverview(ctx, false, 20)
	if err != nil {
		result.skipped = skipFromError("mem", err)
		result.warnings = append(result.warnings, fmt.Sprintf("内存采集失败: %s", errs.Message(err)))
		return result
	}
	result.warnings = append(result.warnings, overview.Warnings...)
	result.memory = &rules.MemoryOverview{
		TotalBytes:       overview.TotalBytes,
		AvailableBytes:   overview.AvailableBytes,
		UsagePercent:     overview.UsagePercent,
		SwapUsagePercent: overview.SwapUsagePercent,
	}

	topResult, topErr := memcollector.CollectTop(ctx, memcollector.TopOptions{By: memcollector.SortByRSS, TopN: 30})
	if topErr != nil {
		result.warnings = append(result.warnings, fmt.Sprintf("读取高内存进程失败: %s", errs.Message(topErr)))
		for _, proc := range overview.TopProcesses {
			result.memoryTop = append(result.memoryTop, rules.MemoryProcess{
				PID:        proc.PID,
				Name:       proc.Name,
				Command:    proc.Command,
				MemPercent: float64(proc.MemPercent),
				RSSBytes:   proc.RSSBytes,
				SwapBytes:  proc.SwapBytes,
			})
		}
		return result
	}
	result.warnings = append(result.warnings, topResult.Warnings...)
	for _, proc := range topResult.Processes {
		result.memoryTop = append(result.memoryTop, rules.MemoryProcess{
			PID:        proc.PID,
			Name:       proc.Name,
			Command:    proc.Command,
			MemPercent: float64(proc.MemPercent),
			RSSBytes:   proc.RSSBytes,
			SwapBytes:  proc.SwapBytes,
		})
	}
	return result
}

func collectDiskModule(ctx context.Context) moduleResult {
	_ = ctx
	result := moduleResult{name: "disk"}
	overview, err := diskcollector.CollectOverview()
	if err != nil {
		wrapped := errs.ExecutionFailed("采集磁盘总览失败", err)
		result.skipped = skipFromError("disk", wrapped)
		result.warnings = append(result.warnings, fmt.Sprintf("磁盘采集失败: %s", errs.Message(wrapped)))
		return result
	}
	result.warnings = append(result.warnings, overview.Warnings...)
	for _, partition := range overview.Partitions {
		result.disk = append(result.disk, rules.DiskPartition{
			MountPoint:   partition.MountPoint,
			UsagePercent: partition.UsagePercent,
			FreeBytes:    partition.FreeBytes,
		})
	}
	return result
}

func mergeModuleResult(snapshots *rules.DiagnoseSnapshots, result moduleResult) {
	if snapshots == nil {
		return
	}
	if len(result.ports) > 0 {
		snapshots.Ports = append(snapshots.Ports, result.ports...)
	}
	if len(result.processes) > 0 {
		snapshots.Processes = append(snapshots.Processes, result.processes...)
	}
	if result.cpu != nil {
		snapshots.CPU = result.cpu
	}
	if result.memory != nil {
		snapshots.Memory = result.memory
	}
	if len(result.memoryTop) > 0 {
		snapshots.MemoryTop = append(snapshots.MemoryTop, result.memoryTop...)
	}
	if len(result.disk) > 0 {
		snapshots.Disk = append(snapshots.Disk, result.disk...)
	}
}

func buildBaseInput(runtime *runtimeBundle) (rules.DiagnoseInput, []string) {
	input := rules.DiagnoseInput{}
	warnings := make([]string, 0, 4)
	if runtime == nil {
		return input, warnings
	}

	input.Options = runtimeRuleOptions(runtime)
	return input, warnings
}

func runtimeRuleOptions(runtime *runtimeBundle) rules.DiagnoseOptions {
	options := rules.DiagnoseOptions{}
	if runtime == nil || runtime.config == nil {
		return options
	}

	options.Thresholds.CPUPercent = runtime.config.Thresholds.CPUPercent
	options.Thresholds.MemPercent = runtime.config.Thresholds.MemPercent
	options.Thresholds.DiskPercent = runtime.config.Thresholds.DiskPercent
	options.Thresholds.FileSizeGB = runtime.config.Thresholds.FileSizeGB
	options.Excludes.Ports = append([]int(nil), runtime.config.Excludes.Ports...)
	options.Excludes.Processes = append([]string(nil), runtime.config.Excludes.Processes...)

	if runtime.policy != nil {
		if runtime.policy.ThresholdOverrides.CPUPercent > 0 {
			options.Thresholds.CPUPercent = runtime.policy.ThresholdOverrides.CPUPercent
		}
		if runtime.policy.ThresholdOverrides.MemPercent > 0 {
			options.Thresholds.MemPercent = runtime.policy.ThresholdOverrides.MemPercent
		}
		if runtime.policy.ThresholdOverrides.DiskPercent > 0 {
			options.Thresholds.DiskPercent = runtime.policy.ThresholdOverrides.DiskPercent
		}
		if runtime.policy.ThresholdOverrides.FileSizeGB > 0 {
			options.Thresholds.FileSizeGB = runtime.policy.ThresholdOverrides.FileSizeGB
		}
		options.Policy.AllowPublicListen = append([]string(nil), runtime.policy.AllowPublicListen...)
	}

	return options
}

func loadRuntime(cmd *cobra.Command) (*runtimeBundle, error) {
	cfgResult, err := config.Load(config.LoadOptions{
		ExplicitPath: strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "config")),
	})
	if err != nil {
		return nil, err
	}

	policyResult, err := policy.Load(policy.LoadOptions{
		ExplicitPath: strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "policy")),
	})
	if err != nil {
		return nil, err
	}

	return &runtimeBundle{
		config: cfgResult.Config,
		policy: policyResult.Policy,
	}, nil
}

func collectPortEntries(ctx context.Context) ([]portcollector.PortEntry, []string, error) {
	list, err := portcollector.ListPorts(ctx, portcollector.ListOptions{By: "port"}, true)
	if err != nil {
		return nil, nil, err
	}
	return list.Entries, list.Warnings, nil
}

func enabledRulesForAll(modules []string, mode string) []string {
	set := make(map[string]struct{}, 12)
	for _, module := range modules {
		switch module {
		case "port":
			set["PORT-001"] = struct{}{}
			set["PORT-002"] = struct{}{}
		case "cpu":
			set["CPU-001"] = struct{}{}
			set["PROC-001"] = struct{}{}
		case "mem":
			set["MEM-001"] = struct{}{}
			set["PROC-002"] = struct{}{}
		case "disk":
			set["DISK-001"] = struct{}{}
			set["FILE-001"] = struct{}{}
			if mode == "deep" {
				set["DISK-002"] = struct{}{}
			}
		}
	}
	set["ENV-001"] = struct{}{}

	result := make([]string, 0, len(set))
	for item := range set {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func enabledRulesForDisk(analyzeGrowth bool) []string {
	rulesList := []string{"DISK-001", "FILE-001"}
	if analyzeGrowth {
		rulesList = append(rulesList, "DISK-002")
	}
	return rulesList
}

func normalizeMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return "quick", nil
	}
	switch mode {
	case "quick", "deep":
		return mode, nil
	default:
		return "", errs.InvalidArgument("--mode 仅支持 quick/deep")
	}
}

func parseModules(raw string) ([]string, error) {
	items := cliutil.SplitCSV(raw)
	if len(items) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{"port": {}, "cpu": {}, "mem": {}, "disk": {}}
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if _, ok := allowed[normalized]; !ok {
			return nil, errs.InvalidArgument(fmt.Sprintf("--exclude 包含不支持模块: %s", item))
		}
		set[normalized] = struct{}{}
	}

	result := make([]string, 0, len(set))
	for item := range set {
		result = append(result, item)
	}
	sort.Strings(result)
	return result, nil
}

func selectModules(excluded []string) []string {
	excludedSet := make(map[string]struct{}, len(excluded))
	for _, item := range excluded {
		excludedSet[item] = struct{}{}
	}

	result := make([]string, 0, len(doctorModules))
	for _, module := range doctorModules {
		if _, ok := excludedSet[module]; ok {
			continue
		}
		result = append(result, module)
	}
	return result
}

func containsModule(modules []string, target string) bool {
	for _, module := range modules {
		if module == target {
			return true
		}
	}
	return false
}

func coverageOf(totalModules int, skipped int) float64 {
	if totalModules <= 0 {
		return 0
	}
	if skipped < 0 {
		skipped = 0
	}
	covered := totalModules - skipped
	if covered < 0 {
		covered = 0
	}
	return float64(covered) * 100 / float64(totalModules)
}

func splitPathEntries(rawPath string) []string {
	if strings.TrimSpace(rawPath) == "" {
		return nil
	}
	parts := strings.Split(rawPath, string(os.PathListSeparator))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func skipFromError(module string, err error) *model.SkippedModule {
	if degraded := rules.ClassifyModuleError(module, err); degraded != nil {
		skipped := degraded.ToSkippedModule()
		return &skipped
	}

	suggestion := errs.Suggestion(err)
	if suggestion == "" {
		suggestion = "请查看错误信息后重试"
	}
	return &model.SkippedModule{
		Module:     module,
		Reason:     model.SkipReasonExecutionFailed,
		Impact:     fmt.Sprintf("%s 模块采集失败，未参与本轮规则评估", module),
		Suggestion: suggestion,
	}
}

func intSet(values []int) map[int]struct{} {
	set := make(map[int]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

// resolveCommonPorts 优先使用策略中的关键端口；未配置时回退到 P0 默认关键端口集。
// 这样 `doctor all` 的 common-ports 过滤与规则判定口径保持一致。
func resolveCommonPorts(runtime *runtimeBundle) []int {
	options := runtimeRuleOptions(runtime)
	if len(options.Policy.CriticalPorts) > 0 {
		return append([]int(nil), options.Policy.CriticalPorts...)
	}
	return append([]int(nil), doctorCommonPorts...)
}

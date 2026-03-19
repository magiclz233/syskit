// Package report 负责报告生成命令组。
package report

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"syskit/internal/cliutil"
	"syskit/internal/config"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"syskit/internal/output"
	"syskit/internal/storage"
	"time"

	"github.com/spf13/cobra"
)

const (
	reportTypeHealth     = "health"
	reportTypeInspection = "inspection"
	reportTypeMonitor    = "monitor"
)

var reportSupportedModules = []string{"port", "cpu", "mem", "disk", "proc"}

var reportSupportedModuleSet = map[string]struct{}{
	"port": {},
	"cpu":  {},
	"mem":  {},
	"disk": {},
	"proc": {},
}

type generateOptions struct {
	reportType string
	timeRange  string
}

type moduleCount struct {
	Module string `json:"module"`
	Count  int    `json:"count"`
}

type healthReportData struct {
	LatestSnapshot *model.SnapshotSummary `json:"latest_snapshot"`
	HealthScore    int                    `json:"health_score"`
	HealthLevel    string                 `json:"health_level"`
	WarningCount   int                    `json:"warning_count"`
	ModuleCoverage float64                `json:"module_coverage"`
	SnapshotCount  int                    `json:"snapshot_count"`
}

type inspectionReportData struct {
	SnapshotCount int           `json:"snapshot_count"`
	FirstAt       *time.Time    `json:"first_at,omitempty"`
	LastAt        *time.Time    `json:"last_at,omitempty"`
	Modules       []moduleCount `json:"modules"`
}

type monitorReportData struct {
	MonitorFileCount int    `json:"monitor_file_count"`
	RangeFileCount   int    `json:"range_file_count"`
	SnapshotSamples  int    `json:"snapshot_samples"`
	Note             string `json:"note,omitempty"`
}

type generateOutputData struct {
	Type        string                `json:"type"`
	GeneratedAt time.Time             `json:"generated_at"`
	TimeRange   string                `json:"time_range"`
	WindowStart time.Time             `json:"window_start"`
	WindowEnd   time.Time             `json:"window_end"`
	Health      *healthReportData     `json:"health,omitempty"`
	Inspection  *inspectionReportData `json:"inspection,omitempty"`
	Monitor     *monitorReportData    `json:"monitor,omitempty"`
	Warnings    []string              `json:"warnings,omitempty"`
}

// NewCommand 创建 `report` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "报告生成命令",
		Long:  "report 用于基于快照和监控目录生成健康、巡检和监控报告。",
		Example: "  syskit report generate\n" +
			"  syskit report generate --type inspection --time-range 7d\n" +
			"  syskit report generate --type monitor --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newGenerateCommand())
	return cmd
}

func newGenerateCommand() *cobra.Command {
	opts := &generateOptions{
		reportType: reportTypeHealth,
		timeRange:  "24h",
	}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "生成 health/inspection/monitor 报告",
		Long: "report generate 会读取快照存储和监控目录，在给定时间窗口内汇总为结构化报告。" +
			"\n\n当窗口内缺少样本时，结果会通过 warnings 明确提示降级情况。",
		Example: "  syskit report generate\n" +
			"  syskit report generate --type inspection --time-range 7d\n" +
			"  syskit report generate --type health --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.reportType, "type", reportTypeHealth, "报告类型: health/inspection/monitor")
	flags.StringVar(&opts.timeRange, "time-range", "24h", "时间范围（示例: 24h, 7d, 2w）")
	return cmd
}

func runGenerate(cmd *cobra.Command, opts *generateOptions) error {
	startedAt := time.Now()
	reportType, err := normalizeReportType(opts.reportType)
	if err != nil {
		return err
	}
	rangeDuration, err := parseTimeRange(opts.timeRange)
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
	layout, err := storage.EnsureLayout(cfg.Storage.DataDir)
	if err != nil {
		return err
	}

	windowEnd := time.Now().UTC()
	windowStart := windowEnd.Add(-rangeDuration)

	allSnapshots, err := store.List(ctx, 0)
	if err != nil {
		return err
	}
	rangeSnapshots := filterSnapshotsByWindow(allSnapshots, windowStart, windowEnd)

	data := &generateOutputData{
		Type:        reportType,
		GeneratedAt: windowEnd,
		TimeRange:   opts.timeRange,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Warnings:    make([]string, 0, 4),
	}

	switch reportType {
	case reportTypeHealth:
		report, warnings, buildErr := buildHealthReport(ctx, store, rangeSnapshots, allSnapshots)
		if buildErr != nil {
			return buildErr
		}
		data.Health = report
		data.Warnings = append(data.Warnings, warnings...)
	case reportTypeInspection:
		data.Inspection = buildInspectionReport(rangeSnapshots)
		if len(rangeSnapshots) == 0 {
			data.Warnings = append(data.Warnings, "当前时间范围没有巡检快照数据")
		}
	case reportTypeMonitor:
		report, warnings := buildMonitorReport(layout.MonitorDir, windowStart, windowEnd, rangeSnapshots)
		data.Monitor = report
		data.Warnings = append(data.Warnings, warnings...)
	}
	data.Warnings = dedupeStrings(data.Warnings)

	msg := fmt.Sprintf("报告生成完成（type=%s）", reportType)
	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newReportPresenter(data))
}

func normalizeReportType(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case reportTypeHealth, reportTypeInspection, reportTypeMonitor:
		return value, nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--type 仅支持 health/inspection/monitor，当前为: %s", raw))
	}
}

func parseTimeRange(raw string) (time.Duration, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return 24 * time.Hour, nil
	}

	if duration, err := time.ParseDuration(value); err == nil {
		if duration <= 0 {
			return 0, errs.InvalidArgument("--time-range 必须大于 0")
		}
		return duration, nil
	}

	if strings.HasSuffix(value, "d") || strings.HasSuffix(value, "w") {
		numeric := strings.TrimSuffix(strings.TrimSuffix(value, "d"), "w")
		n, err := strconv.Atoi(numeric)
		if err != nil || n <= 0 {
			return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --time-range: %s", raw))
		}
		if strings.HasSuffix(value, "d") {
			return time.Duration(n) * 24 * time.Hour, nil
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}

	return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --time-range: %s（示例: 24h, 7d, 2w）", raw))
}

func filterSnapshotsByWindow(items []model.SnapshotSummary, start time.Time, end time.Time) []model.SnapshotSummary {
	if len(items) == 0 {
		return nil
	}
	result := make([]model.SnapshotSummary, 0, len(items))
	for _, item := range items {
		if item.CreatedAt.Before(start) || item.CreatedAt.After(end) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func buildHealthReport(ctx context.Context, store *storage.SnapshotStore, rangeItems []model.SnapshotSummary, allItems []model.SnapshotSummary) (*healthReportData, []string, error) {
	warnings := make([]string, 0, 2)
	candidate := rangeItems
	if len(candidate) == 0 {
		candidate = allItems
		warnings = append(warnings, "当前时间范围无快照，已回退到最近可用快照")
	}
	if len(candidate) == 0 {
		return nil, nil, errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, "没有可用于生成 health 报告的快照")
	}

	latest := candidate[0]
	snapshot, err := store.Load(ctx, latest.ID)
	if err != nil {
		return nil, nil, err
	}

	covered := 0
	for module := range snapshot.Modules {
		if _, ok := reportSupportedModuleSet[module]; ok {
			covered++
		}
	}
	coverage := float64(covered) * 100 / float64(len(reportSupportedModules))
	warningCount := len(snapshot.Warnings)
	score := 100 - warningCount*5 - int((100-coverage)/2)
	if score < 0 {
		score = 0
	}
	level := "healthy"
	if score < 60 {
		level = "warning"
	} else if score < 85 {
		level = "degraded"
	}

	return &healthReportData{
		LatestSnapshot: &latest,
		HealthScore:    score,
		HealthLevel:    level,
		WarningCount:   warningCount,
		ModuleCoverage: coverage,
		SnapshotCount:  len(candidate),
	}, warnings, nil
}

func buildInspectionReport(items []model.SnapshotSummary) *inspectionReportData {
	report := &inspectionReportData{
		SnapshotCount: len(items),
		Modules:       make([]moduleCount, 0, 8),
	}
	if len(items) == 0 {
		return report
	}

	first := items[len(items)-1].CreatedAt
	last := items[0].CreatedAt
	report.FirstAt = &first
	report.LastAt = &last

	moduleCounter := make(map[string]int, 8)
	for _, item := range items {
		for _, module := range item.Modules {
			moduleCounter[module]++
		}
	}
	for module, count := range moduleCounter {
		report.Modules = append(report.Modules, moduleCount{Module: module, Count: count})
	}
	sort.Slice(report.Modules, func(i int, j int) bool {
		if report.Modules[i].Count == report.Modules[j].Count {
			return report.Modules[i].Module < report.Modules[j].Module
		}
		return report.Modules[i].Count > report.Modules[j].Count
	})
	return report
}

func buildMonitorReport(monitorDir string, start time.Time, end time.Time, snapshots []model.SnapshotSummary) (*monitorReportData, []string) {
	report := &monitorReportData{}
	warnings := make([]string, 0, 2)

	entries, err := os.ReadDir(monitorDir)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("读取 monitor 目录失败: %v", err))
		report.SnapshotSamples = len(snapshots)
		if len(snapshots) == 0 {
			report.Note = "当前无 monitor 文件，也无快照样本"
		}
		return report, warnings
	}

	for _, item := range entries {
		if item.IsDir() {
			continue
		}
		report.MonitorFileCount++
		info, statErr := item.Info()
		if statErr != nil {
			continue
		}
		modTime := info.ModTime().UTC()
		if !modTime.Before(start) && !modTime.After(end) {
			report.RangeFileCount++
		}
	}
	report.SnapshotSamples = len(snapshots)
	if report.MonitorFileCount == 0 {
		report.Note = "monitor 模块尚未落地，当前以快照样本作为替代指标"
		warnings = append(warnings, report.Note)
	}
	return report, warnings
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

func loadRuntimeConfig(cmd *cobra.Command) (*config.Config, error) {
	loadResult, err := config.Load(config.LoadOptions{
		ExplicitPath: strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "config")),
	})
	if err != nil {
		return nil, err
	}
	return loadResult.Config, nil
}

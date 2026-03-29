package doctor

import (
	"context"
	"os"
	"sort"
	"strings"
	"syskit/internal/config"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"syskit/internal/policy"
)

// AllRunRequest 定义程序化触发 `doctor all` 的输入参数。
type AllRunRequest struct {
	ConfigPath string
	PolicyPath string
	Mode       string
	Exclude    string
	FailOn     string
}

// AllRunResult 表示一次 `doctor all` 的结构化执行结果。
type AllRunResult struct {
	Mode          string                `json:"mode"`
	Modules       []string              `json:"modules"`
	HealthScore   int                   `json:"health_score"`
	HealthLevel   string                `json:"health_level"`
	Coverage      float64               `json:"coverage"`
	FailOn        string                `json:"fail_on"`
	FailOnMatched bool                  `json:"fail_on_matched"`
	Issues        []model.Issue         `json:"issues"`
	Skipped       []model.SkippedModule `json:"skipped,omitempty"`
	Warnings      []string              `json:"warnings,omitempty"`
	ExitCode      int                   `json:"exit_code"`
}

// RunAllOnce 以服务方式执行一次 `doctor all`，供 monitor 定时巡检复用。
func RunAllOnce(ctx context.Context, req AllRunRequest) (*AllRunResult, error) {
	mode, err := normalizeMode(req.Mode)
	if err != nil {
		return nil, err
	}
	excludedModules, err := parseModules(req.Exclude)
	if err != nil {
		return nil, err
	}
	selectedModules := selectModules(excludedModules)
	if len(selectedModules) == 0 {
		return nil, errs.InvalidArgument("--exclude 不能排空所有模块")
	}

	runtime, err := loadRuntimeByPath(req.ConfigPath, req.PolicyPath)
	if err != nil {
		return nil, err
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
	failOn := strings.ToLower(strings.TrimSpace(req.FailOn))
	if failOn == "" {
		failOn = "high"
	}

	outputData, exitCode, evalErr := evaluateDoctor(ctx, diagnoseInput, enabledRules, coverage, failOn)
	if evalErr != nil {
		return nil, evalErr
	}

	return &AllRunResult{
		Mode:          mode,
		Modules:       selectedModules,
		HealthScore:   outputData.HealthScore,
		HealthLevel:   outputData.HealthLevel,
		Coverage:      outputData.Coverage,
		FailOn:        outputData.FailOn,
		FailOnMatched: outputData.FailOnMatched,
		Issues:        outputData.Issues,
		Skipped:       outputData.Skipped,
		Warnings:      appendUniqueStrings(warnings, outputData.Warnings...),
		ExitCode:      exitCode,
	}, nil
}

func loadRuntimeByPath(configPath string, policyPath string) (*runtimeBundle, error) {
	cfgResult, err := config.Load(config.LoadOptions{
		ExplicitPath: strings.TrimSpace(configPath),
	})
	if err != nil {
		return nil, err
	}
	policyResult, err := policy.Load(policy.LoadOptions{
		ExplicitPath: strings.TrimSpace(policyPath),
	})
	if err != nil {
		return nil, err
	}
	return &runtimeBundle{
		config: cfgResult.Config,
		policy: policyResult.Policy,
	}, nil
}

func appendUniqueStrings(items []string, values ...string) []string {
	set := make(map[string]struct{}, len(items)+len(values))
	result := make([]string, 0, len(items)+len(values))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := set[item]; ok {
			continue
		}
		set[item] = struct{}{}
		result = append(result, item)
	}
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := set[item]; ok {
			continue
		}
		set[item] = struct{}{}
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

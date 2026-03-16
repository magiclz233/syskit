// Package rules 提供规则执行接口与默认规则引擎实现。
package rules

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
)

const (
	unknownModule = "unknown"
)

// DiagnoseInput 是规则执行时共享的输入载体。
// P0 先提供最小通用字段，后续专项诊断可在不破坏协议的前提下增量扩展。
type DiagnoseInput struct {
	Mode       string         `json:"mode,omitempty"`
	ModuleData map[string]any `json:"module_data,omitempty"`
	Skipped    []model.SkippedModule
}

// Rule 是单条诊断规则的执行接口。
// Phase 字段用于后续按 P0/P1/P2 进行规则分层。
type Rule interface {
	ID() string
	Phase() string
	Check(ctx context.Context, in DiagnoseInput) (*model.Issue, error)
}

// ModuleScopedRule 是可选接口。
// 规则实现该接口后，规则引擎可在模块降级时自动跳过同模块后续规则。
type ModuleScopedRule interface {
	Module() string
}

// Engine 是规则引擎接口。
type Engine interface {
	Evaluate(ctx context.Context, in DiagnoseInput, enabled []string) (*EvaluationResult, error)
}

// EvaluationResult 是规则执行结果。
type EvaluationResult struct {
	Issues  []model.Issue         `json:"issues"`
	Skipped []model.SkippedModule `json:"skipped,omitempty"`
}

// ModuleDegradeError 表示模块级可降级错误。
// 该错误不会直接终止整体诊断，而会被转换为 skipped 输出。
type ModuleDegradeError struct {
	Module             string
	Reason             string
	RequiredPermission string
	Impact             string
	Suggestion         string
	Err                error
}

// Error 实现 error 接口。
func (e *ModuleDegradeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("模块 %s 降级: %s", normalizeModule(e.Module), model.NormalizeSkipReason(e.Reason))
}

// Unwrap 返回底层错误。
func (e *ModuleDegradeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ToSkippedModule 转换为统一的 skipped 结构。
func (e *ModuleDegradeError) ToSkippedModule() model.SkippedModule {
	if e == nil {
		return model.SkippedModule{}
	}
	module := normalizeModule(e.Module)
	reason := model.NormalizeSkipReason(e.Reason)
	return model.SkippedModule{
		Module:             module,
		Reason:             reason,
		RequiredPermission: strings.TrimSpace(e.RequiredPermission),
		Impact:             fallbackText(strings.TrimSpace(e.Impact), fmt.Sprintf("未覆盖 %s 模块检查", module)),
		Suggestion:         fallbackText(strings.TrimSpace(e.Suggestion), defaultSuggestionByReason(reason)),
	}
}

// NewModuleDegradeError 创建模块降级错误。
func NewModuleDegradeError(module string, reason string, impact string, suggestion string, err error) *ModuleDegradeError {
	return &ModuleDegradeError{
		Module:     normalizeModule(module),
		Reason:     model.NormalizeSkipReason(reason),
		Impact:     strings.TrimSpace(impact),
		Suggestion: strings.TrimSpace(suggestion),
		Err:        err,
	}
}

// DefaultEngine 是 P0 阶段的默认规则引擎实现。
type DefaultEngine struct {
	rules []Rule
}

// NewEngine 创建默认规则引擎。
func NewEngine(rules ...Rule) *DefaultEngine {
	cloned := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		cloned = append(cloned, rule)
	}
	return &DefaultEngine{rules: cloned}
}

// Evaluate 执行规则并返回问题清单与模块级跳过项。
// 设计原则：
// 1. 同模块出现权限/超时/平台不支持时，后续同模块规则直接跳过；
// 2. 其他未知错误直接返回，避免静默吞错。
func (e *DefaultEngine) Evaluate(ctx context.Context, in DiagnoseInput, enabled []string) (*EvaluationResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	result := &EvaluationResult{
		Issues: make([]model.Issue, 0, len(e.rules)),
	}
	skippedMap := make(map[string]model.SkippedModule, len(in.Skipped))
	for _, skipped := range in.Skipped {
		appendSkipped(skippedMap, skipped)
	}

	enabledSet := buildEnabledSet(enabled)
	for _, rule := range e.rules {
		if err := ctx.Err(); err != nil {
			return nil, wrapContextError(err)
		}

		ruleID := strings.TrimSpace(rule.ID())
		if ruleID == "" {
			return nil, errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "规则 ID 不能为空")
		}
		if len(enabledSet) > 0 {
			if _, ok := enabledSet[strings.ToUpper(ruleID)]; !ok {
				continue
			}
		}

		module := ruleModule(rule)
		if _, skipped := skippedMap[module]; skipped {
			continue
		}

		issue, err := rule.Check(ctx, in)
		if err != nil {
			if degraded := ClassifyModuleError(module, err); degraded != nil {
				appendSkipped(skippedMap, degraded.ToSkippedModule())
				continue
			}
			return nil, errs.ExecutionFailed(fmt.Sprintf("规则 %s 执行失败", ruleID), err)
		}
		if issue == nil {
			continue
		}

		normalized, normalizeErr := normalizeIssue(ruleID, issue)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		result.Issues = append(result.Issues, normalized)
	}

	sortIssues(result.Issues)
	result.Skipped = sortedSkipped(skippedMap)
	return result, nil
}

// ClassifyModuleError 把可降级错误映射为模块跳过信息。
// 仅权限不足、超时、平台不支持三类错误会降级；其他错误继续向上抛出。
func ClassifyModuleError(module string, err error) *ModuleDegradeError {
	if err == nil {
		return nil
	}

	var degraded *ModuleDegradeError
	if errors.As(err, &degraded) {
		normalized := NewModuleDegradeError(
			fallbackText(strings.TrimSpace(degraded.Module), module),
			degraded.Reason,
			degraded.Impact,
			degraded.Suggestion,
			degraded.Err,
		)
		normalized.RequiredPermission = strings.TrimSpace(degraded.RequiredPermission)
		return normalized
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return NewModuleDegradeError(module, model.SkipReasonTimeout, "", "", err)
	}

	switch errs.ErrorCode(err) {
	case errs.CodePermissionDenied:
		degraded = NewModuleDegradeError(module, model.SkipReasonPermissionDenied, "", "", err)
		degraded.RequiredPermission = "admin/root"
		return degraded
	case errs.CodeTimeout:
		return NewModuleDegradeError(module, model.SkipReasonTimeout, "", "", err)
	case errs.CodePlatformUnsupported:
		return NewModuleDegradeError(module, model.SkipReasonUnsupported, "", "", err)
	default:
		return nil
	}
}

func buildEnabledSet(enabled []string) map[string]struct{} {
	set := make(map[string]struct{}, len(enabled))
	for _, item := range enabled {
		key := strings.ToUpper(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		set[key] = struct{}{}
	}
	return set
}

func normalizeIssue(ruleID string, issue *model.Issue) (model.Issue, error) {
	normalized := *issue
	if strings.TrimSpace(normalized.RuleID) == "" {
		normalized.RuleID = ruleID
	}

	severity, ok := model.NormalizeSeverity(normalized.Severity)
	if !ok {
		return model.Issue{}, errs.New(
			errs.ExitExecutionFailed,
			errs.CodeExecutionFailed,
			fmt.Sprintf("规则 %s 返回非法 severity: %s", normalized.RuleID, normalized.Severity),
		)
	}
	normalized.Severity = severity
	normalized.Scope = model.NormalizeScope(normalized.Scope)
	normalized.Confidence = clamp(normalized.Confidence, 0, 100)
	return normalized, nil
}

func sortIssues(issues []model.Issue) {
	sort.Slice(issues, func(i int, j int) bool {
		left := model.SeverityRank(issues[i].Severity)
		right := model.SeverityRank(issues[j].Severity)
		if left != right {
			return left > right
		}
		if issues[i].RuleID != issues[j].RuleID {
			return issues[i].RuleID < issues[j].RuleID
		}
		return issues[i].Summary < issues[j].Summary
	})
}

func sortedSkipped(items map[string]model.SkippedModule) []model.SkippedModule {
	if len(items) == 0 {
		return nil
	}
	result := make([]model.SkippedModule, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	sort.Slice(result, func(i int, j int) bool {
		if result[i].Module != result[j].Module {
			return result[i].Module < result[j].Module
		}
		return result[i].Reason < result[j].Reason
	})
	return result
}

func appendSkipped(collection map[string]model.SkippedModule, skipped model.SkippedModule) {
	if collection == nil {
		return
	}
	module := normalizeModule(skipped.Module)
	collection[module] = model.SkippedModule{
		Module:             module,
		Reason:             model.NormalizeSkipReason(skipped.Reason),
		RequiredPermission: strings.TrimSpace(skipped.RequiredPermission),
		Impact:             fallbackText(strings.TrimSpace(skipped.Impact), fmt.Sprintf("未覆盖 %s 模块检查", module)),
		Suggestion: fallbackText(
			strings.TrimSpace(skipped.Suggestion),
			defaultSuggestionByReason(model.NormalizeSkipReason(skipped.Reason)),
		),
	}
}

func ruleModule(rule Rule) string {
	if scoped, ok := rule.(ModuleScopedRule); ok {
		return normalizeModule(scoped.Module())
	}
	return unknownModule
}

func normalizeModule(module string) string {
	value := strings.ToLower(strings.TrimSpace(module))
	if value == "" {
		return unknownModule
	}
	return value
}

func defaultSuggestionByReason(reason string) string {
	switch model.NormalizeSkipReason(reason) {
	case model.SkipReasonPermissionDenied:
		return "请提升权限后重试"
	case model.SkipReasonTimeout:
		return "请调大 --timeout 或缩小检查范围后重试"
	case model.SkipReasonUnsupported:
		return "请在受支持的平台执行或调整策略"
	default:
		return "请查看错误信息后重试"
	}
}

func wrapContextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "规则执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "规则执行已取消")
	}
	return errs.ExecutionFailed("规则执行失败", err)
}

func clamp(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func fallbackText(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

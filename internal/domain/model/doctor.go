// Package model 定义 doctor 领域共享模型。
package model

import "strings"

const (
	// SeverityCritical 表示最高严重级别，通常用于直接阻断。
	SeverityCritical = "critical"
	// SeverityHigh 表示高严重级别。
	SeverityHigh = "high"
	// SeverityMedium 表示中严重级别。
	SeverityMedium = "medium"
	// SeverityLow 表示低严重级别。
	SeverityLow = "low"
)

const (
	// ScopeLocal 表示问题作用在本地进程或局部资源。
	ScopeLocal = "local"
	// ScopeSystem 表示问题作用在系统级资源。
	ScopeSystem = "system"
	// ScopeDestructive 表示问题或修复动作具备较高破坏性。
	ScopeDestructive = "destructive"
)

const (
	// SkipReasonPermissionDenied 表示模块因权限不足被跳过。
	SkipReasonPermissionDenied = "permission_denied"
	// SkipReasonTimeout 表示模块因超时被跳过。
	SkipReasonTimeout = "timeout"
	// SkipReasonUnsupported 表示模块因平台不支持被跳过。
	SkipReasonUnsupported = "unsupported"
	// SkipReasonExecutionFailed 表示模块因其他执行错误被跳过。
	SkipReasonExecutionFailed = "execution_failed"
)

// DoctorReport 是 doctor 命令的数据主体。
// 该结构与 CLI 规范保持一致，便于后续 doctor all 和 report 复用。
type DoctorReport struct {
	HealthScore int             `json:"health_score"`
	HealthLevel string          `json:"health_level"`
	Coverage    float64         `json:"coverage"`
	Issues      []Issue         `json:"issues"`
	Skipped     []SkippedModule `json:"skipped"`
}

// Issue 描述一条规则命中的诊断问题。
type Issue struct {
	RuleID      string `json:"rule_id"`
	Severity    string `json:"severity"`
	Summary     string `json:"summary"`
	Evidence    any    `json:"evidence"`
	Impact      string `json:"impact"`
	Suggestion  string `json:"suggestion"`
	FixCommand  string `json:"fix_command"`
	AutoFixable bool   `json:"auto_fixable"`
	Confidence  int    `json:"confidence"`
	Scope       string `json:"scope"`
}

// SkippedModule 记录模块级降级信息。
type SkippedModule struct {
	Module             string `json:"module"`
	Reason             string `json:"reason"`
	RequiredPermission string `json:"required_permission"`
	Impact             string `json:"impact"`
	Suggestion         string `json:"suggestion"`
}

// NormalizeSeverity 归一化并校验严重级别取值。
func NormalizeSeverity(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow:
		return value, true
	default:
		return "", false
	}
}

// SeverityRank 返回严重级别的排序权重，值越大表示优先级越高。
func SeverityRank(severity string) int {
	switch normalized, _ := NormalizeSeverity(severity); normalized {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}

// NormalizeScope 返回规范化后的作用域，空值回退到 local。
func NormalizeScope(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case ScopeLocal, ScopeSystem, ScopeDestructive:
		return value
	default:
		return ScopeLocal
	}
}

// NormalizeSkipReason 归一化模块跳过原因，未知值回退到 execution_failed。
func NormalizeSkipReason(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case SkipReasonPermissionDenied, SkipReasonTimeout, SkipReasonUnsupported, SkipReasonExecutionFailed:
		return value
	default:
		return SkipReasonExecutionFailed
	}
}

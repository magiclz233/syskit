package rules

import (
	"math"
	"strings"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
)

const (
	// FailOnNever 表示始终不触发 CI 阻断。
	FailOnNever = "never"
)

// Scorer 定义健康分计算接口。
type Scorer interface {
	Score(issues []model.Issue, coverage float64) (score int, level string)
}

// DefaultScorer 是 P0 阶段默认评分器。
// 当前按 severity/confidence/scope 综合扣分，coverage 仅透传给上层展示。
type DefaultScorer struct{}

// NewScorer 创建默认评分器。
func NewScorer() *DefaultScorer {
	return &DefaultScorer{}
}

// Score 计算健康分和健康等级。
func (s *DefaultScorer) Score(issues []model.Issue, coverage float64) (int, string) {
	_ = coverage // coverage 单独输出，不折算进健康分

	totalPenalty := 0.0
	for _, issue := range issues {
		totalPenalty += penaltyOf(issue)
	}

	score := int(math.Round(100 - totalPenalty))
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, levelByScore(score)
}

// IsFailOnMatched 判断问题列表是否命中 fail-on 阈值。
func IsFailOnMatched(issues []model.Issue, failOn string) bool {
	threshold := normalizeFailOn(failOn)
	if threshold == FailOnNever {
		return false
	}

	thresholdRank := model.SeverityRank(threshold)
	if thresholdRank == 0 {
		return false
	}

	for _, issue := range issues {
		if model.SeverityRank(issue.Severity) >= thresholdRank {
			return true
		}
	}
	return false
}

// ResolveDoctorExitCode 根据问题列表和 fail-on 阈值返回 doctor 场景退出码。
func ResolveDoctorExitCode(issues []model.Issue, failOn string) int {
	if IsFailOnMatched(issues, failOn) {
		return errs.ExitFailOnMatched
	}
	if len(issues) > 0 {
		return errs.ExitWarning
	}
	return errs.ExitSuccess
}

func penaltyOf(issue model.Issue) float64 {
	base := basePenalty(issue.Severity)
	if base <= 0 {
		return 0
	}
	return base * confidenceWeight(issue.Confidence) * scopeWeight(issue.Scope)
}

func basePenalty(severity string) float64 {
	switch normalized, _ := model.NormalizeSeverity(severity); normalized {
	case model.SeverityCritical:
		return 20
	case model.SeverityHigh:
		return 10
	case model.SeverityMedium:
		return 5
	case model.SeverityLow:
		return 2
	default:
		return 0
	}
}

func confidenceWeight(confidence int) float64 {
	value := confidence
	if value <= 0 {
		value = 100
	}
	if value > 100 {
		value = 100
	}
	return float64(value) / 100.0
}

func scopeWeight(scope string) float64 {
	switch model.NormalizeScope(scope) {
	case model.ScopeSystem:
		return 1.2
	case model.ScopeDestructive:
		return 1.5
	default:
		return 1.0
	}
}

func levelByScore(score int) string {
	switch {
	case score >= 90:
		return "healthy"
	case score >= 70:
		return "degraded"
	case score >= 60:
		return "warning"
	default:
		return "critical"
	}
}

func normalizeFailOn(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case model.SeverityCritical:
		return model.SeverityCritical
	case model.SeverityHigh:
		return model.SeverityHigh
	case model.SeverityMedium:
		return model.SeverityMedium
	case model.SeverityLow:
		return model.SeverityLow
	case FailOnNever:
		return FailOnNever
	default:
		return model.SeverityHigh
	}
}

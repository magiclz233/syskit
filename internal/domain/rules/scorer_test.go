package rules

import (
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"testing"
)

func TestScoreUsesSeverityConfidenceScope(t *testing.T) {
	scorer := NewScorer()
	issues := []model.Issue{
		{
			RuleID:      "DISK-001",
			Severity:    model.SeverityCritical,
			Summary:     "磁盘爆满",
			Confidence:  50,
			Scope:       model.ScopeDestructive,
			AutoFixable: true,
		},
		{
			RuleID:     "ENV-001",
			Severity:   model.SeverityLow,
			Summary:    "PATH 冲突",
			Confidence: 100,
			Scope:      model.ScopeLocal,
		},
	}

	score, level := scorer.Score(issues, 50)
	if score != 83 {
		t.Fatalf("score want=83 got=%d", score)
	}
	if level != "degraded" {
		t.Fatalf("level want=degraded got=%s", level)
	}
}

func TestScoreClampsToZero(t *testing.T) {
	scorer := NewScorer()
	issues := []model.Issue{
		{Severity: model.SeverityCritical, Confidence: 100, Scope: model.ScopeDestructive},
		{Severity: model.SeverityCritical, Confidence: 100, Scope: model.ScopeDestructive},
		{Severity: model.SeverityCritical, Confidence: 100, Scope: model.ScopeDestructive},
		{Severity: model.SeverityCritical, Confidence: 100, Scope: model.ScopeDestructive},
		{Severity: model.SeverityCritical, Confidence: 100, Scope: model.ScopeDestructive},
	}

	score, level := scorer.Score(issues, 100)
	if score != 0 {
		t.Fatalf("score want=0 got=%d", score)
	}
	if level != "critical" {
		t.Fatalf("level want=critical got=%s", level)
	}
}

func TestIsFailOnMatched(t *testing.T) {
	issues := []model.Issue{
		{Severity: model.SeverityMedium},
		{Severity: model.SeverityCritical},
	}

	if !IsFailOnMatched(issues, model.SeverityHigh) {
		t.Fatalf("expected fail-on high to be matched")
	}
	if IsFailOnMatched(issues, FailOnNever) {
		t.Fatalf("fail-on never should not match")
	}
	if IsFailOnMatched([]model.Issue{{Severity: model.SeverityLow}}, model.SeverityMedium) {
		t.Fatalf("fail-on medium should not match low severity")
	}
}

func TestResolveDoctorExitCode(t *testing.T) {
	if got := ResolveDoctorExitCode(nil, model.SeverityHigh); got != errs.ExitSuccess {
		t.Fatalf("empty issues exit code want=%d got=%d", errs.ExitSuccess, got)
	}
	if got := ResolveDoctorExitCode([]model.Issue{{Severity: model.SeverityLow}}, model.SeverityHigh); got != errs.ExitWarning {
		t.Fatalf("warning issues exit code want=%d got=%d", errs.ExitWarning, got)
	}
	if got := ResolveDoctorExitCode([]model.Issue{{Severity: model.SeverityCritical}}, model.SeverityHigh); got != errs.ExitFailOnMatched {
		t.Fatalf("fail-on matched exit code want=%d got=%d", errs.ExitFailOnMatched, got)
	}
}

func TestLevelBoundaries(t *testing.T) {
	scorer := NewScorer()
	tests := []struct {
		name   string
		issues []model.Issue
		level  string
	}{
		{
			name:   "healthy boundary score 90",
			issues: []model.Issue{{Severity: model.SeverityHigh, Confidence: 100, Scope: model.ScopeLocal}},
			level:  "healthy",
		},
		{
			name: "degraded score 89",
			issues: []model.Issue{
				{Severity: model.SeverityHigh, Confidence: 100, Scope: model.ScopeLocal},
				{Severity: model.SeverityLow, Confidence: 50, Scope: model.ScopeLocal},
			},
			level: "degraded",
		},
		{
			name: "warning boundary score 60",
			issues: []model.Issue{
				{Severity: model.SeverityCritical, Confidence: 100, Scope: model.ScopeLocal},
				{Severity: model.SeverityCritical, Confidence: 100, Scope: model.ScopeLocal},
			},
			level: "warning",
		},
		{
			name: "critical score below 60",
			issues: []model.Issue{
				{Severity: model.SeverityCritical, Confidence: 100, Scope: model.ScopeLocal},
				{Severity: model.SeverityCritical, Confidence: 100, Scope: model.ScopeLocal},
				{Severity: model.SeverityLow, Confidence: 100, Scope: model.ScopeLocal},
			},
			level: "critical",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, level := scorer.Score(tc.issues, 100)
			if level != tc.level {
				t.Fatalf("level want=%s got=%s", tc.level, level)
			}
		})
	}
}

func TestIsFailOnMatchedInvalidValueFallsBackToHigh(t *testing.T) {
	issues := []model.Issue{
		{Severity: model.SeverityMedium},
		{Severity: model.SeverityHigh},
	}
	if !IsFailOnMatched(issues, "bad-threshold") {
		t.Fatal("invalid fail-on should fallback to high and be matched by high issue")
	}
}

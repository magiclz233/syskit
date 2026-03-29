package doctor

import (
	"testing"

	dnscollector "syskit/internal/collectors/dns"
	networkprobe "syskit/internal/collectors/networkprobe"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
)

func TestAnalyzePingStageCritical(t *testing.T) {
	issue := analyzePingStage("1.1.1.1", &networkprobe.PingResult{
		Target:       "1.1.1.1",
		Count:        3,
		SuccessCount: 0,
		FailureCount: 3,
		LossRate:     100,
	})
	if issue == nil {
		t.Fatal("analyzePingStage() = nil, want critical issue")
	}
	if issue.RuleID != "NET-003" || issue.Severity != model.SeverityCritical {
		t.Fatalf("issue = %+v, want NET-003 critical", issue)
	}
}

func TestAnalyzeTracerouteStageGatewayTimeout(t *testing.T) {
	issues := analyzeTracerouteStage("1.1.1.1", &networkprobe.TracerouteResult{
		Target:   "1.1.1.1",
		HopCount: 2,
		Reached:  false,
		Hops: []networkprobe.TracerouteHop{
			{Hop: 1, Timeout: true},
			{Hop: 2, Timeout: true},
		},
	})
	if len(issues) != 2 {
		t.Fatalf("issues len = %d, want 2", len(issues))
	}
}

func TestAnalyzeDNSStageSlow(t *testing.T) {
	issue := analyzeDNSStage("example.com", &dnscollector.ResolveResult{
		Domain:    "example.com",
		QueryType: "A",
		Count:     1,
	}, &dnscollector.BenchResult{
		Domain:       "example.com",
		QueryType:    "A",
		Count:        3,
		SuccessCount: 3,
		FailureCount: 0,
		AvgMs:        280,
		MaxMs:        300,
		MinMs:        250,
	})
	if issue == nil {
		t.Fatal("analyzeDNSStage() = nil, want medium issue")
	}
	if issue.RuleID != "NET-001" || issue.Severity != model.SeverityMedium {
		t.Fatalf("issue = %+v, want NET-001 medium", issue)
	}
}

func TestClassifyNetworkStageErrorDependencyMissing(t *testing.T) {
	stageIssue, stageSkipped := classifyNetworkStageError("ping", errs.NewWithSuggestion(
		errs.ExitExecutionFailed,
		errs.CodeDependencyMissing,
		"未找到系统命令: ping",
		"请确认 ping 可执行并在 PATH 中",
	))
	if stageIssue != nil {
		t.Fatalf("stageIssue = %+v, want nil", stageIssue)
	}
	if stageSkipped == nil {
		t.Fatal("stageSkipped = nil, want unsupported skipped")
	}
	if stageSkipped.Reason != model.SkipReasonUnsupported {
		t.Fatalf("stageSkipped.Reason = %s, want %s", stageSkipped.Reason, model.SkipReasonUnsupported)
	}
}

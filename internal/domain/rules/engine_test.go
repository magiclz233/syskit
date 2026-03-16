package rules

import (
	"context"
	"errors"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"testing"
)

type fakeRule struct {
	id     string
	phase  string
	module string
	issue  *model.Issue
	err    error
	calls  *int
}

func (r fakeRule) ID() string {
	return r.id
}

func (r fakeRule) Phase() string {
	return r.phase
}

func (r fakeRule) Module() string {
	return r.module
}

func (r fakeRule) Check(ctx context.Context, in DiagnoseInput) (*model.Issue, error) {
	if r.calls != nil {
		*r.calls++
	}
	return r.issue, r.err
}

type fakeRuleWithoutModule struct {
	id    string
	phase string
	err   error
}

func (r fakeRuleWithoutModule) ID() string {
	return r.id
}

func (r fakeRuleWithoutModule) Phase() string {
	return r.phase
}

func (r fakeRuleWithoutModule) Check(ctx context.Context, in DiagnoseInput) (*model.Issue, error) {
	return nil, r.err
}

func TestEvaluateSortAndEnabled(t *testing.T) {
	engine := NewEngine(
		fakeRule{
			id:     "PORT-001",
			phase:  "P0",
			module: "port",
			issue: &model.Issue{
				Severity:   "critical",
				Summary:    "端口冲突",
				Confidence: 80,
				Scope:      "system",
			},
		},
		fakeRule{
			id:     "MEM-001",
			phase:  "P0",
			module: "mem",
			issue: &model.Issue{
				Severity: "high",
				Summary:  "内存告警",
			},
		},
		fakeRule{
			id:     "ENV-001",
			phase:  "P0",
			module: "env",
			issue: &model.Issue{
				Severity: "low",
				Summary:  "PATH 重复",
			},
		},
	)

	got, err := engine.Evaluate(context.Background(), DiagnoseInput{}, []string{"MEM-001", "PORT-001"})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if len(got.Issues) != 2 {
		t.Fatalf("issues want=2 got=%d", len(got.Issues))
	}
	if got.Issues[0].RuleID != "PORT-001" || got.Issues[1].RuleID != "MEM-001" {
		t.Fatalf("unexpected issue order: %+v", got.Issues)
	}
}

func TestEvaluateModuleDegradeSkipsFollowingRules(t *testing.T) {
	callPort1 := 0
	callPort2 := 0
	callCPU := 0

	engine := NewEngine(
		fakeRule{
			id:     "PORT-001",
			phase:  "P0",
			module: "port",
			err:    errs.PermissionDenied("需要管理员权限", "请提升权限"),
			calls:  &callPort1,
		},
		fakeRule{
			id:     "PORT-002",
			phase:  "P0",
			module: "port",
			issue: &model.Issue{
				Severity: "high",
				Summary:  "公网监听",
			},
			calls: &callPort2,
		},
		fakeRule{
			id:     "CPU-001",
			phase:  "P0",
			module: "cpu",
			issue: &model.Issue{
				Severity: "high",
				Summary:  "CPU 过高",
			},
			calls: &callCPU,
		},
	)

	got, err := engine.Evaluate(context.Background(), DiagnoseInput{}, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if callPort1 != 1 {
		t.Fatalf("PORT-001 calls want=1 got=%d", callPort1)
	}
	if callPort2 != 0 {
		t.Fatalf("PORT-002 should be skipped after module degrade, calls=%d", callPort2)
	}
	if callCPU != 1 {
		t.Fatalf("CPU-001 calls want=1 got=%d", callCPU)
	}

	if len(got.Issues) != 1 || got.Issues[0].RuleID != "CPU-001" {
		t.Fatalf("unexpected issues: %+v", got.Issues)
	}
	if len(got.Skipped) != 1 {
		t.Fatalf("skipped want=1 got=%d", len(got.Skipped))
	}
	if got.Skipped[0].Module != "port" || got.Skipped[0].Reason != model.SkipReasonPermissionDenied {
		t.Fatalf("unexpected skipped item: %+v", got.Skipped[0])
	}
}

func TestEvaluateRespectsInputSkipped(t *testing.T) {
	callMem := 0
	callDisk := 0

	engine := NewEngine(
		fakeRule{
			id:     "MEM-001",
			phase:  "P0",
			module: "mem",
			issue: &model.Issue{
				Severity: "high",
				Summary:  "内存不足",
			},
			calls: &callMem,
		},
		fakeRule{
			id:     "DISK-001",
			phase:  "P0",
			module: "disk",
			issue: &model.Issue{
				Severity: "critical",
				Summary:  "磁盘写满",
			},
			calls: &callDisk,
		},
	)

	in := DiagnoseInput{
		Skipped: []model.SkippedModule{
			{Module: "mem", Reason: model.SkipReasonTimeout},
		},
	}

	got, err := engine.Evaluate(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if callMem != 0 {
		t.Fatalf("MEM-001 should be skipped, calls=%d", callMem)
	}
	if callDisk != 1 {
		t.Fatalf("DISK-001 calls want=1 got=%d", callDisk)
	}
	if len(got.Issues) != 1 || got.Issues[0].RuleID != "DISK-001" {
		t.Fatalf("unexpected issues: %+v", got.Issues)
	}
	if len(got.Skipped) != 1 || got.Skipped[0].Module != "mem" {
		t.Fatalf("unexpected skipped: %+v", got.Skipped)
	}
}

func TestEvaluateFailsOnUnknownError(t *testing.T) {
	engine := NewEngine(
		fakeRule{
			id:     "FILE-001",
			phase:  "P0",
			module: "file",
			err:    errors.New("collector exploded"),
		},
	)

	_, err := engine.Evaluate(context.Background(), DiagnoseInput{}, nil)
	if err == nil {
		t.Fatalf("Evaluate should fail on unknown error")
	}
	if errs.ErrorCode(err) != errs.CodeExecutionFailed {
		t.Fatalf("error code want=%s got=%s", errs.CodeExecutionFailed, errs.ErrorCode(err))
	}
}

func TestEvaluateNormalizeIssueAndModuleFallback(t *testing.T) {
	engine := NewEngine(
		fakeRuleWithoutModule{
			id:    "ENV-001",
			phase: "P0",
			err:   errs.New(errs.ExitExecutionFailed, errs.CodePlatformUnsupported, "平台不支持"),
		},
	)

	got, err := engine.Evaluate(context.Background(), DiagnoseInput{}, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if len(got.Skipped) != 1 {
		t.Fatalf("skipped want=1 got=%d", len(got.Skipped))
	}
	if got.Skipped[0].Module != unknownModule || got.Skipped[0].Reason != model.SkipReasonUnsupported {
		t.Fatalf("unexpected skipped item: %+v", got.Skipped[0])
	}
}

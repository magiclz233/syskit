package fixrun

import (
	"context"
	"testing"

	"syskit/internal/config"
)

func TestBuildPlan(t *testing.T) {
	plan, err := BuildPlan("cleanup-temp, echo hello", false, "continue")
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(plan.Steps))
	}
	if !plan.Steps[0].Builtin {
		t.Fatalf("step0 builtin = false, want true")
	}
}

func TestExecuteDryRun(t *testing.T) {
	cfg := config.DefaultConfig()
	plan, err := BuildPlan("cleanup-temp", false, "stop")
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	result, err := Execute(context.Background(), plan, cfg)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("result.Success = false, want true")
	}
}

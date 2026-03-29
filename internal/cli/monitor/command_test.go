package monitor

import (
	"testing"
	"time"

	"syskit/internal/config"
)

func TestNormalizeOptionsDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	normalized, warnings, err := normalizeOptions(&allOptions{}, cfg, "medium")
	if err != nil {
		t.Fatalf("normalizeOptions() error = %v", err)
	}
	if normalized.interval != time.Duration(cfg.Monitor.IntervalSec)*time.Second {
		t.Fatalf("interval = %s, want %ds", normalized.interval, cfg.Monitor.IntervalSec)
	}
	if normalized.maxSamples != cfg.Monitor.MaxSamples {
		t.Fatalf("maxSamples = %d, want %d", normalized.maxSamples, cfg.Monitor.MaxSamples)
	}
	if normalized.inspectionFailOn != "medium" {
		t.Fatalf("inspectionFailOn = %s, want medium", normalized.inspectionFailOn)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want empty", warnings)
	}
}

func TestNormalizeOptionsInvalidInterval(t *testing.T) {
	cfg := config.DefaultConfig()
	_, _, err := normalizeOptions(&allOptions{interval: "bad"}, cfg, "high")
	if err == nil {
		t.Fatal("normalizeOptions() error = nil, want invalid argument")
	}
}

func TestNormalizeOptionsInvalidInspectionFailOn(t *testing.T) {
	cfg := config.DefaultConfig()
	_, _, err := normalizeOptions(&allOptions{inspectionFailOn: "bad"}, cfg, "high")
	if err == nil {
		t.Fatal("normalizeOptions() error = nil, want invalid argument")
	}
}

func TestEvaluateSingleAlert(t *testing.T) {
	consecutive := map[string]int{}
	stats := map[string]*alertAccumulator{}
	now := time.Now().UTC()

	if got := evaluateSingleAlert(now, "cpu", "cpu", "cpu", 90, 80, 2, consecutive, stats); len(got) != 0 {
		t.Fatalf("first hit triggered = %d, want 0", len(got))
	}
	if got := evaluateSingleAlert(now.Add(time.Second), "cpu", "cpu", "cpu", 91, 80, 2, consecutive, stats); len(got) != 1 {
		t.Fatalf("second hit triggered = %d, want 1", len(got))
	}
	if got := evaluateSingleAlert(now.Add(2*time.Second), "cpu", "cpu", "cpu", 92, 80, 2, consecutive, stats); len(got) != 0 {
		t.Fatalf("third hit triggered = %d, want 0", len(got))
	}
}

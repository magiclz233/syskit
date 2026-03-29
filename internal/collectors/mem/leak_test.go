package mem

import (
	"context"
	"testing"
	"time"
)

func TestNormalizeLeakOptions(t *testing.T) {
	normalized, err := normalizeLeakOptions(LeakOptions{
		PID:      100,
		Duration: 30 * time.Second,
		Interval: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("normalizeLeakOptions(valid) error = %v", err)
	}
	if normalized.PID != 100 {
		t.Fatalf("PID = %d, want 100", normalized.PID)
	}
	if normalized.Duration != 30*time.Second {
		t.Fatalf("Duration = %s, want 30s", normalized.Duration)
	}
	if normalized.Interval != 2*time.Second {
		t.Fatalf("Interval = %s, want 2s", normalized.Interval)
	}

	if _, err := normalizeLeakOptions(LeakOptions{PID: 0}); err == nil {
		t.Fatal("normalizeLeakOptions(PID=0) should fail")
	}
	if _, err := normalizeLeakOptions(LeakOptions{PID: 1, Duration: time.Second, Interval: 2 * time.Second}); err == nil {
		t.Fatal("duration < interval should fail")
	}
}

func TestClassifyLeakRisk(t *testing.T) {
	if level, _ := classifyLeakRisk(0, 0, 5); level != "low" {
		t.Fatalf("classifyLeakRisk(no growth) = %s, want low", level)
	}
	if level, _ := classifyLeakRisk(300*1024*1024, 6, 5); level != "medium" {
		t.Fatalf("classifyLeakRisk(medium) = %s, want medium", level)
	}
	if level, _ := classifyLeakRisk(1200*1024*1024, 30, 5); level != "high" {
		t.Fatalf("classifyLeakRisk(high) = %s, want high", level)
	}
	if level, _ := classifyLeakRisk(100*1024*1024, 2, 2); level != "low" {
		t.Fatalf("classifyLeakRisk(insufficient samples) = %s, want low", level)
	}
}

func TestLeakStopReason(t *testing.T) {
	if got := leakStopReason(context.DeadlineExceeded); got != "timeout" {
		t.Fatalf("leakStopReason(timeout) = %s, want timeout", got)
	}
	if got := leakStopReason(context.Canceled); got != "canceled" {
		t.Fatalf("leakStopReason(canceled) = %s, want canceled", got)
	}
	if got := leakStopReason(nil); got != "completed" {
		t.Fatalf("leakStopReason(nil) = %s, want completed", got)
	}
}

package cpu

import (
	"context"
	"testing"
	"time"
)

func TestNormalizeWatchOptions(t *testing.T) {
	normalized, err := normalizeWatchOptions(WatchOptions{})
	if err != nil {
		t.Fatalf("normalizeWatchOptions(default) error = %v", err)
	}
	if normalized.TopN != defaultWatchTopN {
		t.Fatalf("TopN = %d, want %d", normalized.TopN, defaultWatchTopN)
	}
	if normalized.Interval != defaultWatchInterval {
		t.Fatalf("Interval = %s, want %s", normalized.Interval, defaultWatchInterval)
	}
	if normalized.ThresholdCPU != defaultWatchThresholdCPU {
		t.Fatalf("ThresholdCPU = %f, want %f", normalized.ThresholdCPU, defaultWatchThresholdCPU)
	}

	if _, err := normalizeWatchOptions(WatchOptions{ThresholdLoad: -1}); err == nil {
		t.Fatal("normalizeWatchOptions with negative threshold-load should fail")
	}
}

func TestBuildWatchAlerts(t *testing.T) {
	base := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)
	alertMap := map[string]*watchAlertAccumulator{
		"system_load": {
			typ:         "system_load",
			threshold:   4,
			peakValue:   8.2,
			occurrences: 3,
			firstSeenAt: base,
			lastSeenAt:  base.Add(2 * time.Second),
		},
		"process_cpu:1024": {
			typ:         "process_cpu",
			threshold:   80,
			peakValue:   130.4,
			occurrences: 5,
			pid:         1024,
			processName: "go-test",
			command:     "go test ./...",
			firstSeenAt: base.Add(time.Second),
			lastSeenAt:  base.Add(3 * time.Second),
		},
	}

	alerts := buildWatchAlerts(alertMap)
	if len(alerts) != 2 {
		t.Fatalf("len(alerts) = %d, want 2", len(alerts))
	}
	if alerts[0].Type != "process_cpu" {
		t.Fatalf("first alert type = %s, want process_cpu", alerts[0].Type)
	}
	if alerts[0].Occurrences != 5 {
		t.Fatalf("first alert occurrences = %d, want 5", alerts[0].Occurrences)
	}
}

func TestWatchStopReason(t *testing.T) {
	if got := watchStopReason(context.DeadlineExceeded); got != "timeout" {
		t.Fatalf("watchStopReason(timeout) = %s, want timeout", got)
	}
	if got := watchStopReason(context.Canceled); got != "canceled" {
		t.Fatalf("watchStopReason(canceled) = %s, want canceled", got)
	}
	if got := watchStopReason(nil); got != "completed" {
		t.Fatalf("watchStopReason(nil) = %s, want completed", got)
	}
}

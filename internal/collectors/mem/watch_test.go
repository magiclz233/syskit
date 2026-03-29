package mem

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
	if normalized.ThresholdMem != defaultWatchThresholdMem {
		t.Fatalf("ThresholdMem = %f, want %f", normalized.ThresholdMem, defaultWatchThresholdMem)
	}
	if normalized.ThresholdSwap != defaultWatchThresholdSwap {
		t.Fatalf("ThresholdSwap = %f, want %f", normalized.ThresholdSwap, defaultWatchThresholdSwap)
	}
}

func TestBuildWatchAlerts(t *testing.T) {
	base := time.Date(2026, time.March, 29, 15, 0, 0, 0, time.UTC)
	alertMap := map[string]*watchAlertAccumulator{
		"system_mem": {
			typ:         "system_mem",
			threshold:   90,
			peakValue:   97,
			occurrences: 3,
			firstSeenAt: base,
			lastSeenAt:  base.Add(time.Second),
		},
		"process_mem:1": {
			typ:         "process_mem",
			threshold:   90,
			peakValue:   99,
			occurrences: 5,
			pid:         1,
			processName: "java",
			firstSeenAt: base,
			lastSeenAt:  base.Add(2 * time.Second),
		},
	}

	alerts := buildWatchAlerts(alertMap)
	if len(alerts) != 2 {
		t.Fatalf("len(alerts) = %d, want 2", len(alerts))
	}
	if alerts[0].Type != "process_mem" {
		t.Fatalf("first type = %s, want process_mem", alerts[0].Type)
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

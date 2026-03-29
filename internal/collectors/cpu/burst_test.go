package cpu

import (
	"testing"
	"time"
)

func TestNormalizeBurstOptions(t *testing.T) {
	normalized, err := normalizeBurstOptions(BurstOptions{
		Interval:         200 * time.Millisecond,
		Duration:         2 * time.Second,
		ThresholdPercent: 60,
	})
	if err != nil {
		t.Fatalf("normalizeBurstOptions(valid) error = %v", err)
	}
	if normalized.Interval != 200*time.Millisecond {
		t.Fatalf("interval = %s, want 200ms", normalized.Interval)
	}
	if normalized.Duration != 2*time.Second {
		t.Fatalf("duration = %s, want 2s", normalized.Duration)
	}
	if normalized.ThresholdPercent != 60 {
		t.Fatalf("threshold = %f, want 60", normalized.ThresholdPercent)
	}

	continuous, err := normalizeBurstOptions(BurstOptions{
		Interval:         500 * time.Millisecond,
		Duration:         0,
		ThresholdPercent: 0,
	})
	if err != nil {
		t.Fatalf("normalizeBurstOptions(continuous) error = %v", err)
	}
	if continuous.Duration != 0 {
		t.Fatalf("continuous duration = %s, want 0", continuous.Duration)
	}
	if continuous.ThresholdPercent != defaultBurstThreshold {
		t.Fatalf("threshold = %f, want %f", continuous.ThresholdPercent, defaultBurstThreshold)
	}

	if _, err := normalizeBurstOptions(BurstOptions{
		Interval: 500 * time.Millisecond,
		Duration: 100 * time.Millisecond,
	}); err == nil {
		t.Fatal("duration < interval should return invalid argument")
	}
}

func TestBuildBurstProcessesSortAndDuration(t *testing.T) {
	base := time.Date(2026, time.March, 29, 10, 0, 0, 0, time.UTC)
	items := map[int32]*burstAccumulator{
		1001: {
			meta:     burstMeta{name: "worker-a", user: "svc", command: "run-a"},
			hits:     3,
			sum:      240,
			peak:     90,
			peakAt:   base.Add(2 * time.Second),
			firstHit: base,
			lastHit:  base.Add(2 * time.Second),
		},
		1002: {
			meta:     burstMeta{name: "worker-b", user: "svc", command: "run-b"},
			hits:     1,
			sum:      85,
			peak:     85,
			peakAt:   base.Add(time.Second),
			firstHit: base.Add(time.Second),
			lastHit:  base.Add(time.Second),
		},
	}

	processes := buildBurstProcesses(items, time.Second)
	if len(processes) != 2 {
		t.Fatalf("len(processes) = %d, want 2", len(processes))
	}
	if processes[0].PID != 1001 {
		t.Fatalf("first pid = %d, want 1001", processes[0].PID)
	}
	if processes[0].AvgCPUPercent != 80 {
		t.Fatalf("avg cpu = %f, want 80", processes[0].AvgCPUPercent)
	}
	if processes[0].DurationSec != 3 {
		t.Fatalf("duration = %f, want 3", processes[0].DurationSec)
	}
	if processes[1].DurationSec != 1 {
		t.Fatalf("single-hit duration = %f, want 1", processes[1].DurationSec)
	}
}

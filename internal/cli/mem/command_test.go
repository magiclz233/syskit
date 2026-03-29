package mem

import (
	"testing"
	"time"
)

func TestTopOptionsDefault(t *testing.T) {
	cmd := newTopCommand()
	flags := cmd.Flags()

	by, err := flags.GetString("by")
	if err != nil {
		t.Fatalf("read --by failed: %v", err)
	}
	if by != "rss" {
		t.Fatalf("default --by should be rss, got %s", by)
	}

	topN, err := flags.GetInt("top")
	if err != nil {
		t.Fatalf("read --top failed: %v", err)
	}
	if topN != 20 {
		t.Fatalf("default --top should be 20, got %d", topN)
	}
}

func TestLeakCommandDefaults(t *testing.T) {
	cmd := newLeakCommand()
	flags := cmd.Flags()

	duration, err := flags.GetString("duration")
	if err != nil {
		t.Fatalf("read --duration failed: %v", err)
	}
	if duration != "30m" {
		t.Fatalf("default --duration should be 30m, got %s", duration)
	}

	interval, err := flags.GetString("interval")
	if err != nil {
		t.Fatalf("read --interval failed: %v", err)
	}
	if interval != "10s" {
		t.Fatalf("default --interval should be 10s, got %s", interval)
	}
}

func TestParseLeakDurationAndInterval(t *testing.T) {
	duration, err := parseLeakDuration("60")
	if err != nil {
		t.Fatalf("parseLeakDuration(60) error = %v", err)
	}
	if duration != time.Minute {
		t.Fatalf("parseLeakDuration(60) = %s, want 1m", duration)
	}

	interval, err := parseLeakInterval("500ms")
	if err != nil {
		t.Fatalf("parseLeakInterval(500ms) error = %v", err)
	}
	if interval != 500*time.Millisecond {
		t.Fatalf("parseLeakInterval(500ms) = %s, want 500ms", interval)
	}

	if _, err := parseLeakDuration("0"); err == nil {
		t.Fatal("parseLeakDuration(0) should fail")
	}
	if _, err := parseLeakInterval("-1s"); err == nil {
		t.Fatal("parseLeakInterval(-1s) should fail")
	}
}

func TestWatchCommandDefaults(t *testing.T) {
	cmd := newWatchCommand()
	flags := cmd.Flags()

	top, err := flags.GetInt("top")
	if err != nil {
		t.Fatalf("read --top failed: %v", err)
	}
	if top != 10 {
		t.Fatalf("default --top should be 10, got %d", top)
	}

	interval, err := flags.GetString("interval")
	if err != nil {
		t.Fatalf("read --interval failed: %v", err)
	}
	if interval != "5s" {
		t.Fatalf("default --interval should be 5s, got %s", interval)
	}

	thresholdMem, err := flags.GetFloat64("threshold-mem")
	if err != nil {
		t.Fatalf("read --threshold-mem failed: %v", err)
	}
	if thresholdMem != 90 {
		t.Fatalf("default --threshold-mem should be 90, got %f", thresholdMem)
	}

	thresholdSwap, err := flags.GetFloat64("threshold-swap")
	if err != nil {
		t.Fatalf("read --threshold-swap failed: %v", err)
	}
	if thresholdSwap != 50 {
		t.Fatalf("default --threshold-swap should be 50, got %f", thresholdSwap)
	}
}

package cpu

import (
	"testing"
	"time"
)

func TestCPUCommandDefaultsAndArgs(t *testing.T) {
	cmd := NewCommand()
	detail, err := cmd.Flags().GetBool("detail")
	if err != nil {
		t.Fatalf("read --detail failed: %v", err)
	}
	if detail {
		t.Fatal("default --detail should be false")
	}

	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Fatal("cpu command should reject extra args")
	}
}

func TestCPUBurstCommandDefaults(t *testing.T) {
	cmd := newBurstCommand()
	flags := cmd.Flags()

	interval, err := flags.GetString("interval")
	if err != nil {
		t.Fatalf("read --interval failed: %v", err)
	}
	if interval != "500ms" {
		t.Fatalf("default --interval should be 500ms, got %s", interval)
	}

	duration, err := flags.GetString("duration")
	if err != nil {
		t.Fatalf("read --duration failed: %v", err)
	}
	if duration != "10s" {
		t.Fatalf("default --duration should be 10s, got %s", duration)
	}

	threshold, err := flags.GetFloat64("threshold")
	if err != nil {
		t.Fatalf("read --threshold failed: %v", err)
	}
	if threshold != 50 {
		t.Fatalf("default --threshold should be 50, got %f", threshold)
	}
}

func TestParseBurstDurationAndInterval(t *testing.T) {
	interval, err := parseBurstInterval("250ms")
	if err != nil {
		t.Fatalf("parseBurstInterval(250ms) error = %v", err)
	}
	if interval != 250*time.Millisecond {
		t.Fatalf("parseBurstInterval(250ms) = %s, want 250ms", interval)
	}

	duration, err := parseBurstDuration("5")
	if err != nil {
		t.Fatalf("parseBurstDuration(5) error = %v", err)
	}
	if duration != 5*time.Second {
		t.Fatalf("parseBurstDuration(5) = %s, want 5s", duration)
	}

	duration, err = parseBurstDuration("0")
	if err != nil {
		t.Fatalf("parseBurstDuration(0) error = %v", err)
	}
	if duration != 0 {
		t.Fatalf("parseBurstDuration(0) = %s, want 0", duration)
	}

	if _, err := parseBurstInterval("0"); err == nil {
		t.Fatal("parseBurstInterval(0) should return invalid argument")
	}
	if _, err := parseBurstDuration("-1s"); err == nil {
		t.Fatal("parseBurstDuration(-1s) should return invalid argument")
	}
}

func TestCPUWatchCommandDefaults(t *testing.T) {
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
	if interval != "1s" {
		t.Fatalf("default --interval should be 1s, got %s", interval)
	}

	thresholdCPU, err := flags.GetFloat64("threshold-cpu")
	if err != nil {
		t.Fatalf("read --threshold-cpu failed: %v", err)
	}
	if thresholdCPU != 80 {
		t.Fatalf("default --threshold-cpu should be 80, got %f", thresholdCPU)
	}

	thresholdLoad, err := flags.GetFloat64("threshold-load")
	if err != nil {
		t.Fatalf("read --threshold-load failed: %v", err)
	}
	if thresholdLoad != 0 {
		t.Fatalf("default --threshold-load should be 0, got %f", thresholdLoad)
	}
}

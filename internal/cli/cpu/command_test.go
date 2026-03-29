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

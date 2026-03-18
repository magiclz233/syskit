package cpu

import "testing"

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

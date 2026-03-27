package ping

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewCommandBasic(t *testing.T) {
	cmd := NewCommand()
	if cmd.Name() != "ping" {
		t.Fatalf("name = %s, want ping", cmd.Name())
	}
	if cmd.Long == "" || cmd.Example == "" {
		t.Fatalf("Long/Example should not be empty")
	}
}

func TestRunPingRejectsInvalidCount(t *testing.T) {
	err := runPing(&cobra.Command{}, "localhost", &pingOptions{count: 0, size: 32})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRunPingRejectsInvalidSize(t *testing.T) {
	err := runPing(&cobra.Command{}, "localhost", &pingOptions{count: 1, size: 0})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

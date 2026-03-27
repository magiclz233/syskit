package dns

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewCommandHasSubcommands(t *testing.T) {
	cmd := NewCommand()
	for _, name := range []string{"resolve", "bench"} {
		if findSubCommand(cmd, name) == nil {
			t.Fatalf("subcommand %s not found", name)
		}
	}
}

func TestRunResolveInvalidType(t *testing.T) {
	cmd := &cobra.Command{}
	if err := runResolve(cmd, "localhost", &resolveOptions{typ: "PTR"}); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRunBenchInvalidCount(t *testing.T) {
	cmd := &cobra.Command{}
	if err := runBench(cmd, "localhost", &benchOptions{typ: "A", count: 0}); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func findSubCommand(root *cobra.Command, name string) *cobra.Command {
	if root == nil {
		return nil
	}
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

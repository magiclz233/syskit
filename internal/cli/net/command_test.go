package net

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewCommandHasExpectedSubcommands(t *testing.T) {
	cmd := NewCommand()
	for _, want := range []string{"conn", "listen", "speed"} {
		if findSubCommand(cmd, want) == nil {
			t.Fatalf("subcommand %s not found", want)
		}
	}
}

func TestRunConnRejectsNegativePID(t *testing.T) {
	err := runConn(&cobra.Command{}, &connOptions{pid: -1})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRunListenRejectsInvalidProtocol(t *testing.T) {
	err := runListen(&cobra.Command{}, &listenOptions{proto: "icmp"})
	if err == nil {
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

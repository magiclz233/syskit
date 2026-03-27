package traceroute

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewCommandBasic(t *testing.T) {
	cmd := NewCommand()
	if cmd.Name() != "traceroute" {
		t.Fatalf("name = %s, want traceroute", cmd.Name())
	}
	if cmd.Long == "" || cmd.Example == "" {
		t.Fatalf("Long/Example should not be empty")
	}
}

func TestRunTracerouteRejectsInvalidMaxHops(t *testing.T) {
	err := runTraceroute(&cobra.Command{}, "localhost", &traceOptions{maxHops: 0, proto: "icmp"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRunTracerouteRejectsInvalidProtocol(t *testing.T) {
	err := runTraceroute(&cobra.Command{}, "localhost", &traceOptions{maxHops: 5, proto: "udp"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

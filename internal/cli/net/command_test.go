package net

import (
	"strings"
	"testing"

	netcollector "syskit/internal/collectors/net"

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

func TestRunSpeedRejectsInvalidMode(t *testing.T) {
	err := runSpeed(&cobra.Command{}, &speedOptions{mode: "mixed"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestResolveSpeedTimeoutRejectsInvalid(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("timeout", "", "timeout")
	if err := cmd.Flags().Set("timeout", "bad"); err == nil {
		if _, parseErr := resolveSpeedTimeout(cmd); parseErr == nil {
			t.Fatalf("expected parse error, got nil")
		}
	}
}

func TestBuildSpeedMessageIncludesAssessmentSummary(t *testing.T) {
	message := buildSpeedMessage(&netcollector.SpeedResult{
		Mode: string(netcollector.SpeedModeFull),
		Ping: &netcollector.SpeedPingStats{AvgMs: 12.3},
		Download: &netcollector.SpeedSample{
			Mbps: 88.8,
		},
		Upload: &netcollector.SpeedSample{
			Mbps: 22.2,
		},
		Assessment: &netcollector.SpeedAssessment{
			Summary: "延迟良好；下载带宽良好；上传带宽一般",
		},
	})
	if !strings.Contains(message, "结论：延迟良好") {
		t.Fatalf("message should include assessment summary, got %q", message)
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

package port

import (
	"reflect"
	portcollector "syskit/internal/collectors/port"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestParseSinglePort(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "normal", raw: "8080", want: 8080},
		{name: "trim", raw: " 80 ", want: 80},
		{name: "zero", raw: "0", wantErr: true},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "overflow", raw: "70000", wantErr: true},
		{name: "invalid", raw: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSinglePort(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("want %d, got %d", tt.want, got)
			}
		})
	}
}

func TestResolveProbeTimeout(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("timeout", "", "timeout")

	got, err := resolveProbeTimeout(cmd, time.Second)
	if err != nil {
		t.Fatalf("resolveProbeTimeout default error = %v", err)
	}
	if got != time.Second {
		t.Fatalf("default timeout = %v, want 1s", got)
	}

	if err := cmd.Flags().Set("timeout", "2s"); err != nil {
		t.Fatalf("set timeout error = %v", err)
	}
	got, err = resolveProbeTimeout(cmd, time.Second)
	if err != nil {
		t.Fatalf("resolveProbeTimeout 2s error = %v", err)
	}
	if got != 2*time.Second {
		t.Fatalf("timeout = %v, want 2s", got)
	}

	if err := cmd.Flags().Set("timeout", "bad"); err == nil {
		_, parseErr := resolveProbeTimeout(cmd, time.Second)
		if parseErr == nil {
			t.Fatal("expected error for invalid timeout")
		}
	}
}

func TestResolveScanPorts(t *testing.T) {
	tests := []struct {
		name      string
		mode      portcollector.ScanMode
		raw       string
		wantPorts []int
		wantErr   bool
	}{
		{
			name:      "custom expression",
			mode:      portcollector.ScanModeQuick,
			raw:       "22,80,443",
			wantPorts: []int{22, 80, 443},
		},
		{
			name:      "quick default",
			mode:      portcollector.ScanModeQuick,
			raw:       "",
			wantPorts: quickScanDefaultPorts,
		},
		{
			name:      "full default",
			mode:      portcollector.ScanModeFull,
			raw:       "",
			wantPorts: []int{1, fullScanDefaultUpperPort},
		},
		{
			name:    "invalid expression",
			mode:    portcollector.ScanModeQuick,
			raw:     "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := resolveScanPorts(tt.mode, tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveScanPorts error = %v", err)
			}
			if tt.name == "full default" {
				if len(got) != fullScanDefaultUpperPort {
					t.Fatalf("full ports len = %d, want %d", len(got), fullScanDefaultUpperPort)
				}
				if got[0] != tt.wantPorts[0] || got[len(got)-1] != tt.wantPorts[1] {
					t.Fatalf("full default boundary = [%d,%d], want [%d,%d]", got[0], got[len(got)-1], tt.wantPorts[0], tt.wantPorts[1])
				}
				return
			}
			if !reflect.DeepEqual(got, tt.wantPorts) {
				t.Fatalf("ports = %v, want %v", got, tt.wantPorts)
			}
		})
	}
}

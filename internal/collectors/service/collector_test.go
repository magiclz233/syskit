package service

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestParseSystemdListUnits(t *testing.T) {
	output := []byte("cron.service loaded active running Regular background program processing daemon\n" +
		"dbus.service loaded inactive dead D-Bus System Message Bus\n")
	entries := parseSystemdListUnits(output)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Name != "cron.service" {
		t.Fatalf("entry[0].name = %s, want cron.service", entries[0].Name)
	}
	if entries[0].State != "running" {
		t.Fatalf("entry[0].state = %s, want running", entries[0].State)
	}
	if entries[1].State != "stopped" {
		t.Fatalf("entry[1].state = %s, want stopped", entries[1].State)
	}
}

func TestFilterServices(t *testing.T) {
	services := []ServiceEntry{
		{Name: "alpha", State: "running", Startup: "auto"},
		{Name: "beta", State: "stopped", Startup: "manual"},
		{Name: "gamma", State: "running", Startup: "disabled"},
	}
	stateSet := map[string]struct{}{"running": {}}
	startupSet := map[string]struct{}{"auto": {}}
	filtered := filterServices(services, stateSet, startupSet, "")
	if len(filtered) != 1 {
		t.Fatalf("filtered = %d, want 1", len(filtered))
	}
	if filtered[0].Name != "alpha" {
		t.Fatalf("filtered[0].name = %s, want alpha", filtered[0].Name)
	}
}

func TestListServicesDegradeWhenCommandMissing(t *testing.T) {
	originRuntime := runtimeName
	originRunner := commandRunner
	t.Cleanup(func() {
		runtimeName = originRuntime
		commandRunner = originRunner
	})

	runtimeName = "linux"
	commandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, exec.ErrNotFound
	}

	result, err := ListServices(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListServices() error = %v", err)
	}
	if result.Total != 0 {
		t.Fatalf("total = %d, want 0", result.Total)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("warnings is empty, want degrade warning")
	}
	if !strings.Contains(result.Warnings[0], "降级") {
		t.Fatalf("warning = %v, want contains 降级", result.Warnings)
	}
}

func TestCheckServiceInvalidName(t *testing.T) {
	_, err := CheckService(context.Background(), " ", CheckOptions{})
	if err == nil {
		t.Fatal("CheckService() error = nil, want invalid argument")
	}
}

func TestMapContextErrorTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := mapContextError(ctx, "x")
	if err == nil {
		t.Fatal("mapContextError() error = nil, want timeout/canceled")
	}
}

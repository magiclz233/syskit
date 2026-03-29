package startup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListItemsAndFilter(t *testing.T) {
	root := t.TempDir()
	userDir := filepath.Join(root, "user-startup")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	normal := filepath.Join(userDir, "normal.desktop")
	if err := os.WriteFile(normal, []byte("[Desktop Entry]\nExec=/usr/bin/demo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(normal) error = %v", err)
	}
	risky := filepath.Join(userDir, "risk.disabled")
	if err := os.WriteFile(risky, []byte("powershell -EncodedCommand x"), 0o644); err != nil {
		t.Fatalf("WriteFile(risky) error = %v", err)
	}

	oldRuntime := runtimeName
	oldResolver := resolveStartupDirs
	t.Cleanup(func() {
		runtimeName = oldRuntime
		resolveStartupDirs = oldResolver
	})
	runtimeName = "linux"
	resolveStartupDirs = func(goos string) []startupDir {
		return []startupDir{
			{path: userDir, location: "autostart", user: "tester"},
		}
	}

	result, err := ListItems(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("result.Total = %d, want 2", result.Total)
	}

	riskOnly, err := ListItems(context.Background(), ListOptions{OnlyRisk: true})
	if err != nil {
		t.Fatalf("ListItems(only-risk) error = %v", err)
	}
	if riskOnly.Total == 0 {
		t.Fatalf("riskOnly.Total = 0, want >0")
	}
	foundDisabled := false
	for _, item := range riskOnly.Items {
		if strings.Contains(item.SourcePath, "risk.disabled") {
			foundDisabled = true
			break
		}
	}
	if !foundDisabled {
		t.Fatalf("risk.disabled should be included in risk items: %+v", riskOnly.Items)
	}
}

func TestExecuteActionDisableAndEnable(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "demo.desktop")
	if err := os.WriteFile(target, []byte("Exec=/usr/bin/demo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	planDisable := &ActionPlan{
		Action:   ActionDisable,
		ID:       "demo",
		Platform: "linux",
		Found:    true,
		Current: Item{
			ID:         "demo",
			Name:       "demo",
			SourcePath: target,
			Enabled:    true,
		},
	}

	disabled, err := ExecuteAction(context.Background(), planDisable)
	if err != nil {
		t.Fatalf("ExecuteAction(disable) error = %v", err)
	}
	if !disabled.Success || disabled.After.Enabled {
		t.Fatalf("disable result unexpected: %+v", disabled)
	}
	if _, err := os.Stat(target + ".disabled"); err != nil {
		t.Fatalf("disabled file missing: %v", err)
	}

	planEnable := &ActionPlan{
		Action:   ActionEnable,
		ID:       disabled.After.ID,
		Platform: "linux",
		Found:    true,
		Current:  disabled.After,
	}
	enabled, err := ExecuteAction(context.Background(), planEnable)
	if err != nil {
		t.Fatalf("ExecuteAction(enable) error = %v", err)
	}
	if !enabled.Success || !enabled.After.Enabled {
		t.Fatalf("enable result unexpected: %+v", enabled)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("enabled file missing: %v", err)
	}
}

func TestParseAction(t *testing.T) {
	if _, err := ParseAction("enable"); err != nil {
		t.Fatalf("ParseAction(enable) error = %v", err)
	}
	if _, err := ParseAction("disable"); err != nil {
		t.Fatalf("ParseAction(disable) error = %v", err)
	}
	if _, err := ParseAction("bad"); err == nil {
		t.Fatal("ParseAction(bad) error = nil, want invalid argument")
	}
}

package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syskit/internal/errs"
	"testing"
)

func TestLoadPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	programData := filepath.Join(tempDir, "programdata")
	userHome := filepath.Join(tempDir, "userhome")

	systemPath := filepath.Join(programData, "syskit", "config.yaml")
	userPath := filepath.Join(userHome, ".syskit", "config.yaml")

	writeTestFile(t, systemPath, `
output:
  format: markdown
  no_color: true
risk:
  dry_run_default: false
`)
	writeTestFile(t, userPath, `
output:
  quiet: true
logging:
  level: debug
`)

	t.Setenv("ProgramData", programData)
	t.Setenv("USERPROFILE", userHome)
	t.Setenv("HOME", userHome)
	t.Setenv("SYSKIT_OUTPUT", "json")
	t.Setenv("SYSKIT_NO_COLOR", "false")
	t.Setenv("SYSKIT_DATA_DIR", filepath.Join(tempDir, "env-data"))
	t.Setenv("SYSKIT_LOG_LEVEL", "warn")
	t.Setenv("SYSKIT_CONFIG", "")

	result, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !slices.Equal(result.Paths, []string{systemPath, userPath}) {
		t.Fatalf("Load() paths = %v, want %v", result.Paths, []string{systemPath, userPath})
	}

	if got := result.Config.Output.Format; got != "json" {
		t.Fatalf("Output.Format = %q, want %q", got, "json")
	}
	if got := result.Config.Output.NoColor; got {
		t.Fatalf("Output.NoColor = %v, want false", got)
	}
	if got := result.Config.Output.Quiet; !got {
		t.Fatalf("Output.Quiet = %v, want true", got)
	}
	if got := result.Config.Risk.DryRunDefault; got {
		t.Fatalf("Risk.DryRunDefault = %v, want false", got)
	}
	if got := result.Config.Logging.Level; got != "warn" {
		t.Fatalf("Logging.Level = %q, want %q", got, "warn")
	}
	if got := result.Config.Storage.DataDir; got != filepath.Join(tempDir, "env-data") {
		t.Fatalf("Storage.DataDir = %q, want %q", got, filepath.Join(tempDir, "env-data"))
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	writeTestFile(t, configPath, `
output:
  format: invalid
`)

	_, err := Load(LoadOptions{ExplicitPath: configPath})
	if err == nil {
		t.Fatal("Load() error = nil, want config invalid error")
	}
	if got := errs.Code(err); got != errs.ExitInvalidArgument {
		t.Fatalf("errs.Code(err) = %d, want %d", got, errs.ExitInvalidArgument)
	}
	if got := errs.ErrorCode(err); got != errs.CodeConfigInvalid {
		t.Fatalf("errs.ErrorCode(err) = %q, want %q", got, errs.CodeConfigInvalid)
	}
	if !strings.Contains(errs.Message(err), "output.format") {
		t.Fatalf("errs.Message(err) = %q, want field name", errs.Message(err))
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

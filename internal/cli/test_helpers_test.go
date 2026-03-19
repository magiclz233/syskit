package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"syskit/internal/config"
	"syskit/internal/errs"
	policycfg "syskit/internal/policy"

	"gopkg.in/yaml.v3"
)

type cliResult struct {
	Stdout   string
	Stderr   string
	Err      error
	ExitCode int
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()

	app := newApplication("test-version")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app.rootCmd.SetOut(&stdout)
	app.rootCmd.SetErr(&stderr)
	app.rootCmd.SetIn(strings.NewReader(""))
	app.rootCmd.SetArgs(args)

	originalArgs := append([]string(nil), os.Args...)
	os.Args = append([]string{"syskit"}, args...)
	defer func() {
		os.Args = originalArgs
	}()

	err := app.rootCmd.Execute()
	if err != nil && !shouldSuppressErrorRender(err) {
		if renderErr := app.renderError(err); renderErr != nil {
			t.Fatalf("renderError() error = %v", renderErr)
		}
	}

	return cliResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Err:      err,
		ExitCode: exitCodeOf(err),
	}
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	return errs.Code(err)
}

func writeRuntimeConfig(t *testing.T, root string) (string, string, string) {
	t.Helper()

	dataDir := filepath.Join(root, "data")
	logFile := filepath.Join(root, "logs", "syskit.log")
	cfg := config.DefaultConfig()
	cfg.Storage.DataDir = dataDir
	cfg.Logging.File = logFile
	cfg.Output.NoColor = true

	payload, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal(config) error = %v", err)
	}

	path := filepath.Join(root, "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(config dir) error = %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	return path, dataDir, logFile
}

func writePolicyFile(t *testing.T, root string) string {
	t.Helper()

	payload, err := yaml.Marshal(policycfg.DefaultPolicy())
	if err != nil {
		t.Fatalf("yaml.Marshal(policy) error = %v", err)
	}

	path := filepath.Join(root, "policy.yaml")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}
	return path
}

func parseJSONResult(t *testing.T, raw string) map[string]any {
	t.Helper()

	data := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, raw=%s", err, raw)
	}
	return data
}

func mustMap(t *testing.T, value any, field string) map[string]any {
	t.Helper()
	item, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want map[string]any", field, value)
	}
	return item
}

func mustSlice(t *testing.T, value any, field string) []any {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("%s is %T, want []any", field, value)
	}
	return items
}

package cli

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"syskit/internal/domain/model"

	"github.com/spf13/cobra"
)

func TestCommandTreeMatchesSpec(t *testing.T) {
	app := newApplication("test-version")
	paths := collectCommandPaths(app.rootCmd)
	expected := []string{
		"syskit",
		"syskit cpu",
		"syskit cpu burst",
		"syskit cpu watch",
		"syskit disk",
		"syskit disk scan",
		"syskit dns",
		"syskit dns bench",
		"syskit dns resolve",
		"syskit doctor",
		"syskit doctor all",
		"syskit doctor cpu",
		"syskit doctor disk",
		"syskit doctor disk-full",
		"syskit doctor mem",
		"syskit doctor network",
		"syskit doctor port",
		"syskit doctor slowness",
		"syskit file",
		"syskit file archive",
		"syskit file dedup",
		"syskit file dup",
		"syskit file empty",
		"syskit fix",
		"syskit fix cleanup",
		"syskit fix run",
		"syskit log",
		"syskit log search",
		"syskit log watch",
		"syskit mem",
		"syskit mem leak",
		"syskit mem top",
		"syskit mem watch",
		"syskit monitor",
		"syskit monitor all",
		"syskit net",
		"syskit net conn",
		"syskit net listen",
		"syskit net speed",
		"syskit ping",
		"syskit policy",
		"syskit policy init",
		"syskit policy show",
		"syskit policy validate",
		"syskit port",
		"syskit port kill",
		"syskit port list",
		"syskit port ping",
		"syskit port scan",
		"syskit proc",
		"syskit proc info",
		"syskit proc kill",
		"syskit proc top",
		"syskit proc tree",
		"syskit report",
		"syskit report generate",
		"syskit service",
		"syskit service check",
		"syskit service disable",
		"syskit service enable",
		"syskit service list",
		"syskit service restart",
		"syskit service start",
		"syskit service stop",
		"syskit snapshot",
		"syskit snapshot create",
		"syskit snapshot delete",
		"syskit snapshot diff",
		"syskit snapshot list",
		"syskit snapshot show",
		"syskit startup",
		"syskit startup disable",
		"syskit startup enable",
		"syskit startup list",
		"syskit traceroute",
	}
	slices.Sort(expected)

	if !slices.Equal(paths, expected) {
		t.Fatalf("command tree mismatch\nwant: %v\ngot:  %v", expected, paths)
	}
}

func TestFormalHelpSurfaceRemovesLegacyScan(t *testing.T) {
	result := runCLI(t)
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	if strings.Contains(result.Stdout, "syskit [path]") {
		t.Fatalf("root help still exposes legacy path entry: %s", result.Stdout)
	}
	for _, text := range []string{"--top", "--include-files", "--include-dirs", "--export-csv"} {
		if strings.Contains(result.Stdout, text) {
			t.Fatalf("root help still exposes legacy scan flag %q", text)
		}
	}
	if strings.Contains(result.Stdout, "service list") {
		t.Fatalf("root help still uses pending command in examples: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "-v, --verbose") {
		t.Fatalf("root help does not expose expected verbose short flag: %s", result.Stdout)
	}
	if strings.Contains(result.Stdout, "-V, --verbose") {
		t.Fatalf("root help still exposes unexpected verbose short flag: %s", result.Stdout)
	}
}

func TestImplementedCommandsHaveLongAndExample(t *testing.T) {
	app := newApplication("test-version")
	required := [][]string{
		{"doctor"},
		{"doctor", "all"},
		{"doctor", "port"},
		{"doctor", "cpu"},
		{"doctor", "mem"},
		{"doctor", "disk"},
		{"doctor", "network"},
		{"doctor", "disk-full"},
		{"doctor", "slowness"},
		{"dns"},
		{"dns", "resolve"},
		{"dns", "bench"},
		{"port"},
		{"port", "list"},
		{"port", "ping"},
		{"port", "scan"},
		{"port", "kill"},
		{"net"},
		{"net", "conn"},
		{"net", "listen"},
		{"net", "speed"},
		{"ping"},
		{"traceroute"},
		{"proc"},
		{"proc", "top"},
		{"proc", "tree"},
		{"proc", "info"},
		{"proc", "kill"},
		{"cpu"},
		{"cpu", "burst"},
		{"cpu", "watch"},
		{"mem"},
		{"mem", "top"},
		{"mem", "leak"},
		{"mem", "watch"},
		{"monitor"},
		{"monitor", "all"},
		{"disk"},
		{"disk", "scan"},
		{"fix"},
		{"fix", "cleanup"},
		{"snapshot"},
		{"snapshot", "create"},
		{"snapshot", "list"},
		{"snapshot", "show"},
		{"snapshot", "diff"},
		{"snapshot", "delete"},
		{"report"},
		{"report", "generate"},
		{"service"},
		{"service", "list"},
		{"service", "check"},
		{"policy"},
		{"policy", "show"},
		{"policy", "init"},
		{"policy", "validate"},
	}

	for _, path := range required {
		cmd := mustFindCommand(t, app.rootCmd, path...)
		if strings.TrimSpace(cmd.Long) == "" {
			t.Fatalf("%s Long is empty", strings.Join(path, " "))
		}
		if strings.TrimSpace(cmd.Example) == "" {
			t.Fatalf("%s Example is empty", strings.Join(path, " "))
		}
	}
}

func TestDangerousHelpMentionsApplyAndYes(t *testing.T) {
	cases := []struct {
		args     []string
		expected []string
	}{
		{args: []string{"port", "kill", "--help"}, expected: []string{"--apply --yes", "审计"}},
		{args: []string{"proc", "kill", "--help"}, expected: []string{"--apply --yes", "审计"}},
		{args: []string{"snapshot", "delete", "--help"}, expected: []string{"--apply --yes", "审计"}},
	}

	for _, tc := range cases {
		result := runCLI(t, tc.args...)
		if result.ExitCode != 0 {
			t.Fatalf("%v exit code = %d, stderr=%s", tc.args, result.ExitCode, result.Stderr)
		}
		for _, want := range tc.expected {
			if !strings.Contains(result.Stdout, want) {
				t.Fatalf("%v help missing %q\n%s", tc.args, want, result.Stdout)
			}
		}
	}
}

func TestResultFieldContracts(t *testing.T) {
	assertJSONFieldSet(t, model.CommandResult{}, []string{"code", "msg", "data", "error", "metadata"})
	assertJSONFieldSet(t, model.Metadata{}, []string{"schema_version", "timestamp", "host", "command", "execution_ms", "platform", "trace_id"})
	assertJSONFieldSet(t, model.ErrorInfo{}, []string{"error_code", "error_message", "suggestion"})
	assertJSONFieldSet(t, model.Issue{}, []string{"rule_id", "severity", "summary", "evidence", "impact", "suggestion", "fix_command", "auto_fixable", "confidence", "scope"})
	assertJSONFieldSet(t, model.SkippedModule{}, []string{"module", "reason", "required_permission", "impact", "suggestion"})
}

func collectCommandPaths(root *cobra.Command) []string {
	paths := []string{root.CommandPath()}
	for _, cmd := range root.Commands() {
		if cmd.Hidden || cmd.Name() == "help" {
			continue
		}
		paths = append(paths, collectChildPaths(cmd)...)
	}
	slices.Sort(paths)
	return paths
}

func collectChildPaths(cmd *cobra.Command) []string {
	paths := []string{cmd.CommandPath()}
	for _, child := range cmd.Commands() {
		if child.Hidden || child.Name() == "help" {
			continue
		}
		paths = append(paths, collectChildPaths(child)...)
	}
	return paths
}

func mustFindCommand(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
	t.Helper()
	current := root
	for _, segment := range path {
		next := findCommand(current, segment)
		if next == nil {
			t.Fatalf("command %s not found", strings.Join(path, " "))
		}
		current = next
	}
	return current
}

func assertJSONFieldSet(t *testing.T, value any, expected []string) {
	t.Helper()
	typ := reflect.TypeOf(value)
	got := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		got = append(got, name)
	}
	slices.Sort(got)
	slices.Sort(expected)
	if !slices.Equal(got, expected) {
		t.Fatalf("%s json fields mismatch\nwant: %v\ngot:  %v", typ.Name(), expected, got)
	}
}

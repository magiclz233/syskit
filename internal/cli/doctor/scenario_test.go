package doctor

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"syskit/internal/scanner"
)

func TestNewCommandIncludesScenarioSubcommands(t *testing.T) {
	cmd := NewCommand()
	names := make([]string, 0, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		names = append(names, child.Name())
	}
	for _, want := range []string{"network", "disk-full", "slowness"} {
		if !slices.Contains(names, want) {
			t.Fatalf("doctor subcommand %s not found in %v", want, names)
		}
	}
}

func TestUniqueWarnings(t *testing.T) {
	got := uniqueWarnings([]string{"  beta ", "alpha", "beta", "", "alpha"})
	want := []string{"alpha", "beta"}
	if !slices.Equal(got, want) {
		t.Fatalf("uniqueWarnings() = %v, want %v", got, want)
	}
}

func TestToFileObservations(t *testing.T) {
	modTime := time.Now().UTC()
	got := toFileObservations([]scanner.FileInfo{
		{Path: filepath.Clean("/tmp/a.log"), Size: 128, ModTime: modTime},
	})
	if len(got) != 1 {
		t.Fatalf("len(toFileObservations) = %d, want 1", len(got))
	}
	item := got[0]
	if item.Path == "" || item.SizeBytes != 128 {
		t.Fatalf("observation = %#v", item)
	}
	if item.GrowthMBPerHour != 0 {
		t.Fatalf("GrowthMBPerHour = %v, want 0", item.GrowthMBPerHour)
	}
}

func TestEnabledRulesForSlownessQuickAndDeep(t *testing.T) {
	quick := enabledRulesForSlowness("quick")
	deep := enabledRulesForSlowness("deep")
	if len(quick) == 0 || len(deep) == 0 {
		t.Fatalf("enabled rules should not be empty: quick=%v deep=%v", quick, deep)
	}
	if !slices.Contains(deep, "DISK-002") {
		t.Fatalf("deep rules should contain DISK-002, got %v", deep)
	}
	if slices.Contains(quick, "DISK-002") {
		t.Fatalf("quick rules should not contain DISK-002, got %v", quick)
	}
}

func TestCoverageForScenario(t *testing.T) {
	if got := coverageOf(2, 1); got != 50 {
		t.Fatalf("coverageOf(2,1) = %v, want 50", got)
	}
}

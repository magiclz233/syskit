package snapshot

import (
	"slices"
	"syskit/internal/domain/model"
	"testing"
	"time"
)

func TestParseSnapshotModules(t *testing.T) {
	all, err := parseSnapshotModules("", true)
	if err != nil {
		t.Fatalf("parseSnapshotModules(all) error = %v", err)
	}
	if !slices.Equal(all, snapshotSupportedModules) {
		t.Fatalf("parseSnapshotModules(all) = %v, want %v", all, snapshotSupportedModules)
	}

	selected, err := parseSnapshotModules("mem,port,mem", false)
	if err != nil {
		t.Fatalf("parseSnapshotModules(selected) error = %v", err)
	}
	if !slices.Equal(selected, []string{"port", "mem"}) {
		t.Fatalf("parseSnapshotModules(selected) = %v, want [port mem]", selected)
	}

	_, err = parseSnapshotModules("bad", false)
	if err == nil {
		t.Fatal("parseSnapshotModules(bad) error = nil, want invalid argument")
	}
}

func TestBuildSnapshotDiff(t *testing.T) {
	base := &model.Snapshot{
		ID:        "a",
		Name:      "a",
		CreatedAt: time.Now(),
		Modules: map[string]any{
			"cpu":  map[string]any{"usage": 10},
			"disk": map[string]any{"usage": 20},
		},
	}
	target := &model.Snapshot{
		ID:        "b",
		Name:      "b",
		CreatedAt: time.Now(),
		Modules: map[string]any{
			"cpu":  map[string]any{"usage": 15},
			"port": map[string]any{"open": 8080},
		},
	}

	diff := buildSnapshotDiff(base, target, nil, false)
	if !diff.HasChanges {
		t.Fatal("diff.HasChanges = false, want true")
	}
	if len(diff.Added) != 1 || diff.Added[0].Module != "port" {
		t.Fatalf("diff.Added = %+v, want port added", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Module != "disk" {
		t.Fatalf("diff.Removed = %+v, want disk removed", diff.Removed)
	}
	if len(diff.Changed) != 1 || diff.Changed[0].Module != "cpu" {
		t.Fatalf("diff.Changed = %+v, want cpu changed", diff.Changed)
	}
	if len(diff.Unchanged) != 0 {
		t.Fatalf("diff.Unchanged = %v, want empty", diff.Unchanged)
	}
}

package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"testing"
	"time"
)

func TestSnapshotStoreSaveLoadListDelete(t *testing.T) {
	store, err := NewSnapshotStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSnapshotStore() error = %v", err)
	}

	baseTime := time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC)
	s1 := &model.Snapshot{
		ID:          "snap-a",
		Name:        "snapshot-a",
		Description: "first",
		CreatedAt:   baseTime,
		Host:        "host-a",
		Platform:    "windows",
		Modules: map[string]any{
			"cpu": map[string]any{"usage": 10},
		},
	}
	s2 := &model.Snapshot{
		ID:          "snap-b",
		Name:        "snapshot-b",
		Description: "second",
		CreatedAt:   baseTime.Add(time.Minute),
		Host:        "host-b",
		Platform:    "windows",
		Modules: map[string]any{
			"mem": map[string]any{"usage": 20},
		},
	}

	summaryA, err := store.Save(context.Background(), s1)
	if err != nil {
		t.Fatalf("Save(s1) error = %v", err)
	}
	summaryB, err := store.Save(context.Background(), s2)
	if err != nil {
		t.Fatalf("Save(s2) error = %v", err)
	}
	if summaryA.ID != "snap-a" || summaryB.ID != "snap-b" {
		t.Fatalf("unexpected summary id: %+v %+v", summaryA, summaryB)
	}

	all, err := store.List(context.Background(), 0)
	if err != nil {
		t.Fatalf("List(all) error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List(all) len = %d, want 2", len(all))
	}
	if all[0].ID != "snap-b" || all[1].ID != "snap-a" {
		t.Fatalf("List(all) order = %v, want [snap-b snap-a]", []string{all[0].ID, all[1].ID})
	}

	limited, err := store.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("List(limit=1) error = %v", err)
	}
	if len(limited) != 1 || limited[0].ID != "snap-b" {
		t.Fatalf("List(limit=1) = %+v, want snap-b", limited)
	}

	loaded, err := store.Load(context.Background(), "snap-a")
	if err != nil {
		t.Fatalf("Load(snap-a) error = %v", err)
	}
	if loaded.Name != "snapshot-a" {
		t.Fatalf("Load(snap-a).Name = %q, want snapshot-a", loaded.Name)
	}

	deleted, err := store.Delete(context.Background(), "snap-a")
	if err != nil {
		t.Fatalf("Delete(snap-a) error = %v", err)
	}
	if deleted.ID != "snap-a" {
		t.Fatalf("Delete(snap-a).ID = %q, want snap-a", deleted.ID)
	}

	_, err = store.Load(context.Background(), "snap-a")
	if err == nil {
		t.Fatal("Load(deleted) error = nil, want not found")
	}
	if got := errs.ErrorCode(err); got != errs.CodeNotFound {
		t.Fatalf("errs.ErrorCode(err) = %q, want %q", got, errs.CodeNotFound)
	}
}

func TestSnapshotStoreNormalizeID(t *testing.T) {
	_, err := normalizeSnapshotID("SNAP_ABC-01")
	if err != nil {
		t.Fatalf("normalizeSnapshotID() error = %v", err)
	}

	_, err = normalizeSnapshotID("../bad")
	if err == nil {
		t.Fatal("normalizeSnapshotID('../bad') error = nil, want invalid argument")
	}
	if got := errs.ErrorCode(err); got != errs.CodeInvalidArgument {
		t.Fatalf("errs.ErrorCode(err) = %q, want %q", got, errs.CodeInvalidArgument)
	}
}

func TestSnapshotStoreReadsLegacyCreatedAtFromFileTime(t *testing.T) {
	store, err := NewSnapshotStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSnapshotStore() error = %v", err)
	}

	filePath := filepath.Join(store.layout.SnapshotsDir, "legacy_legacy-id.json")
	content := `{"id":"legacy-id","name":"legacy","host":"h","platform":"p","modules":{}}`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	now := time.Date(2026, 3, 18, 8, 30, 0, 0, time.UTC)
	if err := os.Chtimes(filePath, now, now); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	snapshot, _, err := store.readSnapshotFile(filePath)
	if err != nil {
		t.Fatalf("readSnapshotFile() error = %v", err)
	}
	if !snapshot.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %s, want %s", snapshot.CreatedAt, now)
	}

	if _, err := os.Stat(filePath + ".missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected stat check error: %v", err)
	}
}

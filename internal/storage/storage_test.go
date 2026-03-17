package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syskit/internal/errs"
	"testing"
	"time"
)

func TestEnsureLayoutCreatesManagedDirs(t *testing.T) {
	root := filepath.Join(t.TempDir(), "syskit-data")
	layout, err := EnsureLayout(root)
	if err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	allDirs := append([]string{layout.RootDir}, layout.ManagedDirs()...)
	for _, dir := range allDirs {
		info, statErr := os.Stat(dir)
		if statErr != nil {
			t.Fatalf("os.Stat(%q) error = %v", dir, statErr)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", dir)
		}
	}
}

func TestApplyRetentionByAgeAndSize(t *testing.T) {
	layout, err := EnsureLayout(t.TempDir())
	if err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	oldFile := filepath.Join(layout.SnapshotsDir, "old.snapshot")
	midFile := filepath.Join(layout.ReportsDir, "mid.report")
	newFile := filepath.Join(layout.AuditDir, "new.audit")

	writeSizedFile(t, oldFile, 200*1024, now.AddDate(0, 0, -10))
	writeSizedFile(t, midFile, 700*1024, now.Add(-2*time.Hour))
	writeSizedFile(t, newFile, 600*1024, now.Add(-1*time.Hour))

	stats, err := ApplyRetention(context.Background(), layout, RetentionPolicy{
		RetentionDays: 3,
		MaxStorageMB:  1,
	}, now)
	if err != nil {
		t.Fatalf("ApplyRetention() error = %v", err)
	}

	if fileExists(oldFile) {
		t.Fatalf("old file should be deleted: %s", oldFile)
	}
	if fileExists(midFile) {
		t.Fatalf("mid file should be deleted by size policy: %s", midFile)
	}
	if !fileExists(newFile) {
		t.Fatalf("new file should be kept: %s", newFile)
	}

	if stats.ScannedFiles != 3 {
		t.Fatalf("ScannedFiles = %d, want 3", stats.ScannedFiles)
	}
	if stats.DeletedFiles != 2 {
		t.Fatalf("DeletedFiles = %d, want 2", stats.DeletedFiles)
	}
	if stats.DeletedByAge != 1 {
		t.Fatalf("DeletedByAge = %d, want 1", stats.DeletedByAge)
	}
	if stats.DeletedBySize != 1 {
		t.Fatalf("DeletedBySize = %d, want 1", stats.DeletedBySize)
	}
	if stats.FreedBytes != int64(900*1024) {
		t.Fatalf("FreedBytes = %d, want %d", stats.FreedBytes, 900*1024)
	}
	if stats.RemainingBytes != int64(600*1024) {
		t.Fatalf("RemainingBytes = %d, want %d", stats.RemainingBytes, 600*1024)
	}
}

func TestApplyRetentionRejectsActiveLock(t *testing.T) {
	layout, err := EnsureLayout(t.TempDir())
	if err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	now := time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)
	lockPath := filepath.Join(layout.RootDir, retentionLockFile)
	if err := os.WriteFile(lockPath, []byte("busy"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", lockPath, err)
	}
	if err := os.Chtimes(lockPath, now, now); err != nil {
		t.Fatalf("Chtimes(%q) error = %v", lockPath, err)
	}

	_, err = ApplyRetention(context.Background(), layout, RetentionPolicy{}, now)
	if err == nil {
		t.Fatal("ApplyRetention() error = nil, want lock error")
	}
	if got := errs.Code(err); got != errs.ExitExecutionFailed {
		t.Fatalf("errs.Code(err) = %d, want %d", got, errs.ExitExecutionFailed)
	}
	if !strings.Contains(errs.Message(err), "正在执行") {
		t.Fatalf("errs.Message(err) = %q, want lock hint", errs.Message(err))
	}
}

func TestApplyRetentionCanRecoverStaleLock(t *testing.T) {
	layout, err := EnsureLayout(t.TempDir())
	if err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	now := time.Date(2026, 3, 17, 13, 0, 0, 0, time.UTC)
	lockPath := filepath.Join(layout.RootDir, retentionLockFile)
	if err := os.WriteFile(lockPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", lockPath, err)
	}
	staleAt := now.Add(-retentionLockStaleAfter - time.Minute)
	if err := os.Chtimes(lockPath, staleAt, staleAt); err != nil {
		t.Fatalf("Chtimes(%q) error = %v", lockPath, err)
	}

	stats, err := ApplyRetention(context.Background(), layout, RetentionPolicy{}, now)
	if err != nil {
		t.Fatalf("ApplyRetention() error = %v", err)
	}
	if stats.ScannedFiles != 0 {
		t.Fatalf("ScannedFiles = %d, want 0", stats.ScannedFiles)
	}
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file should be removed after run, stat err = %v", err)
	}
}

func writeSizedFile(t *testing.T, path string, size int, modTime time.Time) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}

	data := make([]byte, size)
	for i := range data {
		data[i] = 'x'
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes(%q) error = %v", path, err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

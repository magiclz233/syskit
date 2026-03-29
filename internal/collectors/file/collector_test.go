package filecollector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindDuplicatesAndDedup(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	c := filepath.Join(root, "c.txt")
	if err := os.WriteFile(a, []byte("same-content"), 0o644); err != nil {
		t.Fatalf("WriteFile(a) error = %v", err)
	}
	if err := os.WriteFile(b, []byte("same-content"), 0o644); err != nil {
		t.Fatalf("WriteFile(b) error = %v", err)
	}
	if err := os.WriteFile(c, []byte("different"), 0o644); err != nil {
		t.Fatalf("WriteFile(c) error = %v", err)
	}

	dup, err := FindDuplicates(context.Background(), DupOptions{
		Path:    root,
		MinSize: 1,
		Hash:    HashSHA256,
	})
	if err != nil {
		t.Fatalf("FindDuplicates() error = %v", err)
	}
	if dup.GroupCount != 1 {
		t.Fatalf("GroupCount = %d, want 1", dup.GroupCount)
	}

	plan, err := BuildDedupPlan(context.Background(), DupOptions{
		Path:    root,
		MinSize: 1,
		Hash:    HashSHA256,
	})
	if err != nil {
		t.Fatalf("BuildDedupPlan() error = %v", err)
	}
	if len(plan.DeleteFiles) != 1 {
		t.Fatalf("DeleteFiles = %d, want 1", len(plan.DeleteFiles))
	}

	result, err := ExecuteDedupPlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("ExecuteDedupPlan() error = %v", err)
	}
	if result.DeletedFiles != 1 {
		t.Fatalf("DeletedFiles = %d, want 1", result.DeletedFiles)
	}
}

func TestArchiveAndEmpty(t *testing.T) {
	root := t.TempDir()
	oldFile := filepath.Join(root, "logs", "old.log")
	if err := os.MkdirAll(filepath.Dir(oldFile), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(oldFile, []byte("line"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	oldTime := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	archivePlan, err := BuildArchivePlan(context.Background(), ArchiveOptions{
		Path:      root,
		OlderThan: 24 * time.Hour,
		Compress:  "gzip",
	})
	if err != nil {
		t.Fatalf("BuildArchivePlan() error = %v", err)
	}
	if archivePlan.CandidateCount != 1 {
		t.Fatalf("CandidateCount = %d, want 1", archivePlan.CandidateCount)
	}
	archiveResult, err := ExecuteArchivePlan(context.Background(), archivePlan)
	if err != nil {
		t.Fatalf("ExecuteArchivePlan() error = %v", err)
	}
	if archiveResult.ArchivedCount != 1 {
		t.Fatalf("ArchivedCount = %d, want 1", archiveResult.ArchivedCount)
	}

	emptyDir := filepath.Join(root, "empty-dir", "nested")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(empty) error = %v", err)
	}
	emptyPlan, err := BuildEmptyPlan(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("BuildEmptyPlan() error = %v", err)
	}
	if emptyPlan.CandidateCount == 0 {
		t.Fatalf("CandidateCount = 0, want >0")
	}
	emptyResult, err := ExecuteEmptyPlan(context.Background(), emptyPlan)
	if err != nil {
		t.Fatalf("ExecuteEmptyPlan() error = %v", err)
	}
	if emptyResult.DeletedDirs == 0 {
		t.Fatalf("DeletedDirs = 0, want >0")
	}
}

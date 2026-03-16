package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseTargets(t *testing.T) {
	targets, err := ParseTargets("")
	if err != nil {
		t.Fatalf("ParseTargets empty failed: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("expect 3 default targets, got %d", len(targets))
	}

	targets, err = ParseTargets("temp,cache")
	if err != nil {
		t.Fatalf("ParseTargets temp,cache failed: %v", err)
	}
	if len(targets) != 2 || targets[0] != TargetTemp || targets[1] != TargetCache {
		t.Fatalf("unexpected targets: %+v", targets)
	}

	if _, err := ParseTargets("invalid"); err == nil {
		t.Fatalf("ParseTargets invalid should fail")
	}
}

func TestParseOlderThan(t *testing.T) {
	if got, err := ParseOlderThan("72h"); err != nil || got != 72*time.Hour {
		t.Fatalf("ParseOlderThan 72h failed, got=%v err=%v", got, err)
	}
	if got, err := ParseOlderThan("7d"); err != nil || got != 7*24*time.Hour {
		t.Fatalf("ParseOlderThan 7d failed, got=%v err=%v", got, err)
	}
	if got, err := ParseOlderThan("2w"); err != nil || got != 14*24*time.Hour {
		t.Fatalf("ParseOlderThan 2w failed, got=%v err=%v", got, err)
	}
	if _, err := ParseOlderThan("0h"); err == nil {
		t.Fatalf("ParseOlderThan 0h should fail")
	}
}

func TestBuildPlanAndApply(t *testing.T) {
	base := t.TempDir()
	dataDir := filepath.Join(base, "data")
	logsDir := filepath.Join(base, "logs")
	cacheDir := filepath.Join(base, "cache")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data failed: %v", err)
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("mkdir logs failed: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache failed: %v", err)
	}

	oldFile := filepath.Join(logsDir, "old.log")
	newFile := filepath.Join(cacheDir, "new.cache")
	if err := os.WriteFile(oldFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old file failed: %v", err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0o644); err != nil {
		t.Fatalf("write new file failed: %v", err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes old file failed: %v", err)
	}

	plan, err := BuildPlan(nil, PlanOptions{
		Targets:        []Target{TargetLogs, TargetCache},
		OlderThan:      24 * time.Hour,
		StorageDataDir: dataDir,
		LoggingFile:    filepath.Join(logsDir, "syskit.log"),
		Now:            time.Now(),
	})
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if plan.CandidateCount != 1 {
		t.Fatalf("candidate count want=1 got=%d", plan.CandidateCount)
	}
	if plan.Candidates[0].Path != oldFile {
		t.Fatalf("candidate path want=%s got=%s", oldFile, plan.Candidates[0].Path)
	}

	result, err := ApplyPlan(nil, plan)
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}
	if !result.Applied {
		t.Fatalf("result should be applied")
	}
	if result.DeletedCount != 1 {
		t.Fatalf("deleted count want=1 got=%d", result.DeletedCount)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("old file should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("new file should remain, stat err=%v", err)
	}
}

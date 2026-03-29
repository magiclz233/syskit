package logcollector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAnalyzeAndSearch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app.log")
	content := "2026-03-29T10:00:00Z INFO startup ok\n" +
		"2026-03-29T10:01:00Z ERROR db timeout\n" +
		"2026-03-29T10:02:00Z WARN retry\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	overview, err := Analyze(context.Background(), OverviewOptions{
		Files:  []string{path},
		Level:  "all",
		Top:    10,
		Detail: true,
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if overview.TotalLines != 3 {
		t.Fatalf("TotalLines = %d, want 3", overview.TotalLines)
	}
	if overview.LevelCounts["error"] != 1 {
		t.Fatalf("error count = %d, want 1", overview.LevelCounts["error"])
	}

	search, err := Search(context.Background(), SearchOptions{
		Files:      []string{path},
		Keyword:    "timeout",
		IgnoreCase: true,
		Context:    1,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if search.TotalMatches != 1 {
		t.Fatalf("TotalMatches = %d, want 1", search.TotalMatches)
	}
	if search.Matches[0].Line != 2 {
		t.Fatalf("match line = %d, want 2", search.Matches[0].Line)
	}
}

func TestWatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "watch.log")
	if err := os.WriteFile(path, []byte("init\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(init) error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan *WatchResult, 1)
	go func() {
		result, _ := Watch(ctx, WatchOptions{
			Files:          []string{path},
			ThresholdSize:  1,
			ThresholdError: 1,
			Interval:       50 * time.Millisecond,
			MaxSamples:     2,
		})
		done <- result
	}()

	time.Sleep(70 * time.Millisecond)
	if err := os.WriteFile(path, []byte("init\nERROR something bad\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(update) error = %v", err)
	}

	select {
	case result := <-done:
		if result == nil {
			t.Fatal("Watch() result is nil")
		}
		if result.SampleCount != 2 {
			t.Fatalf("SampleCount = %d, want 2", result.SampleCount)
		}
		if len(result.Alerts) == 0 {
			t.Fatalf("alerts is empty, want >= 1")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch() timeout")
	}
}

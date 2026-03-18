package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoggerLogWritesJSONL(t *testing.T) {
	root := t.TempDir()
	logger, err := NewLogger(root)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	fixed := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	logger.now = func() time.Time { return fixed }

	event := Event{
		Command:    "syskit port kill",
		Action:     "port.kill",
		Target:     "port:8080",
		Result:     "success",
		DurationMs: 1234,
		Metadata: map[string]any{
			"apply": true,
		},
	}
	if err := logger.Log(context.Background(), event); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	path := filepath.Join(root, "audit", "20260318.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if len(data) == 0 {
		t.Fatal("audit file is empty")
	}

	var got Event
	if err := json.Unmarshal(data[:len(data)-1], &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Command != event.Command {
		t.Fatalf("Command = %q, want %q", got.Command, event.Command)
	}
	if got.Action != event.Action {
		t.Fatalf("Action = %q, want %q", got.Action, event.Action)
	}
	if got.Timestamp.IsZero() {
		t.Fatal("Timestamp is zero")
	}
	if got.TraceID == "" {
		t.Fatal("TraceID is empty")
	}
}

func TestLoggerLogRejectsMissingCommand(t *testing.T) {
	logger, err := NewLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	err = logger.Log(context.Background(), Event{
		Action: "port.kill",
	})
	if err == nil {
		t.Fatal("Log() error = nil, want invalid argument")
	}
}

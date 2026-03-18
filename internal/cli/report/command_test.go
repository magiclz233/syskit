package report

import (
	"syskit/internal/domain/model"
	"testing"
	"time"
)

func TestNormalizeReportType(t *testing.T) {
	got, err := normalizeReportType(" HeAlTh ")
	if err != nil {
		t.Fatalf("normalizeReportType(health) error = %v", err)
	}
	if got != reportTypeHealth {
		t.Fatalf("normalizeReportType(health) = %q, want %q", got, reportTypeHealth)
	}

	_, err = normalizeReportType("bad")
	if err == nil {
		t.Fatal("normalizeReportType(bad) error = nil, want invalid argument")
	}
}

func TestParseTimeRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "empty_default", input: "", want: 24 * time.Hour},
		{name: "duration_hours", input: "48h", want: 48 * time.Hour},
		{name: "duration_minutes", input: "90m", want: 90 * time.Minute},
		{name: "days_suffix", input: "7d", want: 7 * 24 * time.Hour},
		{name: "weeks_suffix", input: "2w", want: 14 * 24 * time.Hour},
		{name: "invalid_zero", input: "0h", wantErr: true},
		{name: "invalid_format", input: "abc", wantErr: true},
		{name: "invalid_days_zero", input: "0d", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTimeRange(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseTimeRange(%q) error = nil, want error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTimeRange(%q) error = %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseTimeRange(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestFilterSnapshotsByWindow(t *testing.T) {
	base := time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC)
	start := base.Add(-2 * time.Hour)
	end := base

	items := []model.SnapshotSummary{
		{ID: "old", CreatedAt: start.Add(-time.Second)},
		{ID: "left", CreatedAt: start},
		{ID: "mid", CreatedAt: start.Add(30 * time.Minute)},
		{ID: "right", CreatedAt: end},
		{ID: "new", CreatedAt: end.Add(time.Second)},
	}

	got := filterSnapshotsByWindow(items, start, end)
	if len(got) != 3 {
		t.Fatalf("filterSnapshotsByWindow len = %d, want 3", len(got))
	}
	if got[0].ID != "left" || got[1].ID != "mid" || got[2].ID != "right" {
		t.Fatalf("filterSnapshotsByWindow ids = [%s %s %s], want [left mid right]", got[0].ID, got[1].ID, got[2].ID)
	}
}

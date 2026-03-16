package model

import "testing"

func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantOK  bool
		wantPos int
	}{
		{name: "critical", input: "critical", want: SeverityCritical, wantOK: true, wantPos: 4},
		{name: "upper high", input: "HIGH", want: SeverityHigh, wantOK: true, wantPos: 3},
		{name: "medium trim", input: " medium ", want: SeverityMedium, wantOK: true, wantPos: 2},
		{name: "low", input: "low", want: SeverityLow, wantOK: true, wantPos: 1},
		{name: "invalid", input: "warn", want: "", wantOK: false, wantPos: 0},
	}

	for _, tt := range tests {
		got, ok := NormalizeSeverity(tt.input)
		if ok != tt.wantOK {
			t.Fatalf("%s: ok want=%v got=%v", tt.name, tt.wantOK, ok)
		}
		if got != tt.want {
			t.Fatalf("%s: severity want=%s got=%s", tt.name, tt.want, got)
		}
		if rank := SeverityRank(tt.input); rank != tt.wantPos {
			t.Fatalf("%s: rank want=%d got=%d", tt.name, tt.wantPos, rank)
		}
	}
}

func TestNormalizeScopeAndSkipReason(t *testing.T) {
	if got := NormalizeScope("system"); got != ScopeSystem {
		t.Fatalf("NormalizeScope(system) want=%s got=%s", ScopeSystem, got)
	}
	if got := NormalizeScope("unknown"); got != ScopeLocal {
		t.Fatalf("NormalizeScope(unknown) want=%s got=%s", ScopeLocal, got)
	}

	if got := NormalizeSkipReason("permission_denied"); got != SkipReasonPermissionDenied {
		t.Fatalf("NormalizeSkipReason(permission_denied) want=%s got=%s", SkipReasonPermissionDenied, got)
	}
	if got := NormalizeSkipReason("something_else"); got != SkipReasonExecutionFailed {
		t.Fatalf("NormalizeSkipReason(something_else) want=%s got=%s", SkipReasonExecutionFailed, got)
	}
}

package proc

import "testing"

func TestParseSortBy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    SortBy
		wantErr bool
	}{
		{name: "cpu", input: "cpu", want: SortByCPU},
		{name: "mem-upper", input: "MEM", want: SortByMem},
		{name: "io-trim", input: " io ", want: SortByIO},
		{name: "fd", input: "fd", want: SortByFD},
		{name: "invalid", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSortBy(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("want %s, got %s", tt.want, got)
			}
		})
	}
}

func TestParseEnvMap(t *testing.T) {
	env := parseEnvMap([]string{
		"FOO=1",
		"",
		" BAR =2 ",
		"INVALID",
		"FOO=3",
	})

	if got := env["FOO"]; got != "1" {
		t.Fatalf("FOO expected 1, got %q", got)
	}
	if got := env["BAR"]; got != "2 " {
		t.Fatalf("BAR expected %q, got %q", "2 ", got)
	}
	if _, ok := env["INVALID"]; ok {
		t.Fatalf("INVALID should be ignored")
	}
}

func TestSortTopSnapshotsByMem(t *testing.T) {
	items := []ProcessSnapshot{
		{PID: 2, RSSBytes: 10},
		{PID: 1, RSSBytes: 20},
		{PID: 3, RSSBytes: 20},
	}
	sortTopSnapshots(items, SortByMem)

	want := []int32{1, 3, 2}
	for i := range want {
		if items[i].PID != want[i] {
			t.Fatalf("index %d expected pid %d, got %d", i, want[i], items[i].PID)
		}
	}
}

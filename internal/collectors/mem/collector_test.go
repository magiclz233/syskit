package mem

import "testing"

func TestParseSortBy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    SortBy
		wantErr bool
	}{
		{name: "default", input: "", want: SortByRSS},
		{name: "rss", input: "rss", want: SortByRSS},
		{name: "vms-upper", input: "VMS", want: SortByVMS},
		{name: "swap", input: " swap ", want: SortBySwap},
		{name: "invalid", input: "cpu", wantErr: true},
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

func TestSortEntries(t *testing.T) {
	items := []ProcessEntry{
		{PID: 3, RSSBytes: 100, VMSBytes: 300, SwapBytes: 10},
		{PID: 1, RSSBytes: 200, VMSBytes: 100, SwapBytes: 20},
		{PID: 2, RSSBytes: 200, VMSBytes: 200, SwapBytes: 5},
	}

	sortEntries(items, SortByRSS)
	if items[0].PID != 1 || items[1].PID != 2 {
		t.Fatalf("sort by rss failed: %+v", items)
	}

	sortEntries(items, SortByVMS)
	if items[0].PID != 3 {
		t.Fatalf("sort by vms failed: %+v", items)
	}

	sortEntries(items, SortBySwap)
	if items[0].PID != 1 {
		t.Fatalf("sort by swap failed: %+v", items)
	}
}

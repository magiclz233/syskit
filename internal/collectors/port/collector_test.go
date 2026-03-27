package port

import (
	"reflect"
	"testing"
)

func TestParsePortExpression(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []int
		wantErr bool
	}{
		{name: "single", raw: "80", want: []int{80}},
		{name: "multi", raw: "80,443", want: []int{80, 443}},
		{name: "range", raw: "8080-8082", want: []int{8080, 8081, 8082}},
		{name: "mix-and-dedup", raw: "80,81-82,80", want: []int{80, 81, 82}},
		{name: "invalid-char", raw: "abc", wantErr: true},
		{name: "invalid-range", raw: "100-90", wantErr: true},
		{name: "out-of-range", raw: "70000", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePortExpression(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("want %v, got %v", tt.want, got)
			}
		})
	}
}

func TestParseProtocolAndSortBy(t *testing.T) {
	if got, err := ParseProtocol("TCP"); err != nil || got != ProtocolTCP {
		t.Fatalf("ParseProtocol tcp failed, got=%v err=%v", got, err)
	}
	if _, err := ParseProtocol("icmp"); err == nil {
		t.Fatalf("ParseProtocol should fail for icmp")
	}

	if got, err := ParseSortBy("pid"); err != nil || got != "pid" {
		t.Fatalf("ParseSortBy pid failed, got=%s err=%v", got, err)
	}
	if _, err := ParseSortBy("name"); err == nil {
		t.Fatalf("ParseSortBy should fail for name")
	}

	if got, err := ParseScanMode("full"); err != nil || got != ScanModeFull {
		t.Fatalf("ParseScanMode full failed, got=%v err=%v", got, err)
	}
	if _, err := ParseScanMode("deep"); err == nil {
		t.Fatalf("ParseScanMode should fail for deep")
	}
}

package dns

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestParseResolveType(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    ResolveType
		wantErr bool
	}{
		{name: "default", raw: "", want: ResolveTypeA},
		{name: "mx", raw: "mx", want: ResolveTypeMX},
		{name: "aaaa", raw: "AAAA", want: ResolveTypeAAAA},
		{name: "invalid", raw: "PTR", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResolveType(tt.raw)
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

func TestNormalizeDNSServer(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty", raw: "", want: ""},
		{name: "host only", raw: "8.8.8.8", want: "8.8.8.8:53"},
		{name: "host port", raw: "8.8.8.8:5353", want: "8.8.8.8:5353"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeDNSServer(tt.raw)
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

func TestUniqueSortedRecords(t *testing.T) {
	input := []ResolveRecord{
		{Type: "a", Value: "127.0.0.1"},
		{Type: "A", Value: "127.0.0.1"},
		{Type: "TXT", Value: "hello"},
	}
	got := uniqueSortedRecords(input)
	want := []ResolveRecord{
		{Type: "A", Value: "127.0.0.1"},
		{Type: "TXT", Value: "hello"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestResolveDomainLocalhost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := ResolveDomain(ctx, ResolveOptions{Domain: "localhost", Type: ResolveTypeA, Timeout: time.Second})
	if err != nil {
		t.Fatalf("ResolveDomain() error = %v", err)
	}
	if result.Count <= 0 {
		t.Fatalf("record count = %d, want > 0", result.Count)
	}
}

func TestBenchDomainLocalhost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := BenchDomain(ctx, BenchOptions{Domain: "localhost", Type: ResolveTypeA, Count: 3, Timeout: time.Second})
	if err != nil {
		t.Fatalf("BenchDomain() error = %v", err)
	}
	if len(result.Attempts) != 3 {
		t.Fatalf("attempt count = %d, want 3", len(result.Attempts))
	}
}

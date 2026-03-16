package proc

import "testing"

func TestParsePID(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int32
		wantErr bool
	}{
		{name: "normal", raw: "123", want: 123},
		{name: "trim", raw: " 42 ", want: 42},
		{name: "zero", raw: "0", wantErr: true},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "invalid", raw: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePID(tt.raw)
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
				t.Fatalf("want %d, got %d", tt.want, got)
			}
		})
	}
}

func TestOptionalPID(t *testing.T) {
	got, err := optionalPID(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil pid when no args")
	}

	got, err = optionalPID([]string{"15"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || *got != 15 {
		t.Fatalf("expected pid 15, got %#v", got)
	}
}

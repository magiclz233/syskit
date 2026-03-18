package policy

import (
	"syskit/internal/errs"
	"testing"
)

func TestNormalizeType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		allowAll bool
		want     string
		wantErr  bool
	}{
		{name: "config", input: "config", allowAll: true, want: "config"},
		{name: "policy", input: " policy ", allowAll: true, want: "policy"},
		{name: "all_allowed", input: "all", allowAll: true, want: "all"},
		{name: "all_not_allowed", input: "all", allowAll: false, wantErr: true},
		{name: "invalid_allow_all", input: "bad", allowAll: true, wantErr: true},
		{name: "invalid_validate", input: "bad", allowAll: false, wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeType(tc.input, tc.allowAll)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeType(%q,%v) error=nil, want error", tc.input, tc.allowAll)
				}
				if code := errs.ErrorCode(err); code != errs.CodeInvalidArgument {
					t.Fatalf("errs.ErrorCode(err) = %s, want %s", code, errs.CodeInvalidArgument)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeType(%q,%v) error=%v", tc.input, tc.allowAll, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeType(%q,%v) = %q, want %q", tc.input, tc.allowAll, got, tc.want)
			}
		})
	}
}

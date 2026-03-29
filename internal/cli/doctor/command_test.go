package doctor

import (
	"slices"
	"syskit/internal/errs"
	"testing"

	"github.com/spf13/cobra"
)

func TestNormalizeMode(t *testing.T) {
	mode, err := normalizeMode("")
	if err != nil {
		t.Fatalf("normalizeMode(\"\") error = %v", err)
	}
	if mode != "quick" {
		t.Fatalf("normalizeMode(\"\") = %q, want quick", mode)
	}

	mode, err = normalizeMode(" DeEp ")
	if err != nil {
		t.Fatalf("normalizeMode(deep) error = %v", err)
	}
	if mode != "deep" {
		t.Fatalf("normalizeMode(deep) = %q, want deep", mode)
	}

	_, err = normalizeMode("bad")
	if err == nil {
		t.Fatal("normalizeMode(bad) error = nil, want invalid argument")
	}
}

func TestParseModules(t *testing.T) {
	got, err := parseModules("")
	if err != nil {
		t.Fatalf("parseModules(\"\") error = %v", err)
	}
	if got != nil {
		t.Fatalf("parseModules(\"\") = %v, want nil", got)
	}

	got, err = parseModules("mem,port,mem")
	if err != nil {
		t.Fatalf("parseModules(valid) error = %v", err)
	}
	if !slices.Equal(got, []string{"mem", "port"}) {
		t.Fatalf("parseModules(valid) = %v, want [mem port]", got)
	}

	_, err = parseModules("bad")
	if err == nil {
		t.Fatal("parseModules(bad) error = nil, want invalid argument")
	}
	if gotCode := errs.ErrorCode(err); gotCode != errs.CodeInvalidArgument {
		t.Fatalf("errs.ErrorCode(err) = %s, want %s", gotCode, errs.CodeInvalidArgument)
	}
}

func TestRunDoctorAllRejectExcludeAllModules(t *testing.T) {
	err := runDoctorAll(&cobra.Command{Use: "doctor all"}, &allOptions{
		mode:    "quick",
		exclude: "port,cpu,mem,disk",
	})
	if err == nil {
		t.Fatal("runDoctorAll() error = nil, want invalid argument")
	}
	if gotCode := errs.ErrorCode(err); gotCode != errs.CodeInvalidArgument {
		t.Fatalf("errs.ErrorCode(err) = %s, want %s", gotCode, errs.CodeInvalidArgument)
	}
}

func TestNormalizeNetworkTarget(t *testing.T) {
	target, dnsDomain, err := normalizeNetworkTarget("1.1.1.1")
	if err != nil {
		t.Fatalf("normalizeNetworkTarget(ip) error = %v", err)
	}
	if target != "1.1.1.1" || dnsDomain != "" {
		t.Fatalf("normalizeNetworkTarget(ip) = (%q,%q), want (1.1.1.1,\"\")", target, dnsDomain)
	}

	target, dnsDomain, err = normalizeNetworkTarget("example.com:443")
	if err != nil {
		t.Fatalf("normalizeNetworkTarget(host:port) error = %v", err)
	}
	if target != "example.com" || dnsDomain != "example.com" {
		t.Fatalf("normalizeNetworkTarget(host:port) = (%q,%q), want (example.com,example.com)", target, dnsDomain)
	}

	_, _, err = normalizeNetworkTarget(" ")
	if err == nil {
		t.Fatal("normalizeNetworkTarget(empty) error = nil, want invalid argument")
	}
}

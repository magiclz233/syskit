package cli

import (
	"syskit/internal/config"
	"syskit/internal/errs"
	"testing"

	"github.com/spf13/cobra"
)

func TestNormalizeAndValidate(t *testing.T) {
	t.Run("json overrides format and apply turns off dry-run", func(t *testing.T) {
		opts, _ := newGlobalTestContext(t)
		opts.format = "table"
		opts.failOn = " MEDIUM "
		opts.json = true
		opts.apply = true
		opts.dryRun = true

		if err := opts.NormalizeAndValidate(); err != nil {
			t.Fatalf("NormalizeAndValidate() error = %v", err)
		}
		if opts.format != "json" {
			t.Fatalf("format = %q, want json", opts.format)
		}
		if opts.failOn != "medium" {
			t.Fatalf("failOn = %q, want medium", opts.failOn)
		}
		if opts.dryRun {
			t.Fatal("dryRun should be false when apply=true")
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		opts, _ := newGlobalTestContext(t)
		opts.format = "bad"

		err := opts.NormalizeAndValidate()
		if err == nil {
			t.Fatal("NormalizeAndValidate() error = nil, want invalid argument")
		}
		if got := errs.ErrorCode(err); got != errs.CodeInvalidArgument {
			t.Fatalf("errs.ErrorCode(err) = %s, want %s", got, errs.CodeInvalidArgument)
		}
	})

	t.Run("invalid fail-on", func(t *testing.T) {
		opts, _ := newGlobalTestContext(t)
		opts.failOn = "bad"

		err := opts.NormalizeAndValidate()
		if err == nil {
			t.Fatal("NormalizeAndValidate() error = nil, want invalid argument")
		}
		if got := errs.ErrorCode(err); got != errs.CodeInvalidArgument {
			t.Fatalf("errs.ErrorCode(err) = %s, want %s", got, errs.CodeInvalidArgument)
		}
	})
}

func TestApplyBootstrapEnv(t *testing.T) {
	t.Run("applies env when flags not changed", func(t *testing.T) {
		opts, cmd := newGlobalTestContext(t)
		t.Setenv("SYSKIT_CONFIG", "/tmp/custom-config.yaml")
		t.Setenv("SYSKIT_POLICY", "/tmp/custom-policy.yaml")
		t.Setenv("SYSKIT_OUTPUT", "json")
		t.Setenv("SYSKIT_NO_COLOR", "true")

		opts.ApplyBootstrapEnv(cmd)
		if opts.config != "/tmp/custom-config.yaml" {
			t.Fatalf("config = %q, want /tmp/custom-config.yaml", opts.config)
		}
		if opts.policy != "/tmp/custom-policy.yaml" {
			t.Fatalf("policy = %q, want /tmp/custom-policy.yaml", opts.policy)
		}
		if opts.format != "json" {
			t.Fatalf("format = %q, want json", opts.format)
		}
		if !opts.noColor {
			t.Fatal("noColor = false, want true")
		}
	})

	t.Run("respects changed flags", func(t *testing.T) {
		opts, cmd := newGlobalTestContext(t)
		if err := cmd.ParseFlags([]string{"--format", "markdown", "--no-color", "--config", "cli-config.yaml"}); err != nil {
			t.Fatalf("ParseFlags() error = %v", err)
		}
		t.Setenv("SYSKIT_CONFIG", "env-config.yaml")
		t.Setenv("SYSKIT_OUTPUT", "json")
		t.Setenv("SYSKIT_NO_COLOR", "false")

		opts.ApplyBootstrapEnv(cmd)
		if opts.config != "cli-config.yaml" {
			t.Fatalf("config = %q, want cli-config.yaml", opts.config)
		}
		if opts.format != "markdown" {
			t.Fatalf("format = %q, want markdown", opts.format)
		}
		if !opts.noColor {
			t.Fatal("noColor should keep true because --no-color changed")
		}
	})
}

func TestApplyConfig(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputConfig{
			Format:  "markdown",
			NoColor: true,
			Quiet:   true,
		},
		Risk: config.RiskConfig{
			DryRunDefault: false,
		},
	}

	t.Run("applies config when flags not changed", func(t *testing.T) {
		opts, cmd := newGlobalTestContext(t)
		opts.ApplyConfig(cmd, cfg)
		if opts.format != "markdown" {
			t.Fatalf("format = %q, want markdown", opts.format)
		}
		if !opts.noColor {
			t.Fatal("noColor = false, want true")
		}
		if !opts.quiet {
			t.Fatal("quiet = false, want true")
		}
		if opts.dryRun {
			t.Fatal("dryRun = true, want false")
		}
	})

	t.Run("respects changed flags", func(t *testing.T) {
		opts, cmd := newGlobalTestContext(t)
		if err := cmd.ParseFlags([]string{"--format", "json", "--quiet=false", "--dry-run=true"}); err != nil {
			t.Fatalf("ParseFlags() error = %v", err)
		}
		opts.ApplyConfig(cmd, cfg)
		if opts.format != "json" {
			t.Fatalf("format = %q, want json", opts.format)
		}
		if opts.quiet {
			t.Fatal("quiet = true, want false")
		}
		if !opts.dryRun {
			t.Fatal("dryRun = false, want true")
		}
	})
}

func newGlobalTestContext(t *testing.T) (*globalOptions, *cobra.Command) {
	t.Helper()
	opts := newGlobalOptions()
	cmd := &cobra.Command{Use: "syskit"}
	opts.Bind(cmd)
	return opts, cmd
}

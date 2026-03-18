package fix

import (
	"syskit/internal/errs"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunCleanupValidateArgs(t *testing.T) {
	cmd := &cobra.Command{Use: "cleanup"}

	t.Run("invalid target", func(t *testing.T) {
		err := runCleanup(cmd, &cleanupOptions{
			target:    "bad",
			olderThan: "7d",
		})
		if err == nil {
			t.Fatal("runCleanup() error = nil, want invalid argument")
		}
		if got := errs.ErrorCode(err); got != errs.CodeInvalidArgument {
			t.Fatalf("errs.ErrorCode(err) = %s, want %s", got, errs.CodeInvalidArgument)
		}
	})

	t.Run("invalid older-than", func(t *testing.T) {
		err := runCleanup(cmd, &cleanupOptions{
			target:    "temp,logs,cache",
			olderThan: "bad",
		})
		if err == nil {
			t.Fatal("runCleanup() error = nil, want invalid argument")
		}
		if got := errs.ErrorCode(err); got != errs.CodeInvalidArgument {
			t.Fatalf("errs.ErrorCode(err) = %s, want %s", got, errs.CodeInvalidArgument)
		}
	})
}

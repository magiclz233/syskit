package disk

import (
	"syskit/internal/errs"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunScanValidateArgs(t *testing.T) {
	cmd := &cobra.Command{Use: "scan"}

	t.Run("invalid min-size", func(t *testing.T) {
		err := runScan(cmd, ".", &scanOptions{
			limit:   20,
			minSize: "bad",
			depth:   0,
		})
		if err == nil {
			t.Fatal("runScan() error = nil, want invalid argument")
		}
		if got := errs.ErrorCode(err); got != errs.CodeInvalidArgument {
			t.Fatalf("errs.ErrorCode(err) = %s, want %s", got, errs.CodeInvalidArgument)
		}
	})

	t.Run("limit <= 0", func(t *testing.T) {
		err := runScan(cmd, ".", &scanOptions{
			limit:   0,
			minSize: "1B",
			depth:   0,
		})
		if err == nil {
			t.Fatal("runScan() error = nil, want invalid argument")
		}
		if got := errs.ErrorCode(err); got != errs.CodeInvalidArgument {
			t.Fatalf("errs.ErrorCode(err) = %s, want %s", got, errs.CodeInvalidArgument)
		}
	})

	t.Run("depth < 0", func(t *testing.T) {
		err := runScan(cmd, ".", &scanOptions{
			limit:   10,
			minSize: "1B",
			depth:   -1,
		})
		if err == nil {
			t.Fatal("runScan() error = nil, want invalid argument")
		}
		if got := errs.ErrorCode(err); got != errs.CodeInvalidArgument {
			t.Fatalf("errs.ErrorCode(err) = %s, want %s", got, errs.CodeInvalidArgument)
		}
	})
}

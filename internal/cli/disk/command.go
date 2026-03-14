package disk

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk",
		Short: "磁盘总览与扫描",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliutil.PendingError(cmd.CommandPath())
		},
	}

	cmd.AddCommand(
		cliutil.NewPendingCommand("scan <path>", "扫描大文件和大目录"),
	)

	return cmd
}

package mem

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mem",
		Short: "内存总览与分析",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliutil.PendingError(cmd.CommandPath())
		},
	}

	cmd.AddCommand(
		cliutil.NewPendingCommand("top", "查看进程内存排行"),
	)

	return cmd
}

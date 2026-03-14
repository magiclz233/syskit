package port

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port <port[,port]|range>",
		Short: "端口查询与释放",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return cliutil.PendingError(cmd.CommandPath())
		},
	}

	cmd.AddCommand(
		cliutil.NewPendingCommand("list", "查看监听端口列表"),
		cliutil.NewPendingCommand("kill <port>", "释放指定端口"),
	)

	return cmd
}

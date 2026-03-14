package policy

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "配置与策略管理命令",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		cliutil.NewPendingCommand("show", "查看当前配置和策略"),
		cliutil.NewPendingCommand("init", "生成配置和策略模板"),
		cliutil.NewPendingCommand("validate <path>", "校验配置或策略文件"),
	)

	return cmd
}

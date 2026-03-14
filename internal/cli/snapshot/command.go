package snapshot

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "快照管理命令",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		cliutil.NewPendingCommand("create", "创建系统快照"),
		cliutil.NewPendingCommand("list", "列出已保存快照"),
		cliutil.NewPendingCommand("show <id>", "查看快照详情"),
		cliutil.NewPendingCommand("diff <idA> [idB]", "比较两个快照"),
		cliutil.NewPendingCommand("delete <id>", "删除指定快照"),
	)

	return cmd
}

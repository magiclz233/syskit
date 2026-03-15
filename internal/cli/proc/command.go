// Package proc 负责进程查询和管理命令组。
package proc

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

// NewCommand 创建 `proc` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proc",
		Short: "进程查询与管理",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		cliutil.NewPendingCommand("top", "查看进程资源排行"),
		cliutil.NewPendingCommand("tree [pid]", "查看进程树"),
		cliutil.NewPendingCommand("info <pid>", "查看单进程详情"),
		cliutil.NewPendingCommand("kill <pid>", "结束指定进程"),
	)

	return cmd
}

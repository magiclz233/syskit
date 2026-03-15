// Package fix 负责修复和清理命令组。
package fix

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

// NewCommand 创建 `fix` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "修复与清理命令",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		cliutil.NewPendingCommand("cleanup", "执行磁盘和缓存清理"),
	)

	return cmd
}

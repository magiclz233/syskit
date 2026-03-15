// Package cpu 负责 CPU 相关命令。
package cpu

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

// NewCommand 创建 `cpu` 顶层命令。
// 当前具体实现还未落地，因此先返回统一占位错误。
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cpu",
		Short: "CPU 总览与分析",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliutil.PendingError(cmd.CommandPath())
		},
	}
}

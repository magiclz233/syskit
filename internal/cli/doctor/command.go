// Package doctor 负责系统体检命令组。
package doctor

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

// NewCommand 创建 `doctor` 顶层命令，并挂载当前已注册的专项诊断子命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "系统体检与专项诊断",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		cliutil.NewPendingCommand("all", "执行全量体检"),
		cliutil.NewPendingCommand("port", "执行端口专项诊断"),
		cliutil.NewPendingCommand("cpu", "执行 CPU 专项诊断"),
		cliutil.NewPendingCommand("mem", "执行内存专项诊断"),
		cliutil.NewPendingCommand("disk", "执行磁盘专项诊断"),
	)

	return cmd
}

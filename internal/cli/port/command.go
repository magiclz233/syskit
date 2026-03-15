// Package port 负责端口查询和端口释放命令。
package port

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

// NewCommand 创建 `port` 顶层命令。
// 直接执行 `port` 且未传参数时展示帮助；传了参数但功能未实现时返回统一占位错误。
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

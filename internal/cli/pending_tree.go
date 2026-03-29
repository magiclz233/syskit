package cli

import (
	"github.com/spf13/cobra"
)

// registerPendingCommands 用于注册规范中“已预留但尚未实现”的命令入口。
// 当前版本已完成 P1 清单能力，暂未保留额外 pending 命令。
func registerPendingCommands(rootCmd *cobra.Command) {
	_ = rootCmd
}

func findCommand(rootCmd *cobra.Command, name string) *cobra.Command {
	if rootCmd == nil {
		return nil
	}
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

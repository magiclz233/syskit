// Package report 负责报告生成命令组。
package report

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

// NewCommand 创建 `report` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "报告生成命令",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		cliutil.NewPendingCommand("generate", "生成体检或巡检报告"),
	)

	return cmd
}

// Package cliutil 中的 pending 工具用于为尚未实现的命令返回统一占位错误。
package cliutil

import (
	"fmt"
	"syskit/internal/errs"

	"github.com/spf13/cobra"
)

// PendingError 为尚未开发完成的命令生成统一错误。
func PendingError(commandPath string) error {
	return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, fmt.Sprintf("%s 尚未开发", commandPath))
}

// NewPendingCommand 返回一个占位子命令。
// 当前很多 P0/P1 命令还在排期中，这个 helper 可以保证命令树先稳定下来，
// 同时对用户明确说明“命令已注册但尚未开发”。
func NewPendingCommand(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return PendingError(cmd.CommandPath())
		},
	}
}

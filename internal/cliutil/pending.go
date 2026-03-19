// Package cliutil 中的 pending 工具用于为尚未实现的命令返回统一占位错误。
package cliutil

import (
	"fmt"
	"syskit/internal/errs"

	"github.com/spf13/cobra"
)

// PendingError 为尚未开发完成的命令生成统一错误。
func PendingError(commandPath string) error {
	return errs.NewWithSuggestion(
		errs.ExitExecutionFailed,
		errs.CodeExecutionFailed,
		fmt.Sprintf("%s 尚未开发", commandPath),
		"请先使用当前已实现的 P0 命令，或通过 --help 查看已开放能力",
	)
}

// NewPendingCommand 返回一个占位子命令。
// 当前很多 P0/P1 命令还在排期中，这个 helper 可以保证命令树先稳定下来，
// 同时对用户明确说明“命令已注册但尚未开发”。
func NewPendingCommand(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  fmt.Sprintf("%s 已在 CLI 规范中保留，但当前版本尚未开发完成。执行该命令会返回统一的占位提示。", use),
		RunE: func(cmd *cobra.Command, args []string) error {
			return PendingError(cmd.CommandPath())
		},
	}
}

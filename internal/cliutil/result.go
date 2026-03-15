// Package cliutil 放置多个命令都会复用的 CLI 小工具。
package cliutil

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"syskit/internal/domain/model"
	"syskit/internal/output"

	"github.com/spf13/cobra"
)

// RenderCommandResult 按当前全局参数渲染统一结果结构。
// 该方法封装了 format 选择、--output 文件输出以及 quiet 提示语逻辑，
// 避免各命令反复复制同一套流程。
func RenderCommandResult(cmd *cobra.Command, result model.CommandResult, presenter output.Presenter) error {
	format := ResolveFormat(cmd)
	outputPath := ResolveStringFlag(cmd, "output")
	writer := io.Writer(cmd.OutOrStdout())

	closeOutput, err := ConfigureOutputWriter(format, outputPath, &writer)
	if err != nil {
		return err
	}
	if closeOutput != nil {
		defer closeOutput()
	}

	if err := output.Render(writer, format, result, presenter, CSVPrefix(outputPath)); err != nil {
		return err
	}

	if outputPath != "" && format != "csv" && !ResolveBoolFlag(cmd, "quiet") {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "✓ 输出已写入: %s\n", outputPath)
	}

	return nil
}

// CSVPrefix 根据 --output 推导 CSV 前缀。
// 当 output 带扩展名时，去掉扩展名，保证导出文件后缀统一由 presenter 控制。
func CSVPrefix(outputPath string) string {
	if outputPath == "" {
		return ""
	}

	ext := filepath.Ext(outputPath)
	if ext == "" {
		return outputPath
	}

	return strings.TrimSuffix(outputPath, ext)
}

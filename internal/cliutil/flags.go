// Package cliutil 放置多个命令都会复用的 CLI 小工具。
package cliutil

import (
	"strings"

	"github.com/spf13/cobra"
)

// ResolveStringFlag 统一读取命令或继承自根命令的字符串 flag。
// 这样子命令不需要关心 flag 是定义在自己身上还是定义在 root persistent flags 上。
func ResolveStringFlag(cmd *cobra.Command, name string) string {
	if cmd == nil {
		return ""
	}

	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		flag = cmd.InheritedFlags().Lookup(name)
	}
	if flag == nil {
		return ""
	}

	return strings.TrimSpace(flag.Value.String())
}

// ResolveBoolFlag 统一读取布尔型 flag。
func ResolveBoolFlag(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}

	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		flag = cmd.InheritedFlags().Lookup(name)
	}
	if flag == nil {
		return false
	}

	return flag.Value.String() == "true"
}

// ResolveFormat 根据 --json 和 --format 计算最终输出格式。
// 这里保持和 root 命令的优先级一致：--json 始终覆盖 --format。
func ResolveFormat(cmd *cobra.Command) string {
	if ResolveBoolFlag(cmd, "json") {
		return "json"
	}

	format := strings.ToLower(strings.TrimSpace(ResolveStringFlag(cmd, "format")))
	if format == "" {
		return "table"
	}

	return format
}

// SplitCSV 把逗号分隔的字符串拆成去空白、去空值后的切片。
// 常用于处理 --exclude 这类用户输入。
func SplitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

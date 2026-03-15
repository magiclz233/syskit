package cliutil

import (
	"strings"

	"github.com/spf13/cobra"
)

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

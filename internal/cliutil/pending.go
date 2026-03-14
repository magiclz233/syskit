package cliutil

import (
	"fmt"
	"syskit/internal/errs"

	"github.com/spf13/cobra"
)

func PendingError(commandPath string) error {
	return errs.New(errs.ExitExecutionFailed, fmt.Sprintf("%s 尚未开发", commandPath))
}

func NewPendingCommand(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return PendingError(cmd.CommandPath())
		},
	}
}

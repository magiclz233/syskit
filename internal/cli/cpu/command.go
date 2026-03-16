// Package cpu 负责 CPU 相关命令。
package cpu

import (
	"syskit/internal/cliutil"
	cpucollector "syskit/internal/collectors/cpu"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type options struct {
	detail bool
}

// NewCommand 创建 `cpu` 顶层命令。
func NewCommand() *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{
		Use:   "cpu",
		Short: "CPU 总览与分析",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOverview(cmd, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.detail, "detail", false, "显示每核心使用率等详细字段")
	return cmd
}

func runOverview(cmd *cobra.Command, opts *options) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	overview, err := cpucollector.CollectOverview(ctx, cpucollector.CollectOptions{
		Detail: opts.detail,
		TopN:   5,
	})
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("CPU 总览采集完成", overview, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newPresenter(overview, opts.detail))
}

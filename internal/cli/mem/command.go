// Package mem 负责内存相关命令。
package mem

import (
	"syskit/internal/cliutil"
	memcollector "syskit/internal/collectors/mem"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type overviewOptions struct {
	detail bool
}

type topOptions struct {
	topN int
	by   string
	user string
	name string
}

// NewCommand 创建 `mem` 顶层命令。
func NewCommand() *cobra.Command {
	overviewOpts := &overviewOptions{}

	cmd := &cobra.Command{
		Use:   "mem",
		Short: "内存总览与分析",
		Long: "mem 用于输出系统内存总览、可用内存、Swap 使用情况和高内存进程概览。" +
			"\n\n需要进一步按进程排序时，可继续使用 `mem top`。",
		Example: "  syskit mem\n" +
			"  syskit mem --detail\n" +
			"  syskit mem --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOverview(cmd, overviewOpts)
		},
	}

	cmd.Flags().BoolVar(&overviewOpts.detail, "detail", false, "显示缓存、缓冲区等详细字段")
	cmd.AddCommand(newTopCommand())
	return cmd
}

func newTopCommand() *cobra.Command {
	opts := &topOptions{
		topN: 20,
		by:   string(memcollector.SortByRSS),
	}

	cmd := &cobra.Command{
		Use:   "top",
		Short: "查看进程内存排行",
		Long: "mem top 用于按 RSS、VMS 或 Swap 维度输出进程内存排行。" +
			"\n\n可以结合 --user 和 --name 缩小排查范围。",
		Example: "  syskit mem top\n" +
			"  syskit mem top --top 10 --by rss\n" +
			"  syskit mem top --user postgres --name java",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTop(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.topN, "top", 20, "显示 Top N 进程")
	flags.StringVar(&opts.by, "by", string(memcollector.SortByRSS), "排序维度: rss/vms/swap")
	flags.StringVar(&opts.user, "user", "", "按用户名过滤（模糊匹配）")
	flags.StringVar(&opts.name, "name", "", "按进程名或命令行过滤（模糊匹配）")
	return cmd
}

func runOverview(cmd *cobra.Command, opts *overviewOptions) error {
	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	overview, err := memcollector.CollectOverview(ctx, opts.detail, 5)
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("内存总览采集完成", overview, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newOverviewPresenter(overview, opts.detail))
}

func runTop(cmd *cobra.Command, opts *topOptions) error {
	startedAt := time.Now()
	by, err := memcollector.ParseSortBy(opts.by)
	if err != nil {
		return err
	}
	if opts.topN <= 0 {
		return errs.InvalidArgument("--top 必须大于 0")
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	topResult, err := memcollector.CollectTop(ctx, memcollector.TopOptions{
		By:   by,
		TopN: opts.topN,
		User: opts.user,
		Name: opts.name,
	})
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("内存排行采集完成", topResult, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newTopPresenter(topResult))
}

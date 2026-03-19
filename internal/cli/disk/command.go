// Package disk 实现磁盘相关命令。
package disk

import (
	"syskit/internal/cliutil"
	collector "syskit/internal/collectors/disk"
	"syskit/internal/errs"
	"syskit/internal/output"
	"syskit/pkg/utils"
	"time"

	"github.com/spf13/cobra"
)

// overviewOptions 保存 `disk` 总览命令参数。
type overviewOptions struct {
	detail bool
}

// scanOptions 保存 `disk scan` 独有的参数。
type scanOptions struct {
	limit     int
	minSize   string
	depth     int
	exclude   string
	exportCSV string
}

// NewCommand 创建 `disk` 顶层命令。
// `disk` 直接执行时输出磁盘容量总览，`disk scan` 继续承载大文件扫描能力。
func NewCommand() *cobra.Command {
	overviewOpts := &overviewOptions{}

	cmd := &cobra.Command{
		Use:   "disk",
		Short: "磁盘总览与扫描",
		Long: "disk 用于查看分区容量、使用率和剩余空间等总览信息。" +
			"\n\n需要定位膨胀目录或大文件时，请使用 `disk scan <path>` 子命令。",
		Example: "  syskit disk\n" +
			"  syskit disk --detail\n" +
			"  syskit disk --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOverview(cmd, overviewOpts)
		},
	}

	cmd.Flags().BoolVar(&overviewOpts.detail, "detail", false, "显示设备、文件系统和可用容量等详细字段")
	cmd.AddCommand(newScanCommand())
	return cmd
}

// newScanCommand 创建 `disk scan` 子命令并绑定其局部参数。
func newScanCommand() *cobra.Command {
	opts := &scanOptions{
		limit:   20,
		minSize: "0",
		depth:   0,
	}

	cmd := &cobra.Command{
		Use:   "scan <path>",
		Short: "扫描大文件和大目录",
		Long: "disk scan 用于扫描指定目录下的大文件和大目录，输出 Top 结果、总体统计和结构化结果。" +
			"\n\n这是正式扫描入口；根命令已不再承载旧扫描兼容行为。",
		Example: "  syskit disk scan /var/log\n" +
			"  syskit disk scan . --limit 50 --min-size 100MB\n" +
			"  syskit disk scan D:\\logs --exclude node_modules,.git --export-csv report",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.limit, "limit", 20, "显示 Top N 结果")
	flags.StringVar(&opts.minSize, "min-size", "0", "只显示不小于该大小的结果，如 100MB、1GB")
	flags.IntVar(&opts.depth, "depth", 0, "限制扫描深度；0 表示不限制")
	flags.StringVar(&opts.exclude, "exclude", "", "排除的目录（逗号分隔，如: node_modules,.git）")
	flags.StringVar(&opts.exportCSV, "export-csv", "", "CSV 导出路径前缀")

	return cmd
}

// runScan 把命令行参数转换成共享扫描执行器所需的结构，
// 并在进入真正扫描前完成必要的参数校验。
func runScan(cmd *cobra.Command, path string, opts *scanOptions) error {
	minSizeBytes, err := utils.ParseSize(opts.minSize)
	if err != nil {
		return errs.InvalidArgument(err.Error())
	}
	if opts.limit <= 0 {
		return errs.InvalidArgument("--limit 必须大于 0")
	}
	if opts.depth < 0 {
		return errs.InvalidArgument("--depth 不能小于 0")
	}

	consoleOut := cmd.OutOrStdout()
	resultOut := consoleOut
	format := cliutil.ResolveFormat(cmd)
	outputPath := cliutil.ResolveStringFlag(cmd, "output")

	closeOutput, err := cliutil.ConfigureOutputWriter(format, outputPath, &resultOut)
	if err != nil {
		return err
	}
	if closeOutput != nil {
		defer closeOutput()
	}

	return cliutil.RunScan(consoleOut, resultOut, cliutil.ScanRunOptions{
		Path:         path,
		Title:        "磁盘扫描",
		TopN:         opts.limit,
		IncludeFiles: true,
		IncludeDirs:  true,
		ExcludeDirs:  cliutil.SplitCSV(opts.exclude),
		ExportCSV:    opts.exportCSV,
		MinSizeBytes: minSizeBytes,
		MaxDepth:     opts.depth,
		ShowBanner:   true,
	}, cliutil.ScanOutputOptions{
		Format:     format,
		OutputPath: outputPath,
		Quiet:      cliutil.ResolveBoolFlag(cmd, "quiet"),
	})
}

// runOverview 采集并输出磁盘容量总览。
func runOverview(cmd *cobra.Command, opts *overviewOptions) error {
	startedAt := time.Now()

	overview, err := collector.CollectOverview()
	if err != nil {
		return errs.ExecutionFailed("采集磁盘总览失败", err)
	}

	result := output.NewSuccessResult("磁盘总览采集完成", overview, startedAt)
	presenter := output.NewDiskOverviewPresenter(overview, opts.detail)
	return cliutil.RenderCommandResult(cmd, result, presenter)
}

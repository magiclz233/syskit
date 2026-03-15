// Package disk 实现磁盘相关命令。
// 当前已落地 `disk scan`，后续 `disk` 总览会继续在这个包里扩展。
package disk

import (
	"syskit/internal/cliutil"
	"syskit/internal/errs"
	"syskit/pkg/utils"

	"github.com/spf13/cobra"
)

// scanOptions 保存 `disk scan` 独有的参数。
type scanOptions struct {
	limit     int
	minSize   string
	depth     int
	exclude   string
	exportCSV string
}

// NewCommand 创建 `disk` 顶层命令。
// 在 `disk` 总览尚未完成前，直接执行 `disk` 会展示帮助而不是报“未开发”，
// 这样用户仍然可以自然发现已经可用的 `disk scan` 子命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk",
		Short: "磁盘总览与扫描",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

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
		Args:  cobra.ExactArgs(1),
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

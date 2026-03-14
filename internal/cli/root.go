package cli

import (
	"syskit/internal/cli/cpu"
	"syskit/internal/cli/disk"
	"syskit/internal/cli/doctor"
	"syskit/internal/cli/fix"
	"syskit/internal/cli/mem"
	"syskit/internal/cli/policy"
	"syskit/internal/cli/port"
	"syskit/internal/cli/proc"
	"syskit/internal/cli/report"
	"syskit/internal/cli/snapshot"

	"github.com/spf13/cobra"
)

type rootFlags struct {
	topN         int
	excludeDirs  string
	includeFiles bool
	includeDirs  bool
	exportCSV    string
	showVersion  bool
}

func Execute(version string) error {
	return newRootCommand(version).Execute()
}

func newRootCommand(version string) *cobra.Command {
	global := newGlobalOptions()
	opts := &rootFlags{
		topN:         20,
		includeFiles: true,
		includeDirs:  true,
	}

	rootCmd := &cobra.Command{
		Use:   "syskit [path]",
		Short: "跨平台本地系统运维 CLI 工具",
		Long:  "syskit 是一个跨平台本地系统运维 CLI 工具。当前根命令仍保留目录扫描能力，P0 其他命令会按开发清单逐步接入。",
		Example: "  syskit D:\\\n" +
			"  syskit --format json D:\\\n" +
			"  syskit doctor all\n" +
			"  syskit disk scan /var/log",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return global.NormalizeAndValidate()
		},
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLegacyScan(cmd, args, version, opts, global)
		},
	}

	global.Bind(rootCmd)

	flags := rootCmd.Flags()
	flags.IntVarP(&opts.topN, "top", "t", 20, "显示 Top N 结果")
	flags.StringVar(&opts.excludeDirs, "exclude", "", "排除的目录（逗号分隔，如: node_modules,.git）")
	flags.BoolVar(&opts.includeFiles, "include-files", true, "包含文件结果")
	flags.BoolVar(&opts.includeDirs, "include-dirs", true, "包含目录结果")
	flags.StringVar(&opts.exportCSV, "export-csv", "", "CSV 导出路径前缀")
	flags.BoolVarP(&opts.showVersion, "version", "v", false, "显示版本信息")

	rootCmd.AddCommand(
		doctor.NewCommand(),
		port.NewCommand(),
		proc.NewCommand(),
		cpu.NewCommand(),
		mem.NewCommand(),
		disk.NewCommand(),
		fix.NewCommand(),
		snapshot.NewCommand(),
		report.NewCommand(),
		policy.NewCommand(),
	)

	return rootCmd
}

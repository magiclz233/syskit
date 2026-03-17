// Package cli 负责构建 syskit 的根命令和全局初始化流程。
package cli

import (
	"io"
	"strings"
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
	"syskit/internal/config"
	"syskit/internal/errs"
	"syskit/internal/output"
	"syskit/internal/storage"
	"time"

	"github.com/spf13/cobra"
)

// rootFlags 保存根命令兼容扫描模式专有的参数。
// 这些参数不会继承给其他子命令。
type rootFlags struct {
	topN         int
	excludeDirs  string
	includeFiles bool
	includeDirs  bool
	exportCSV    string
	showVersion  bool
}

// Execute 是整个 CLI 的统一入口。
// 它负责执行 Cobra 命令树，并在出错时走统一错误渲染逻辑。
func Execute(version string) error {
	app := newApplication(version)
	err := app.rootCmd.Execute()
	if err == nil {
		return nil
	}
	if shouldSuppressErrorRender(err) {
		return err
	}

	if renderErr := app.renderError(err); renderErr != nil {
		return renderErr
	}

	return err
}

// shouldSuppressErrorRender 用于避免“业务成功输出后又被当作错误再渲染一遍”。
// doctor 会通过特定退出码表达 warning/fail-on 语义，此时只需要保留退出码。
func shouldSuppressErrorRender(err error) bool {
	code := errs.Code(err)
	return code == errs.ExitWarning || code == errs.ExitFailOnMatched
}

// application 把 root command、本次启动时刻以及已加载配置聚合在一起，
// 便于在 Execute、PersistentPreRun 和错误渲染之间共享状态。
type application struct {
	rootCmd     *cobra.Command
	global      *globalOptions
	config      *config.Config
	configPaths []string
	startedAt   time.Time
}

// newApplication 创建根命令、绑定全局参数并注册所有一级子命令。
func newApplication(version string) *application {
	app := &application{
		global:    newGlobalOptions(),
		startedAt: time.Now(),
	}

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
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return app.initialize(cmd)
		},
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLegacyScan(cmd, args, version, opts, app.global)
		},
	}

	app.global.Bind(rootCmd)

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

	app.rootCmd = rootCmd
	return app
}

// initialize 是所有命令执行前的统一初始化入口。
// 当前职责包括：
// 1. 读取关键环境变量；
// 2. 在需要时加载配置；
// 3. 把配置映射到全局参数；
// 4. 做最终参数校验。
func (a *application) initialize(cmd *cobra.Command) error {
	a.global.ApplyBootstrapEnv(cmd)
	if shouldSkipConfigLoad(cmd) {
		return a.global.NormalizeAndValidate()
	}

	loadResult, err := config.Load(config.LoadOptions{ExplicitPath: a.global.config})
	if err != nil {
		return err
	}

	a.config = loadResult.Config
	a.configPaths = append([]string(nil), loadResult.Paths...)
	a.global.ApplyConfig(cmd, loadResult.Config)

	if _, err := storage.Bootstrap(storage.BootstrapOptions{
		DataDir:       loadResult.Config.Storage.DataDir,
		RetentionDays: loadResult.Config.Storage.RetentionDays,
		MaxStorageMB:  loadResult.Config.Storage.MaxStorageMB,
	}); err != nil {
		return err
	}

	return a.global.NormalizeAndValidate()
}

// shouldSkipConfigLoad 用于避免配置管理命令被“当前坏配置”反向阻塞。
// 例如 policy validate 本来就是要去检查配置文件本身，如果这里先加载默认配置，反而会影响使用。
func shouldSkipConfigLoad(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}

	switch strings.ToLower(cmd.CommandPath()) {
	case "syskit policy init", "syskit policy validate":
		return true
	case "syskit policy show":
		flag := cmd.Flags().Lookup("default")
		return flag != nil && flag.Changed && flag.Value.String() == "true"
	default:
		return false
	}
}

// renderError 根据最终输出格式渲染错误结果。
// 它和 main 包分离的原因是：错误输出也要遵守全局 format/output/quiet 规则。
func (a *application) renderError(err error) error {
	format := a.global.errorFormat()
	writer := io.Writer(a.rootCmd.ErrOrStderr())

	if format == "json" || format == "markdown" {
		writer = a.rootCmd.OutOrStdout()
	}

	if a.global.outputPath != "" && format != "csv" {
		fileWriter := writer
		closeWriter, outputErr := a.global.configureOutputWriter(&fileWriter)
		if outputErr == nil {
			defer closeWriter()
			writer = fileWriter
		}
	}

	if renderErr := output.RenderError(writer, format, err, a.startedAt); renderErr != nil {
		return renderErr
	}

	if a.global.outputPath != "" && format != "csv" && !a.global.quiet {
		_, _ = io.WriteString(a.rootCmd.ErrOrStderr(), "✓ 错误输出已写入: "+a.global.outputPath+"\n")
	}

	return nil
}

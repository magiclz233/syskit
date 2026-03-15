package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syskit/internal/config"
	"syskit/internal/errs"
	"time"

	"github.com/spf13/cobra"
)

type globalOptions struct {
	format     string
	json       bool
	outputPath string
	config     string
	policy     string
	quiet      bool
	verbose    bool
	noColor    bool
	timeout    time.Duration
	dryRun     bool
	apply      bool
	yes        bool
	failOn     string
}

func newGlobalOptions() *globalOptions {
	return &globalOptions{
		format:  "table",
		dryRun:  true,
		failOn:  "high",
		timeout: 0,
	}
}

func (o *globalOptions) Bind(rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()
	flags.StringVarP(&o.format, "format", "f", "table", "输出格式: table, json, markdown, csv")
	flags.BoolVar(&o.json, "json", false, "等价于 --format json")
	flags.StringVarP(&o.outputPath, "output", "o", "", "导出文件路径；为空时输出到 stdout")
	flags.StringVar(&o.config, "config", "", "指定配置文件路径")
	flags.StringVar(&o.policy, "policy", "", "指定策略文件路径")
	flags.BoolVarP(&o.quiet, "quiet", "q", false, "仅输出核心结果或错误")
	flags.BoolVarP(&o.verbose, "verbose", "V", false, "输出调试信息")
	flags.BoolVar(&o.noColor, "no-color", false, "禁用颜色输出")
	flags.DurationVar(&o.timeout, "timeout", 0, "覆盖命令超时时间")
	flags.BoolVar(&o.dryRun, "dry-run", true, "写操作默认开启，仅 fix/service/startup/file 等命令生效")
	flags.BoolVar(&o.apply, "apply", false, "真实执行写操作")
	flags.BoolVarP(&o.yes, "yes", "y", false, "跳过危险操作确认")
	flags.StringVar(&o.failOn, "fail-on", "high", "CI 阻断阈值: critical/high/medium/low/never")
}

func (o *globalOptions) NormalizeAndValidate() error {
	if o.json {
		o.format = "json"
	}

	o.format = strings.ToLower(strings.TrimSpace(o.format))
	switch o.format {
	case "table", "json", "markdown", "csv":
	default:
		return errs.InvalidArgument(fmt.Sprintf("不支持的输出格式: %s", o.format))
	}

	o.failOn = strings.ToLower(strings.TrimSpace(o.failOn))
	switch o.failOn {
	case "critical", "high", "medium", "low", "never":
	default:
		return errs.InvalidArgument(fmt.Sprintf("不支持的 --fail-on 值: %s", o.failOn))
	}

	if o.apply {
		o.dryRun = false
	}

	return nil
}

func (o *globalOptions) ApplyBootstrapEnv(cmd *cobra.Command) {
	if !flagChanged(cmd, "config") {
		if value := strings.TrimSpace(os.Getenv("SYSKIT_CONFIG")); value != "" {
			o.config = value
		}
	}

	if !flagChanged(cmd, "policy") {
		if value := strings.TrimSpace(os.Getenv("SYSKIT_POLICY")); value != "" {
			o.policy = value
		}
	}

	if !flagChanged(cmd, "format") && !flagChanged(cmd, "json") {
		if value := strings.TrimSpace(os.Getenv("SYSKIT_OUTPUT")); value != "" {
			o.format = value
		}
	}

	if !flagChanged(cmd, "no-color") {
		if value, ok := parseBoolEnv("SYSKIT_NO_COLOR"); ok {
			o.noColor = value
		}
	}
}

func (o *globalOptions) ApplyConfig(cmd *cobra.Command, cfg *config.Config) {
	if cfg == nil {
		return
	}

	if !flagChanged(cmd, "format") && !flagChanged(cmd, "json") {
		o.format = cfg.Output.Format
	}

	if !flagChanged(cmd, "no-color") {
		o.noColor = cfg.Output.NoColor
	}

	if !flagChanged(cmd, "quiet") {
		o.quiet = cfg.Output.Quiet
	}

	if !flagChanged(cmd, "dry-run") && !flagChanged(cmd, "apply") {
		o.dryRun = cfg.Risk.DryRunDefault
	}
}

func (o *globalOptions) errorFormat() string {
	if o.json {
		return "json"
	}

	format := strings.ToLower(strings.TrimSpace(o.format))
	switch format {
	case "table", "json", "markdown", "csv":
		return format
	default:
		return "table"
	}
}

func (o *globalOptions) configureOutputWriter(out *io.Writer) (func(), error) {
	if o.outputPath == "" || o.format == "csv" {
		return nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(o.outputPath), 0o755); err != nil && filepath.Dir(o.outputPath) != "." {
		return nil, errs.ExecutionFailed("创建输出目录失败", err)
	}

	file, err := os.Create(o.outputPath)
	if err != nil {
		return nil, errs.ExecutionFailed("创建输出文件失败", err)
	}

	*out = file
	return func() {
		_ = file.Close()
	}, nil
}

func (o *globalOptions) csvPrefix(fallback string) string {
	if fallback != "" {
		return fallback
	}

	if o.outputPath == "" {
		return ""
	}

	ext := filepath.Ext(o.outputPath)
	if ext == "" {
		return o.outputPath
	}

	return strings.TrimSuffix(o.outputPath, ext)
}

func flagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}

	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		flag = cmd.InheritedFlags().Lookup(name)
	}
	return flag != nil && flag.Changed
}

func parseBoolEnv(name string) (bool, bool) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false, false
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, false
	}

	return parsed, true
}

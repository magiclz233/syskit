package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		return errs.New(errs.ExitInvalidArgument, fmt.Sprintf("不支持的输出格式: %s", o.format))
	}

	o.failOn = strings.ToLower(strings.TrimSpace(o.failOn))
	switch o.failOn {
	case "critical", "high", "medium", "low", "never":
	default:
		return errs.New(errs.ExitInvalidArgument, fmt.Sprintf("不支持的 --fail-on 值: %s", o.failOn))
	}

	if o.apply {
		o.dryRun = false
	}

	return nil
}

func (o *globalOptions) configureOutputWriter(out *io.Writer) (func(), error) {
	if o.outputPath == "" || o.format == "csv" {
		return nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(o.outputPath), 0o755); err != nil && filepath.Dir(o.outputPath) != "." {
		return nil, errs.Wrap(errs.ExitExecutionFailed, err, "创建输出目录失败")
	}

	file, err := os.Create(o.outputPath)
	if err != nil {
		return nil, errs.Wrap(errs.ExitExecutionFailed, err, "创建输出文件失败")
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

// Package policy 负责 `policy` 命令组。
// 这个命令组是 P0 配置/策略闭环的入口，承担查看、初始化和校验三类职责。
package policy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"syskit/internal/cliutil"
	"syskit/internal/config"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"syskit/internal/output"
	policycfg "syskit/internal/policy"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type showOptions struct {
	kind        string
	defaultOnly bool
}

type initOptions struct {
	kind string
}

type validateOptions struct {
	kind string
}

// showResultData 是 `policy show` 的结构化输出载荷。
type showResultData struct {
	Type    string            `json:"type"`
	Default bool              `json:"default"`
	Config  *configResultData `json:"config,omitempty"`
	Policy  *policyResultData `json:"policy,omitempty"`
}

// configResultData 表示“生效配置 + 来源路径”。
type configResultData struct {
	Sources   []string       `json:"sources,omitempty"`
	Effective *config.Config `json:"effective"`
}

// policyResultData 表示“生效策略 + 来源路径”。
type policyResultData struct {
	Sources   []string          `json:"sources,omitempty"`
	Effective *policycfg.Policy `json:"effective"`
}

// initResultData 是 `policy init` 的结构化输出载荷。
type initResultData struct {
	Type           string   `json:"type"`
	GeneratedFiles []string `json:"generated_files,omitempty"`
	ConfigTemplate string   `json:"config_template,omitempty"`
	PolicyTemplate string   `json:"policy_template,omitempty"`
}

// validateResultData 是 `policy validate` 成功时的结构化输出载荷。
type validateResultData struct {
	Type    string `json:"type"`
	Path    string `json:"path"`
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// NewCommand 创建 `policy` 顶层命令，并注册三个 P0 子命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "配置与策略管理命令",
		Long: "policy 用于查看生效配置、生成默认模板和校验配置/策略文件。" +
			"\n\n其中 `policy validate` 会跳过默认配置加载，只校验你显式指定的目标文件。",
		Example: "  syskit policy show\n" +
			"  syskit policy init --type all --output .syskit\n" +
			"  syskit policy validate config.yaml --type config",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newShowCommand(),
		newInitCommand(),
		newValidateCommand(),
	)

	return cmd
}

// newShowCommand 创建 `policy show`。
func newShowCommand() *cobra.Command {
	opts := &showOptions{kind: "all"}

	cmd := &cobra.Command{
		Use:   "show",
		Short: "查看生效配置和策略",
		Long:  "policy show 用于查看当前最终生效的配置和策略内容，也可以只输出内置默认模板。",
		Example: "  syskit policy show\n" +
			"  syskit policy show --type config\n" +
			"  syskit policy show --default --type policy",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.kind, "type", "all", "输出类型: config, policy, all")
	flags.BoolVar(&opts.defaultOnly, "default", false, "仅输出内置默认模板，不读取本地文件")

	return cmd
}

// newInitCommand 创建 `policy init`。
func newInitCommand() *cobra.Command {
	opts := &initOptions{kind: "all"}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "生成配置或策略模板",
		Long: "policy init 用于生成默认配置模板和策略模板。" +
			"\n\n不传 `--output` 时会直接输出到终端，传入 `--output` 时会写入目标文件或目录。",
		Example: "  syskit policy init\n" +
			"  syskit policy init --type config --output config.yaml\n" +
			"  syskit policy init --type all --output .syskit",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.kind, "type", "all", "生成类型: config, policy, all")
	return cmd
}

// newValidateCommand 创建 `policy validate`。
func newValidateCommand() *cobra.Command {
	opts := &validateOptions{kind: "config"}

	cmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "校验配置或策略文件",
		Long: "policy validate 用于校验单个配置文件或策略文件的格式和字段取值。" +
			"\n\n该命令不会受当前坏配置反向阻塞，适合用在修复配置问题前的预检查流程里。",
		Example: "  syskit policy validate config.yaml --type config\n" +
			"  syskit policy validate policy.yaml --type policy\n" +
			"  syskit policy validate .syskit/config.yaml --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.kind, "type", "config", "校验类型: config, policy")
	return cmd
}

// runShow 根据 --type 和 --default 组合出最终要展示的配置/策略内容。
// 当传入 --default 时，命令只展示内置模板，不读取磁盘上的实际文件。
func runShow(cmd *cobra.Command, opts *showOptions) error {
	startedAt := time.Now()
	kind, err := normalizeType(opts.kind, true)
	if err != nil {
		return err
	}

	data := showResultData{
		Type:    kind,
		Default: opts.defaultOnly,
	}
	sections := make([]documentSection, 0, 2)

	if kind == "config" || kind == "all" {
		cfgData, cfgSection, err := loadConfigSection(cmd, opts.defaultOnly)
		if err != nil {
			return err
		}
		data.Config = cfgData
		sections = append(sections, cfgSection)
	}

	if kind == "policy" || kind == "all" {
		policyData, policySection, err := loadPolicySection(cmd, opts.defaultOnly)
		if err != nil {
			return err
		}
		data.Policy = policyData
		sections = append(sections, policySection)
	}

	result := output.NewSuccessResult("策略查看完成", data, startedAt)
	return renderCommandResult(cmd, result, newDocumentPresenter("策略与配置", sections), cliutil.ResolveStringFlag(cmd, "output"))
}

// runInit 负责生成默认配置模板和策略模板。
// 如果传了 --output，则写文件；否则直接按当前 format 输出到 stdout。
func runInit(cmd *cobra.Command, opts *initOptions) error {
	startedAt := time.Now()
	kind, err := normalizeType(opts.kind, true)
	if err != nil {
		return err
	}

	configTemplate, err := marshalYAML(config.DefaultConfig())
	if err != nil {
		return err
	}

	policyTemplate, err := marshalYAML(policycfg.DefaultPolicy())
	if err != nil {
		return err
	}

	outputTarget := strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "output"))
	generatedFiles, err := writeTemplates(kind, outputTarget, configTemplate, policyTemplate)
	if err != nil {
		return err
	}

	data := initResultData{
		Type:           kind,
		GeneratedFiles: generatedFiles,
	}
	sections := make([]documentSection, 0, 2)

	if len(generatedFiles) == 0 {
		if kind == "config" || kind == "all" {
			data.ConfigTemplate = configTemplate
			sections = append(sections, documentSection{
				Title:   "config",
				Sources: []string{"default"},
				Content: configTemplate,
			})
		}
		if kind == "policy" || kind == "all" {
			data.PolicyTemplate = policyTemplate
			sections = append(sections, documentSection{
				Title:   "policy",
				Sources: []string{"default"},
				Content: policyTemplate,
			})
		}
	} else {
		sections = append(sections, documentSection{
			Title: "generated_files",
			Lines: generatedFiles,
		})
	}

	result := output.NewSuccessResult("模板生成完成", data, startedAt)
	return renderCommandResult(cmd, result, newDocumentPresenter("策略模板", sections), "")
}

// runValidate 校验指定路径是合法配置还是合法策略。
// 这里对 config 校验会禁用环境变量覆盖，确保校验的是“文件本身”而不是“文件 + 当前环境”。
func runValidate(cmd *cobra.Command, path string, opts *validateOptions) error {
	startedAt := time.Now()
	kind, err := normalizeType(opts.kind, false)
	if err != nil {
		return err
	}

	switch kind {
	case "config":
		if _, err := config.Load(config.LoadOptions{
			ExplicitPath:        path,
			DisableEnvOverrides: true,
		}); err != nil {
			return err
		}
	case "policy":
		if _, err := policycfg.Load(policycfg.LoadOptions{ExplicitPath: path}); err != nil {
			return err
		}
	default:
		return errs.InvalidArgument("policy validate 仅支持 --type config 或 --type policy")
	}

	data := validateResultData{
		Type:    kind,
		Path:    path,
		Valid:   true,
		Message: "校验通过",
	}
	sections := []documentSection{
		{
			Title: "validation",
			Lines: []string{
				"type: " + kind,
				"path: " + path,
				"result: passed",
			},
		},
	}

	result := output.NewSuccessResult("校验通过", data, startedAt)
	return renderCommandResult(cmd, result, newDocumentPresenter("策略校验", sections), cliutil.ResolveStringFlag(cmd, "output"))
}

// loadConfigSection 读取配置部分，并把结果转换成“结构化数据 + 文档段落”两种表示。
func loadConfigSection(cmd *cobra.Command, defaultOnly bool) (*configResultData, documentSection, error) {
	var (
		cfg   *config.Config
		paths []string
		err   error
	)

	if defaultOnly {
		cfg = config.DefaultConfig()
	} else {
		result, loadErr := config.Load(config.LoadOptions{ExplicitPath: cliutil.ResolveStringFlag(cmd, "config")})
		if loadErr != nil {
			return nil, documentSection{}, loadErr
		}
		cfg = result.Config
		paths = result.Paths
	}

	content, err := marshalYAML(cfg)
	if err != nil {
		return nil, documentSection{}, err
	}

	return &configResultData{
			Sources:   paths,
			Effective: cfg,
		},
		documentSection{
			Title:   "config",
			Sources: sourceLabels(paths, defaultOnly),
			Content: content,
		},
		nil
}

// loadPolicySection 读取策略部分，并同时生成 presenter 所需的段落内容。
func loadPolicySection(cmd *cobra.Command, defaultOnly bool) (*policyResultData, documentSection, error) {
	var (
		cfg   *policycfg.Policy
		paths []string
		err   error
	)

	if defaultOnly {
		cfg = policycfg.DefaultPolicy()
	} else {
		result, loadErr := policycfg.Load(policycfg.LoadOptions{ExplicitPath: cliutil.ResolveStringFlag(cmd, "policy")})
		if loadErr != nil {
			return nil, documentSection{}, loadErr
		}
		cfg = result.Policy
		paths = result.Paths
	}

	content, err := marshalYAML(cfg)
	if err != nil {
		return nil, documentSection{}, err
	}

	return &policyResultData{
			Sources:   paths,
			Effective: cfg,
		},
		documentSection{
			Title:   "policy",
			Sources: sourceLabels(paths, defaultOnly),
			Content: content,
		},
		nil
}

// normalizeType 统一校验 --type 的取值。
// allowAll 主要用于区分 show/init 和 validate：前两者支持 all，validate 不支持。
func normalizeType(value string, allowAll bool) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "config", "policy":
		return value, nil
	case "all":
		if allowAll {
			return value, nil
		}
	}

	if allowAll {
		return "", errs.InvalidArgument("仅支持 --type config、policy 或 all")
	}
	return "", errs.InvalidArgument("仅支持 --type config 或 --type policy")
}

// marshalYAML 把任意对象序列化成 YAML 文本，用于 show/init 的可读输出。
func marshalYAML(value any) (string, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return "", errs.ExecutionFailed("生成 YAML 失败", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// sourceLabels 统一处理“来源路径”展示。
// 如果没有实际文件参与合并，则用 default 表示来自内置模板。
func sourceLabels(paths []string, defaultOnly bool) []string {
	if defaultOnly || len(paths) == 0 {
		return []string{"default"}
	}
	return paths
}

// renderCommandResult 负责把 policy 子命令的结果按当前 format 输出出去，
// 并处理 --output 文件重定向。
func renderCommandResult(cmd *cobra.Command, result model.CommandResult, presenter output.Presenter, outputPath string) error {
	format := cliutil.ResolveFormat(cmd)
	writer := io.Writer(cmd.OutOrStdout())

	if outputPath != "" && format != "csv" {
		closeWriter, err := configureOutputWriter(format, outputPath, &writer)
		if err != nil {
			return err
		}
		if closeWriter != nil {
			defer closeWriter()
		}
	}

	if err := output.Render(writer, format, result, presenter, csvPrefix(outputPath)); err != nil {
		return err
	}

	if outputPath != "" && format != "csv" && !cliutil.ResolveBoolFlag(cmd, "quiet") {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "✓ 输出已写入: %s\n", outputPath)
	}

	return nil
}

// configureOutputWriter 为 policy 命令打开结构化输出文件。
func configureOutputWriter(format string, outputPath string, out *io.Writer) (func(), error) {
	if outputPath == "" || format == "csv" {
		return nil, nil
	}

	dir := filepath.Dir(outputPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, errs.ExecutionFailed("创建输出目录失败", err)
		}
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return nil, errs.ExecutionFailed("创建输出文件失败", err)
	}

	*out = file
	return func() {
		_ = file.Close()
	}, nil
}

// writeTemplates 根据 --type 和 --output 决定是否落盘以及如何落盘。
// - type=all 时，--output 必须表示目录；
// - type=config/policy 时，--output 可以是文件路径，也可以是已有目录。
func writeTemplates(kind string, outputTarget string, configTemplate string, policyTemplate string) ([]string, error) {
	if outputTarget == "" {
		return nil, nil
	}

	switch kind {
	case "all":
		info, err := os.Stat(outputTarget)
		if err == nil && !info.IsDir() {
			return nil, errs.InvalidArgument("policy init --type all 的 --output 必须是目录")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, errs.ExecutionFailed("检查输出目录失败", err)
		}
		if err := os.MkdirAll(outputTarget, 0o755); err != nil {
			return nil, errs.ExecutionFailed("创建输出目录失败", err)
		}

		configPath := filepath.Join(outputTarget, "config.yaml")
		policyPath := filepath.Join(outputTarget, "policy.yaml")
		if err := writeFile(configPath, configTemplate); err != nil {
			return nil, err
		}
		if err := writeFile(policyPath, policyTemplate); err != nil {
			return nil, err
		}
		return []string{configPath, policyPath}, nil
	case "config":
		path, err := resolveSingleOutputPath(outputTarget, "config.yaml")
		if err != nil {
			return nil, err
		}
		if err := writeFile(path, configTemplate); err != nil {
			return nil, err
		}
		return []string{path}, nil
	case "policy":
		path, err := resolveSingleOutputPath(outputTarget, "policy.yaml")
		if err != nil {
			return nil, err
		}
		if err := writeFile(path, policyTemplate); err != nil {
			return nil, err
		}
		return []string{path}, nil
	default:
		return nil, errs.InvalidArgument("仅支持 --type config、policy 或 all")
	}
}

// resolveSingleOutputPath 解析单文件模板的最终输出路径。
func resolveSingleOutputPath(outputTarget string, fileName string) (string, error) {
	info, err := os.Stat(outputTarget)
	switch {
	case err == nil && info.IsDir():
		return filepath.Join(outputTarget, fileName), nil
	case err == nil:
		return outputTarget, nil
	case errors.Is(err, os.ErrNotExist):
		return outputTarget, nil
	default:
		return "", errs.ExecutionFailed("检查输出路径失败", err)
	}
}

// writeFile 用统一错误协议写模板文件。
func writeFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return errs.ExecutionFailed("创建输出目录失败", err)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		return errs.ExecutionFailed("写入模板文件失败", err)
	}
	return nil
}

// csvPrefix 从结构化输出路径中推导 CSV 前缀。
// 当前 policy 命令暂不支持 CSV，但仍保留这层实现，便于和通用 output.Render 接口对齐。
func csvPrefix(outputPath string) string {
	if outputPath == "" {
		return ""
	}

	ext := filepath.Ext(outputPath)
	if ext == "" {
		return outputPath
	}
	return strings.TrimSuffix(outputPath, ext)
}

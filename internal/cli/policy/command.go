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

type showResultData struct {
	Type    string            `json:"type"`
	Default bool              `json:"default"`
	Config  *configResultData `json:"config,omitempty"`
	Policy  *policyResultData `json:"policy,omitempty"`
}

type configResultData struct {
	Sources   []string       `json:"sources,omitempty"`
	Effective *config.Config `json:"effective"`
}

type policyResultData struct {
	Sources   []string          `json:"sources,omitempty"`
	Effective *policycfg.Policy `json:"effective"`
}

type initResultData struct {
	Type           string   `json:"type"`
	GeneratedFiles []string `json:"generated_files,omitempty"`
	ConfigTemplate string   `json:"config_template,omitempty"`
	PolicyTemplate string   `json:"policy_template,omitempty"`
}

type validateResultData struct {
	Type    string `json:"type"`
	Path    string `json:"path"`
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "配置与策略管理命令",
		Args:  cobra.NoArgs,
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

func newShowCommand() *cobra.Command {
	opts := &showOptions{kind: "all"}

	cmd := &cobra.Command{
		Use:   "show",
		Short: "查看生效配置和策略",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.kind, "type", "all", "输出类型: config, policy, all")
	flags.BoolVar(&opts.defaultOnly, "default", false, "仅输出内置默认模板，不读取本地文件")

	return cmd
}

func newInitCommand() *cobra.Command {
	opts := &initOptions{kind: "all"}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "生成配置或策略模板",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.kind, "type", "all", "生成类型: config, policy, all")
	return cmd
}

func newValidateCommand() *cobra.Command {
	opts := &validateOptions{kind: "config"}

	cmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "校验配置或策略文件",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.kind, "type", "config", "校验类型: config, policy")
	return cmd
}

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

func marshalYAML(value any) (string, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return "", errs.ExecutionFailed("生成 YAML 失败", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func sourceLabels(paths []string, defaultOnly bool) []string {
	if defaultOnly || len(paths) == 0 {
		return []string{"default"}
	}
	return paths
}

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

func writeFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return errs.ExecutionFailed("创建输出目录失败", err)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		return errs.ExecutionFailed("写入模板文件失败", err)
	}
	return nil
}

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

// Package config 除了配置模型外，也负责配置文件加载、默认值合并和路径解析。
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syskit/internal/errs"

	"gopkg.in/yaml.v3"
)

type LoadOptions struct {
	// ExplicitPath 表示命令行显式传入的配置路径。
	// 只要这个字段非空，就不再走系统/用户级自动发现。
	ExplicitPath string
	// DisableEnvOverrides 用于“只验证文件本身”的场景。
	// 例如 policy validate config 时，不希望环境变量把待校验文件覆盖掉。
	DisableEnvOverrides bool
}

// LoadResult 返回最终生效配置以及本次参与合并的配置文件路径。
type LoadResult struct {
	Config *Config
	Paths  []string
}

// Load 按既定优先级加载配置：
// 1. 默认值；
// 2. 显式路径或环境变量指定路径；
// 3. 否则按系统级 -> 用户级顺序叠加；
// 4. 最后叠加环境变量覆盖，并做路径展开和字段校验。
func Load(opts LoadOptions) (*LoadResult, error) {
	cfg := DefaultConfig()
	paths := make([]string, 0, 2)

	explicitPath := strings.TrimSpace(opts.ExplicitPath)
	envConfigPath := strings.TrimSpace(os.Getenv("SYSKIT_CONFIG"))

	switch {
	case explicitPath != "":
		if err := mergeFile(cfg, explicitPath, true); err != nil {
			return nil, err
		}
		paths = append(paths, explicitPath)
	case envConfigPath != "":
		if err := mergeFile(cfg, envConfigPath, true); err != nil {
			return nil, err
		}
		paths = append(paths, envConfigPath)
	default:
		systemPath := SystemConfigPath()
		if err := mergeFile(cfg, systemPath, false); err != nil {
			return nil, err
		}
		if fileExists(systemPath) {
			paths = append(paths, systemPath)
		}

		userPath := UserConfigPath()
		if err := mergeFile(cfg, userPath, false); err != nil {
			return nil, err
		}
		if fileExists(userPath) {
			paths = append(paths, userPath)
		}
	}

	if !opts.DisableEnvOverrides {
		applyEnvOverrides(cfg)
	}
	expandConfiguredPaths(cfg)
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return &LoadResult{
		Config: cfg,
		Paths:  paths,
	}, nil
}

// DefaultConfig 返回内置默认配置。
// 这个默认值既用于真正执行命令时的基线，也用于 policy init 生成配置模板。
func DefaultConfig() *Config {
	return &Config{
		Output: OutputConfig{
			Format:  "table",
			NoColor: false,
			Quiet:   false,
		},
		Logging: LoggingConfig{
			Level:      "info",
			File:       defaultLogPath(),
			MaxSizeMB:  100,
			MaxBackups: 3,
		},
		Storage: StorageConfig{
			DataDir:       defaultDataDir(),
			RetentionDays: 14,
			MaxStorageMB:  500,
		},
		Thresholds: ThresholdsConfig{
			CPUPercent:      80.0,
			MemPercent:      90.0,
			DiskPercent:     85.0,
			ConnectionCount: 1000,
			ProcessCount:    500,
			FileSizeGB:      10.0,
		},
		Risk: RiskConfig{
			RequireConfirmFor: []string{"destructive"},
			DryRunDefault:     true,
		},
		Privacy: PrivacyConfig{
			Redact:        true,
			AllowNoRedact: false,
			RedactFields:  []string{"user", "cmdline", "path"},
		},
		Excludes: ExcludesConfig{
			Paths:     []string{".git", "node_modules", "vendor", "build", "/proc", "/sys"},
			Processes: []string{"systemd", "init"},
			Ports:     []int{22, 443},
		},
		Monitor: MonitorConfig{
			IntervalSec:    5,
			AlertThreshold: 3,
			MaxSamples:     1000,
		},
		Fix: FixConfig{
			BackupBeforeFix: true,
			MaxRetry:        3,
			VerifyAfterFix:  true,
		},
		Report: ReportConfig{
			DefaultFormat:      "markdown",
			IncludeEvidence:    true,
			IncludeSuggestions: true,
		},
	}
}

// SystemConfigPath 返回当前平台的系统级配置路径。
func SystemConfigPath() string {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "syskit", "config.yaml")
	case "darwin":
		return `/Library/Application Support/syskit/config.yaml`
	default:
		return `/etc/syskit/config.yaml`
	}
}

// UserConfigPath 返回当前平台的用户级配置路径。
func UserConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".syskit", "config.yaml")
	}
	return filepath.Join(homeDir, ".syskit", "config.yaml")
}

// mergeFile 把单个 YAML 文件叠加到已有配置对象上。
// required 为 true 时，文件不存在会直接报错；否则按“没有该层配置”处理。
func mergeFile(cfg *Config, path string, required bool) error {
	if !fileExists(path) {
		if required {
			return errs.ConfigInvalid("配置文件不存在: "+path, os.ErrNotExist)
		}
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return errs.ConfigInvalid("读取配置文件失败: "+path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return errs.ConfigInvalid("解析配置文件失败: "+path, err)
	}

	return nil
}

// applyEnvOverrides 处理配置层允许的环境变量覆盖。
// 这里的目标是让自动化和 CI 场景可以不改配置文件，直接通过环境变量切换关键参数。
func applyEnvOverrides(cfg *Config) {
	if value := strings.TrimSpace(os.Getenv("SYSKIT_OUTPUT")); value != "" {
		cfg.Output.Format = value
	}

	if value := strings.TrimSpace(os.Getenv("SYSKIT_NO_COLOR")); value != "" {
		if parsed, ok := parseBool(value); ok {
			cfg.Output.NoColor = parsed
		}
	}

	if value := strings.TrimSpace(os.Getenv("SYSKIT_DATA_DIR")); value != "" {
		cfg.Storage.DataDir = value
	}

	if value := strings.TrimSpace(os.Getenv("SYSKIT_LOG_LEVEL")); value != "" {
		cfg.Logging.Level = value
	}
}

// fileExists 只做最轻量的存在性检查，避免在 Load 里重复展开 os.Stat 逻辑。
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseBool 用于解析布尔型环境变量。
// 返回值中的第二个 bool 用来区分“解析为 false”和“解析失败”。
func parseBool(value string) (bool, bool) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, false
	}
	return parsed, true
}

// defaultLogPath 返回默认日志文件路径。
func defaultLogPath() string {
	base := defaultDataRoot()
	return filepath.Join(base, "logs", "syskit.log")
}

// defaultDataDir 返回默认数据目录路径。
func defaultDataDir() string {
	return filepath.Join(defaultDataRoot(), "data")
}

// defaultDataRoot 返回当前平台的默认数据根目录。
func defaultDataRoot() string {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(".", "data")
		}
		return filepath.Join(base, "syskit")
	case "darwin", "linux":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", ".local", "share", "syskit")
		}
		return filepath.Join(homeDir, ".local", "share", "syskit")
	default:
		return filepath.Join(".", ".local", "share", "syskit")
	}
}

// expandConfiguredPaths 负责把配置里允许写成 ~/xxx 的路径展开成绝对路径。
func expandConfiguredPaths(cfg *Config) {
	cfg.Logging.File = expandHome(cfg.Logging.File)
	cfg.Storage.DataDir = expandHome(cfg.Storage.DataDir)
}

// expandHome 将以 ~ 开头的路径替换为当前用户主目录。
// 如果无法获取主目录，则保留原值，避免在加载阶段直接破坏用户输入。
func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "~" {
		return path
	}
	if !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	trimmed := strings.TrimPrefix(strings.TrimPrefix(path, "~/"), `~\`)
	return filepath.Join(homeDir, trimmed)
}

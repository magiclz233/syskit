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
	ExplicitPath        string
	DisableEnvOverrides bool
}

type LoadResult struct {
	Config *Config
	Paths  []string
}

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

func UserConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".syskit", "config.yaml")
	}
	return filepath.Join(homeDir, ".syskit", "config.yaml")
}

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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseBool(value string) (bool, bool) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, false
	}
	return parsed, true
}

func defaultLogPath() string {
	base := defaultDataRoot()
	return filepath.Join(base, "logs", "syskit.log")
}

func defaultDataDir() string {
	return filepath.Join(defaultDataRoot(), "data")
}

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

func expandConfiguredPaths(cfg *Config) {
	cfg.Logging.File = expandHome(cfg.Logging.File)
	cfg.Storage.DataDir = expandHome(cfg.Storage.DataDir)
}

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

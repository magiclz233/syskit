// Package config 定义 syskit 的配置模型。
// 这里的结构体既服务于 YAML 反序列化，也服务于 JSON 输出，
// 因此字段标签需要同时覆盖 yaml/json 两套协议。
package config

// Config 是 syskit 的顶层配置对象。
// 它负责承载默认参数、阈值、风险控制、存储和报告相关的全局设置。
type Config struct {
	Output     OutputConfig     `yaml:"output" json:"output"`
	Logging    LoggingConfig    `yaml:"logging" json:"logging"`
	Storage    StorageConfig    `yaml:"storage" json:"storage"`
	Thresholds ThresholdsConfig `yaml:"thresholds" json:"thresholds"`
	Risk       RiskConfig       `yaml:"risk" json:"risk"`
	Privacy    PrivacyConfig    `yaml:"privacy" json:"privacy"`
	Excludes   ExcludesConfig   `yaml:"excludes" json:"excludes"`
	Monitor    MonitorConfig    `yaml:"monitor" json:"monitor"`
	Fix        FixConfig        `yaml:"fix" json:"fix"`
	Report     ReportConfig     `yaml:"report" json:"report"`
}

// OutputConfig 控制命令的默认输出行为。
type OutputConfig struct {
	Format  string `yaml:"format" json:"format"`
	NoColor bool   `yaml:"no_color" json:"no_color"`
	Quiet   bool   `yaml:"quiet" json:"quiet"`
}

// LoggingConfig 控制本地日志的级别和滚动策略。
type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	File       string `yaml:"file" json:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb" json:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
}

// StorageConfig 定义快照、报告和缓存等持久化数据的存储策略。
type StorageConfig struct {
	DataDir       string `yaml:"data_dir" json:"data_dir"`
	RetentionDays int    `yaml:"retention_days" json:"retention_days"`
	MaxStorageMB  int    `yaml:"max_storage_mb" json:"max_storage_mb"`
}

// ThresholdsConfig 定义规则和诊断所使用的基础阈值。
type ThresholdsConfig struct {
	CPUPercent      float64 `yaml:"cpu_percent" json:"cpu_percent"`
	MemPercent      float64 `yaml:"mem_percent" json:"mem_percent"`
	DiskPercent     float64 `yaml:"disk_percent" json:"disk_percent"`
	ConnectionCount int     `yaml:"connection_count" json:"connection_count"`
	ProcessCount    int     `yaml:"process_count" json:"process_count"`
	FileSizeGB      float64 `yaml:"file_size_gb" json:"file_size_gb"`
}

// RiskConfig 控制高风险命令的默认行为。
type RiskConfig struct {
	RequireConfirmFor []string `yaml:"require_confirm_for" json:"require_confirm_for"`
	DryRunDefault     bool     `yaml:"dry_run_default" json:"dry_run_default"`
}

// PrivacyConfig 控制输出中的脱敏行为。
type PrivacyConfig struct {
	Redact        bool     `yaml:"redact" json:"redact"`
	AllowNoRedact bool     `yaml:"allow_no_redact" json:"allow_no_redact"`
	RedactFields  []string `yaml:"redact_fields" json:"redact_fields"`
}

// ExcludesConfig 定义全局排除项。
// 这些排除项后续会被扫描、规则判断和其他模块复用。
type ExcludesConfig struct {
	Paths     []string `yaml:"paths" json:"paths"`
	Processes []string `yaml:"processes" json:"processes"`
	Ports     []int    `yaml:"ports" json:"ports"`
}

// MonitorConfig 为后续监控能力预留基础参数。
type MonitorConfig struct {
	IntervalSec    int `yaml:"interval_sec" json:"interval_sec"`
	AlertThreshold int `yaml:"alert_threshold" json:"alert_threshold"`
	MaxSamples     int `yaml:"max_samples" json:"max_samples"`
}

// FixConfig 定义修复类命令的默认保护策略。
type FixConfig struct {
	BackupBeforeFix bool `yaml:"backup_before_fix" json:"backup_before_fix"`
	MaxRetry        int  `yaml:"max_retry" json:"max_retry"`
	VerifyAfterFix  bool `yaml:"verify_after_fix" json:"verify_after_fix"`
}

// ReportConfig 定义报告生成时的默认开关。
type ReportConfig struct {
	DefaultFormat      string `yaml:"default_format" json:"default_format"`
	IncludeEvidence    bool   `yaml:"include_evidence" json:"include_evidence"`
	IncludeSuggestions bool   `yaml:"include_suggestions" json:"include_suggestions"`
}

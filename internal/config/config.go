package config

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

type OutputConfig struct {
	Format  string `yaml:"format" json:"format"`
	NoColor bool   `yaml:"no_color" json:"no_color"`
	Quiet   bool   `yaml:"quiet" json:"quiet"`
}

type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	File       string `yaml:"file" json:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb" json:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
}

type StorageConfig struct {
	DataDir       string `yaml:"data_dir" json:"data_dir"`
	RetentionDays int    `yaml:"retention_days" json:"retention_days"`
	MaxStorageMB  int    `yaml:"max_storage_mb" json:"max_storage_mb"`
}

type ThresholdsConfig struct {
	CPUPercent      float64 `yaml:"cpu_percent" json:"cpu_percent"`
	MemPercent      float64 `yaml:"mem_percent" json:"mem_percent"`
	DiskPercent     float64 `yaml:"disk_percent" json:"disk_percent"`
	ConnectionCount int     `yaml:"connection_count" json:"connection_count"`
	ProcessCount    int     `yaml:"process_count" json:"process_count"`
	FileSizeGB      float64 `yaml:"file_size_gb" json:"file_size_gb"`
}

type RiskConfig struct {
	RequireConfirmFor []string `yaml:"require_confirm_for" json:"require_confirm_for"`
	DryRunDefault     bool     `yaml:"dry_run_default" json:"dry_run_default"`
}

type PrivacyConfig struct {
	Redact        bool     `yaml:"redact" json:"redact"`
	AllowNoRedact bool     `yaml:"allow_no_redact" json:"allow_no_redact"`
	RedactFields  []string `yaml:"redact_fields" json:"redact_fields"`
}

type ExcludesConfig struct {
	Paths     []string `yaml:"paths" json:"paths"`
	Processes []string `yaml:"processes" json:"processes"`
	Ports     []int    `yaml:"ports" json:"ports"`
}

type MonitorConfig struct {
	IntervalSec    int `yaml:"interval_sec" json:"interval_sec"`
	AlertThreshold int `yaml:"alert_threshold" json:"alert_threshold"`
	MaxSamples     int `yaml:"max_samples" json:"max_samples"`
}

type FixConfig struct {
	BackupBeforeFix bool `yaml:"backup_before_fix" json:"backup_before_fix"`
	MaxRetry        int  `yaml:"max_retry" json:"max_retry"`
	VerifyAfterFix  bool `yaml:"verify_after_fix" json:"verify_after_fix"`
}

type ReportConfig struct {
	DefaultFormat      string `yaml:"default_format" json:"default_format"`
	IncludeEvidence    bool   `yaml:"include_evidence" json:"include_evidence"`
	IncludeSuggestions bool   `yaml:"include_suggestions" json:"include_suggestions"`
}

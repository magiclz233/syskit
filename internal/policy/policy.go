package policy

import (
	"os"
	"path/filepath"
	"runtime"
)

type Policy struct {
	Name                 string             `yaml:"name" json:"name"`
	Version              string             `yaml:"version" json:"version"`
	RequiredRules        []RequiredRule     `yaml:"required_rules" json:"required_rules"`
	ThresholdOverrides   ThresholdOverrides `yaml:"threshold_overrides" json:"threshold_overrides"`
	ForbiddenProcesses   []ForbiddenProcess `yaml:"forbidden_processes" json:"forbidden_processes"`
	RequiredServices     []RequiredService  `yaml:"required_services" json:"required_services"`
	RequiredStartupItems []string           `yaml:"required_startup_items" json:"required_startup_items"`
	AllowPublicListen    []string           `yaml:"allow_public_listen" json:"allow_public_listen"`
}

type RequiredRule struct {
	RuleID      string `yaml:"rule_id" json:"rule_id"`
	MaxSeverity string `yaml:"max_severity" json:"max_severity"`
}

type ThresholdOverrides struct {
	CPUPercent      float64 `yaml:"cpu_percent" json:"cpu_percent"`
	MemPercent      float64 `yaml:"mem_percent" json:"mem_percent"`
	DiskPercent     float64 `yaml:"disk_percent" json:"disk_percent"`
	ConnectionCount int     `yaml:"connection_count" json:"connection_count"`
	ProcessCount    int     `yaml:"process_count" json:"process_count"`
	FileSizeGB      float64 `yaml:"file_size_gb" json:"file_size_gb"`
}

type ForbiddenProcess struct {
	Name     string `yaml:"name" json:"name"`
	Severity string `yaml:"severity" json:"severity"`
}

type RequiredService struct {
	Name     string   `yaml:"name" json:"name"`
	Platform []string `yaml:"platform" json:"platform"`
}

func DefaultPolicy() *Policy {
	return &Policy{
		Name:    "team-dev-standard",
		Version: "1.0",
		RequiredRules: []RequiredRule{
			{RuleID: "PORT-001", MaxSeverity: "high"},
			{RuleID: "DISK-001", MaxSeverity: "critical"},
		},
		ThresholdOverrides: ThresholdOverrides{
			CPUPercent:  85.0,
			DiskPercent: 90.0,
		},
		ForbiddenProcesses: []ForbiddenProcess{
			{Name: "bitcoin-miner", Severity: "critical"},
		},
		RequiredServices: []RequiredService{
			{Name: "docker", Platform: []string{"linux", "darwin"}},
			{Name: "Docker Desktop", Platform: []string{"windows"}},
		},
		RequiredStartupItems: []string{},
		AllowPublicListen:    []string{"nginx", "caddy"},
	}
}

func SystemPolicyPath() string {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "syskit", "policy.yaml")
	case "darwin":
		return `/Library/Application Support/syskit/policy.yaml`
	default:
		return `/etc/syskit/policy.yaml`
	}
}

func UserPolicyPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".syskit", "policy.yaml")
	}
	return filepath.Join(homeDir, ".syskit", "policy.yaml")
}

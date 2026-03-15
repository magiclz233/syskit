// Package policy 定义策略文件模型。
// 配置文件负责“默认行为”，策略文件负责“团队标准”，两者职责不同，因此单独拆包。
package policy

import (
	"os"
	"path/filepath"
	"runtime"
)

// Policy 是策略文件的顶层对象。
// 它描述团队要求的规则、阈值覆盖、禁止进程和关键服务等标准化约束。
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

// RequiredRule 表示某条规则在团队策略中的要求。
type RequiredRule struct {
	RuleID      string `yaml:"rule_id" json:"rule_id"`
	MaxSeverity string `yaml:"max_severity" json:"max_severity"`
}

// ThresholdOverrides 表示对默认阈值的策略级覆盖。
type ThresholdOverrides struct {
	CPUPercent      float64 `yaml:"cpu_percent" json:"cpu_percent"`
	MemPercent      float64 `yaml:"mem_percent" json:"mem_percent"`
	DiskPercent     float64 `yaml:"disk_percent" json:"disk_percent"`
	ConnectionCount int     `yaml:"connection_count" json:"connection_count"`
	ProcessCount    int     `yaml:"process_count" json:"process_count"`
	FileSizeGB      float64 `yaml:"file_size_gb" json:"file_size_gb"`
}

// ForbiddenProcess 表示策略中明确禁止出现的进程。
type ForbiddenProcess struct {
	Name     string `yaml:"name" json:"name"`
	Severity string `yaml:"severity" json:"severity"`
}

// RequiredService 表示策略要求必须存在或运行的服务。
type RequiredService struct {
	Name     string   `yaml:"name" json:"name"`
	Platform []string `yaml:"platform" json:"platform"`
}

// DefaultPolicy 返回内置默认策略模板。
// 这个模板主要用于 `policy init` 和 `policy show --default`。
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

// SystemPolicyPath 返回当前平台的系统级策略文件路径。
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

// UserPolicyPath 返回当前平台的用户级策略文件路径。
func UserPolicyPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".syskit", "policy.yaml")
	}
	return filepath.Join(homeDir, ".syskit", "policy.yaml")
}

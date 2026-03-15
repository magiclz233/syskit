package policy

import (
	"fmt"
	"strings"
	"syskit/internal/errs"
)

var knownRuleIDs = map[string]struct{}{
	"PORT-001":    {},
	"PORT-002":    {},
	"PROC-001":    {},
	"PROC-002":    {},
	"CPU-001":     {},
	"MEM-001":     {},
	"DISK-001":    {},
	"DISK-002":    {},
	"FILE-001":    {},
	"ENV-001":     {},
	"NET-001":     {},
	"SVC-001":     {},
	"STARTUP-001": {},
	"LOG-001":     {},
}

func Validate(cfg *Policy) error {
	if cfg == nil {
		return errs.PolicyInvalid("策略不能为空", nil)
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return errs.PolicyInvalid("策略项 name 不能为空", nil)
	}
	if strings.TrimSpace(cfg.Version) == "" {
		return errs.PolicyInvalid("策略项 version 不能为空", nil)
	}

	for index, rule := range cfg.RequiredRules {
		if _, ok := knownRuleIDs[strings.ToUpper(strings.TrimSpace(rule.RuleID))]; !ok {
			return errs.PolicyInvalid(fmt.Sprintf("策略项 required_rules[%d].rule_id 无效: %s", index, rule.RuleID), nil)
		}
		if !isSeverity(rule.MaxSeverity) {
			return errs.PolicyInvalid(fmt.Sprintf("策略项 required_rules[%d].max_severity 无效: %s", index, rule.MaxSeverity), nil)
		}
	}

	if err := validateThresholds(cfg.ThresholdOverrides); err != nil {
		return err
	}

	for index, process := range cfg.ForbiddenProcesses {
		if strings.TrimSpace(process.Name) == "" {
			return errs.PolicyInvalid(fmt.Sprintf("策略项 forbidden_processes[%d].name 不能为空", index), nil)
		}
		if !isSeverity(process.Severity) {
			return errs.PolicyInvalid(fmt.Sprintf("策略项 forbidden_processes[%d].severity 无效: %s", index, process.Severity), nil)
		}
	}

	for index, service := range cfg.RequiredServices {
		if strings.TrimSpace(service.Name) == "" {
			return errs.PolicyInvalid(fmt.Sprintf("策略项 required_services[%d].name 不能为空", index), nil)
		}
		for platformIndex, platform := range service.Platform {
			if !isPlatform(platform) {
				return errs.PolicyInvalid(
					fmt.Sprintf("策略项 required_services[%d].platform[%d] 无效: %s", index, platformIndex, platform),
					nil,
				)
			}
		}
	}

	for index, item := range cfg.RequiredStartupItems {
		if strings.TrimSpace(item) == "" {
			return errs.PolicyInvalid(fmt.Sprintf("策略项 required_startup_items[%d] 不能为空", index), nil)
		}
	}

	for index, item := range cfg.AllowPublicListen {
		if strings.TrimSpace(item) == "" {
			return errs.PolicyInvalid(fmt.Sprintf("策略项 allow_public_listen[%d] 不能为空", index), nil)
		}
	}

	return nil
}

func validateThresholds(cfg ThresholdOverrides) error {
	if cfg.CPUPercent < 0 || cfg.CPUPercent > 100 {
		return errs.PolicyInvalid("策略项 threshold_overrides.cpu_percent 必须在 0 到 100 之间", nil)
	}
	if cfg.MemPercent < 0 || cfg.MemPercent > 100 {
		return errs.PolicyInvalid("策略项 threshold_overrides.mem_percent 必须在 0 到 100 之间", nil)
	}
	if cfg.DiskPercent < 0 || cfg.DiskPercent > 100 {
		return errs.PolicyInvalid("策略项 threshold_overrides.disk_percent 必须在 0 到 100 之间", nil)
	}
	if cfg.ConnectionCount < 0 {
		return errs.PolicyInvalid("策略项 threshold_overrides.connection_count 不能小于 0", nil)
	}
	if cfg.ProcessCount < 0 {
		return errs.PolicyInvalid("策略项 threshold_overrides.process_count 不能小于 0", nil)
	}
	if cfg.FileSizeGB < 0 {
		return errs.PolicyInvalid("策略项 threshold_overrides.file_size_gb 不能小于 0", nil)
	}
	return nil
}

func isSeverity(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "high", "medium", "low":
		return true
	default:
		return false
	}
}

func isPlatform(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "windows", "linux", "darwin":
		return true
	default:
		return false
	}
}

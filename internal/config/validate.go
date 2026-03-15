// Package config 同时负责配置文件的字段级校验。
package config

import (
	"fmt"
	"strings"
	"syskit/internal/errs"
)

// Validate 对最终生效配置做基础 schema 校验和取值校验。
// 这里的策略是只做“协议层”校验，不做过重的业务校验，
// 这样既能尽早拦住坏配置，也不会把模块级逻辑耦合进配置层。
func Validate(cfg *Config) error {
	if cfg == nil {
		return errs.ConfigInvalid("配置不能为空", nil)
	}

	if err := validateEnum("output.format", cfg.Output.Format, "table", "json", "markdown", "csv"); err != nil {
		return err
	}
	if err := validateEnum("logging.level", cfg.Logging.Level, "debug", "info", "warn", "error"); err != nil {
		return err
	}
	if err := validateEnum("report.default_format", cfg.Report.DefaultFormat, "table", "json", "markdown", "csv"); err != nil {
		return err
	}

	if err := validatePercent("thresholds.cpu_percent", cfg.Thresholds.CPUPercent); err != nil {
		return err
	}
	if err := validatePercent("thresholds.mem_percent", cfg.Thresholds.MemPercent); err != nil {
		return err
	}
	if err := validatePercent("thresholds.disk_percent", cfg.Thresholds.DiskPercent); err != nil {
		return err
	}

	if cfg.Logging.MaxSizeMB < 0 {
		return errs.ConfigInvalid("配置项 logging.max_size_mb 不能小于 0", nil)
	}
	if cfg.Logging.MaxBackups < 0 {
		return errs.ConfigInvalid("配置项 logging.max_backups 不能小于 0", nil)
	}
	if cfg.Storage.RetentionDays < 0 {
		return errs.ConfigInvalid("配置项 storage.retention_days 不能小于 0", nil)
	}
	if cfg.Storage.MaxStorageMB < 0 {
		return errs.ConfigInvalid("配置项 storage.max_storage_mb 不能小于 0", nil)
	}
	if cfg.Thresholds.ConnectionCount < 0 {
		return errs.ConfigInvalid("配置项 thresholds.connection_count 不能小于 0", nil)
	}
	if cfg.Thresholds.ProcessCount < 0 {
		return errs.ConfigInvalid("配置项 thresholds.process_count 不能小于 0", nil)
	}
	if cfg.Thresholds.FileSizeGB < 0 {
		return errs.ConfigInvalid("配置项 thresholds.file_size_gb 不能小于 0", nil)
	}
	if cfg.Monitor.IntervalSec <= 0 {
		return errs.ConfigInvalid("配置项 monitor.interval_sec 必须大于 0", nil)
	}
	if cfg.Monitor.AlertThreshold < 0 {
		return errs.ConfigInvalid("配置项 monitor.alert_threshold 不能小于 0", nil)
	}
	if cfg.Monitor.MaxSamples <= 0 {
		return errs.ConfigInvalid("配置项 monitor.max_samples 必须大于 0", nil)
	}
	if cfg.Fix.MaxRetry < 0 {
		return errs.ConfigInvalid("配置项 fix.max_retry 不能小于 0", nil)
	}

	return nil
}

// validateEnum 校验枚举型字段是否落在允许值集合中。
func validateEnum(field string, value string, allowed ...string) error {
	value = strings.TrimSpace(strings.ToLower(value))
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}

	return errs.ConfigInvalid(
		fmt.Sprintf("配置项 %s 取值无效: %s（允许值: %s）", field, value, strings.Join(allowed, "/")),
		nil,
	)
}

// validatePercent 校验百分比字段必须落在 0-100 闭区间内。
func validatePercent(field string, value float64) error {
	if value < 0 || value > 100 {
		return errs.ConfigInvalid(fmt.Sprintf("配置项 %s 必须在 0 到 100 之间", field), nil)
	}
	return nil
}

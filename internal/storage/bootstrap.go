package storage

import (
	"context"
	"time"
)

// BootstrapOptions 定义存储基线初始化参数。
type BootstrapOptions struct {
	DataDir       string
	RetentionDays int
	MaxStorageMB  int
	Now           time.Time
}

// BootstrapResult 描述一次存储基线初始化结果。
type BootstrapResult struct {
	Layout    *Layout         `json:"layout"`
	Retention *RetentionStats `json:"retention"`
}

// Bootstrap 初始化存储目录并执行一次基础保留策略清理。
func Bootstrap(opts BootstrapOptions) (*BootstrapResult, error) {
	layout, err := EnsureLayout(opts.DataDir)
	if err != nil {
		return nil, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	retention, err := ApplyRetention(context.Background(), layout, RetentionPolicy{
		RetentionDays: opts.RetentionDays,
		MaxStorageMB:  opts.MaxStorageMB,
	}, now)
	if err != nil {
		return nil, err
	}

	return &BootstrapResult{
		Layout:    layout,
		Retention: retention,
	}, nil
}

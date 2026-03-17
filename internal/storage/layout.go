// Package storage 负责 syskit 数据目录的布局管理和保留策略执行。
package storage

import (
	"os"
	"path/filepath"
	"strings"
	"syskit/internal/errs"
)

const (
	layoutSnapshots = "snapshots"
	layoutMonitor   = "monitor"
	layoutReports   = "reports"
	layoutAudit     = "audit"

	retentionLockFile = ".retention.lock"
)

// Layout 描述 syskit 在 data_dir 下的目录布局。
type Layout struct {
	RootDir      string `json:"root_dir"`
	SnapshotsDir string `json:"snapshots_dir"`
	MonitorDir   string `json:"monitor_dir"`
	ReportsDir   string `json:"reports_dir"`
	AuditDir     string `json:"audit_dir"`
}

// ManagedDirs 返回需要纳入保留策略扫描的目录列表。
func (l *Layout) ManagedDirs() []string {
	if l == nil {
		return nil
	}
	return []string{
		l.SnapshotsDir,
		l.MonitorDir,
		l.ReportsDir,
		l.AuditDir,
	}
}

// EnsureLayout 确保 data_dir 以及快照、报告、审计等子目录存在。
func EnsureLayout(dataDir string) (*Layout, error) {
	layout, err := buildLayout(dataDir)
	if err != nil {
		return nil, err
	}

	needCreate := []string{
		layout.RootDir,
		layout.SnapshotsDir,
		layout.MonitorDir,
		layout.ReportsDir,
		layout.AuditDir,
	}

	for _, dir := range needCreate {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return nil, errs.ExecutionFailed("创建存储目录失败: "+dir, mkErr)
		}
	}

	return layout, nil
}

func buildLayout(dataDir string) (*Layout, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil, errs.InvalidArgument("storage.data_dir 不能为空")
	}

	root := filepath.Clean(dataDir)
	return &Layout{
		RootDir:      root,
		SnapshotsDir: filepath.Join(root, layoutSnapshots),
		MonitorDir:   filepath.Join(root, layoutMonitor),
		ReportsDir:   filepath.Join(root, layoutReports),
		AuditDir:     filepath.Join(root, layoutAudit),
	}, nil
}

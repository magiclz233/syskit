package storage

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syskit/internal/errs"
	"time"
)

const retentionLockStaleAfter = 30 * time.Minute

// RetentionPolicy 定义基线保留策略。
// RetentionDays 和 MaxStorageMB 设置为 0 时表示禁用对应策略。
type RetentionPolicy struct {
	RetentionDays int `json:"retention_days"`
	MaxStorageMB  int `json:"max_storage_mb"`
}

// RetentionStats 描述一次保留策略执行结果。
type RetentionStats struct {
	ScannedFiles   int   `json:"scanned_files"`
	DeletedFiles   int   `json:"deleted_files"`
	DeletedByAge   int   `json:"deleted_by_age"`
	DeletedBySize  int   `json:"deleted_by_size"`
	FreedBytes     int64 `json:"freed_bytes"`
	RemainingBytes int64 `json:"remaining_bytes"`
}

type managedFile struct {
	path    string
	size    int64
	modTime time.Time
	deleted bool
}

// ApplyRetention 对存储目录执行基础保留策略：
// 1. 按 retention_days 清理过期文件；
// 2. 若仍超 max_storage_mb，则按时间从旧到新继续清理。
func ApplyRetention(ctx context.Context, layout *Layout, policy RetentionPolicy, now time.Time) (*RetentionStats, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if layout == nil {
		return nil, errs.InvalidArgument("存储目录布局不能为空")
	}
	if now.IsZero() {
		now = time.Now()
	}

	release, err := acquireRetentionLock(layout.RootDir, now)
	if err != nil {
		return nil, err
	}
	defer release()

	files, totalBytes, err := collectManagedFiles(ctx, layout.ManagedDirs())
	if err != nil {
		return nil, errs.ExecutionFailed("扫描存储目录失败", err)
	}

	stats := &RetentionStats{
		ScannedFiles:   len(files),
		RemainingBytes: totalBytes,
	}

	if policy.RetentionDays > 0 {
		cutoff := now.AddDate(0, 0, -policy.RetentionDays)
		for _, item := range files {
			if item.deleted || !item.modTime.Before(cutoff) {
				continue
			}

			if rmErr := removeRetentionFile(item.path); rmErr != nil {
				return nil, errs.ExecutionFailed("删除过期存储文件失败: "+item.path, rmErr)
			}
			item.deleted = true

			totalBytes -= item.size
			if totalBytes < 0 {
				totalBytes = 0
			}

			stats.DeletedFiles++
			stats.DeletedByAge++
			stats.FreedBytes += item.size
		}
	}

	if policy.MaxStorageMB > 0 {
		limitBytes := int64(policy.MaxStorageMB) * 1024 * 1024
		if totalBytes > limitBytes {
			candidates := make([]*managedFile, 0, len(files))
			for _, item := range files {
				if item.deleted {
					continue
				}
				candidates = append(candidates, item)
			}

			sort.Slice(candidates, func(i, j int) bool {
				if candidates[i].modTime.Equal(candidates[j].modTime) {
					return candidates[i].path < candidates[j].path
				}
				return candidates[i].modTime.Before(candidates[j].modTime)
			})

			for _, item := range candidates {
				if totalBytes <= limitBytes {
					break
				}
				if rmErr := removeRetentionFile(item.path); rmErr != nil {
					return nil, errs.ExecutionFailed("删除超限存储文件失败: "+item.path, rmErr)
				}
				item.deleted = true

				totalBytes -= item.size
				if totalBytes < 0 {
					totalBytes = 0
				}

				stats.DeletedFiles++
				stats.DeletedBySize++
				stats.FreedBytes += item.size
			}
		}
	}

	stats.RemainingBytes = totalBytes
	return stats, nil
}

func collectManagedFiles(ctx context.Context, dirs []string) ([]*managedFile, int64, error) {
	files := make([]*managedFile, 0, 64)
	var totalBytes int64

	for _, root := range dirs {
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// 跳过符号链接，避免跨目录遍历或循环引用。
			if d.Type()&os.ModeSymlink != 0 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}

			info, infoErr := d.Info()
			if infoErr != nil {
				return infoErr
			}
			if !info.Mode().IsRegular() {
				return nil
			}

			files = append(files, &managedFile{
				path:    path,
				size:    info.Size(),
				modTime: info.ModTime(),
			})
			totalBytes += info.Size()
			return nil
		}); err != nil {
			return nil, 0, err
		}
	}

	return files, totalBytes, nil
}

func acquireRetentionLock(rootDir string, now time.Time) (func(), error) {
	if now.IsZero() {
		now = time.Now()
	}

	lockPath := filepath.Join(rootDir, retentionLockFile)
	createLock := func() (*os.File, error) {
		return os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	}

	lockFile, err := createLock()
	if errors.Is(err, fs.ErrExist) {
		stale, staleErr := isLockStale(lockPath, now)
		if staleErr != nil {
			return nil, errs.ExecutionFailed("检查存储清理锁失败", staleErr)
		}
		if stale {
			if rmErr := os.Remove(lockPath); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
				return nil, errs.ExecutionFailed("移除过期存储清理锁失败", rmErr)
			}
			lockFile, err = createLock()
		}
	}

	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "存储清理任务正在执行，请稍后重试")
		}
		return nil, errs.ExecutionFailed("创建存储清理锁失败", err)
	}

	if _, writeErr := lockFile.WriteString(now.Format(time.RFC3339Nano)); writeErr != nil {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
		return nil, errs.ExecutionFailed("写入存储清理锁失败", writeErr)
	}

	return func() {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
	}, nil
}

func isLockStale(lockPath string, now time.Time) (bool, error) {
	info, err := os.Stat(lockPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return now.Sub(info.ModTime()) > retentionLockStaleAfter, nil
}

func removeRetentionFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

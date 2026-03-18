package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"time"
	"unicode"
)

// SnapshotStore 负责快照文件的增删查。
type SnapshotStore struct {
	layout *Layout
}

// NewSnapshotStore 创建快照存储实例，并确保目录布局存在。
func NewSnapshotStore(dataDir string) (*SnapshotStore, error) {
	layout, err := EnsureLayout(dataDir)
	if err != nil {
		return nil, err
	}
	return &SnapshotStore{layout: layout}, nil
}

// Save 持久化一条快照记录，并返回用于列表展示的摘要。
func (s *SnapshotStore) Save(ctx context.Context, snapshot *model.Snapshot) (*model.SnapshotSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.layout == nil {
		return nil, errs.InvalidArgument("快照存储未初始化")
	}
	if snapshot == nil {
		return nil, errs.InvalidArgument("快照内容不能为空")
	}

	id, err := normalizeSnapshotID(snapshot.ID)
	if err != nil {
		return nil, err
	}
	snapshot.ID = id
	if strings.TrimSpace(snapshot.Name) == "" {
		return nil, errs.InvalidArgument("快照名称不能为空")
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	} else {
		snapshot.CreatedAt = snapshot.CreatedAt.UTC()
	}
	if snapshot.Modules == nil {
		snapshot.Modules = make(map[string]any)
	}

	select {
	case <-ctx.Done():
		return nil, errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
	default:
	}

	filename := snapshotFilename(snapshot.CreatedAt, snapshot.ID)
	filePath := filepath.Join(s.layout.SnapshotsDir, filename)
	tempPath := filePath + ".tmp"

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, errs.ExecutionFailed("序列化快照失败", err)
	}

	if err := os.WriteFile(tempPath, payload, 0o644); err != nil {
		return nil, errs.ExecutionFailed("写入快照临时文件失败", err)
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
		return nil, errs.ExecutionFailed("落盘快照失败", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, errs.ExecutionFailed("读取快照文件信息失败", err)
	}

	summary := buildSnapshotSummary(snapshot, info.Size())
	return &summary, nil
}

// List 返回按创建时间倒序排列的快照摘要。
// limit <= 0 表示返回全部。
func (s *SnapshotStore) List(ctx context.Context, limit int) ([]model.SnapshotSummary, error) {
	entries, err := s.scanSnapshotSummaries(ctx)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

// Load 按 ID 读取快照详情。
func (s *SnapshotStore) Load(ctx context.Context, id string) (*model.Snapshot, error) {
	filePath, err := s.resolveSnapshotPath(ctx, id)
	if err != nil {
		return nil, err
	}
	snapshot, _, err := s.readSnapshotFile(filePath)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

// GetSummary 按 ID 读取单条快照摘要。
func (s *SnapshotStore) GetSummary(ctx context.Context, id string) (*model.SnapshotSummary, error) {
	filePath, err := s.resolveSnapshotPath(ctx, id)
	if err != nil {
		return nil, err
	}
	snapshot, sizeBytes, err := s.readSnapshotFile(filePath)
	if err != nil {
		return nil, err
	}
	summary := buildSnapshotSummary(snapshot, sizeBytes)
	return &summary, nil
}

// Delete 按 ID 删除一条快照，并返回被删除对象摘要。
func (s *SnapshotStore) Delete(ctx context.Context, id string) (*model.SnapshotSummary, error) {
	filePath, err := s.resolveSnapshotPath(ctx, id)
	if err != nil {
		return nil, err
	}
	snapshot, sizeBytes, err := s.readSnapshotFile(filePath)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(filePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, errs.ExecutionFailed("删除快照失败", err)
	}
	summary := buildSnapshotSummary(snapshot, sizeBytes)
	return &summary, nil
}

func (s *SnapshotStore) scanSnapshotSummaries(ctx context.Context) ([]model.SnapshotSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.layout == nil {
		return nil, errs.InvalidArgument("快照存储未初始化")
	}

	files, err := os.ReadDir(s.layout.SnapshotsDir)
	if err != nil {
		return nil, errs.ExecutionFailed("读取快照目录失败", err)
	}

	items := make([]model.SnapshotSummary, 0, len(files))
	for _, item := range files {
		select {
		case <-ctx.Done():
			return nil, errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
		default:
		}

		if item.IsDir() || !strings.HasSuffix(strings.ToLower(item.Name()), ".json") {
			continue
		}

		filePath := filepath.Join(s.layout.SnapshotsDir, item.Name())
		snapshot, sizeBytes, readErr := s.readSnapshotFile(filePath)
		if readErr != nil {
			return nil, readErr
		}
		items = append(items, buildSnapshotSummary(snapshot, sizeBytes))
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func (s *SnapshotStore) resolveSnapshotPath(ctx context.Context, id string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.layout == nil {
		return "", errs.InvalidArgument("快照存储未初始化")
	}

	normalizedID, err := normalizeSnapshotID(id)
	if err != nil {
		return "", err
	}

	files, err := os.ReadDir(s.layout.SnapshotsDir)
	if err != nil {
		return "", errs.ExecutionFailed("读取快照目录失败", err)
	}

	needleWithPrefix := "_" + normalizedID + ".json"
	for _, item := range files {
		select {
		case <-ctx.Done():
			return "", errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
		default:
		}

		if item.IsDir() {
			continue
		}
		name := item.Name()
		lowerName := strings.ToLower(name)
		if lowerName == normalizedID+".json" || strings.HasSuffix(lowerName, strings.ToLower(needleWithPrefix)) {
			return filepath.Join(s.layout.SnapshotsDir, name), nil
		}
	}

	return "", errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, "未找到快照: "+normalizedID)
}

func (s *SnapshotStore) readSnapshotFile(path string) (*model.Snapshot, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, 0, errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, "未找到快照文件: "+filepath.Base(path))
		}
		return nil, 0, errs.ExecutionFailed("读取快照文件失败", err)
	}

	snapshot := &model.Snapshot{}
	if err := json.Unmarshal(data, snapshot); err != nil {
		return nil, 0, errs.ExecutionFailed("解析快照文件失败", err)
	}

	if snapshot.CreatedAt.IsZero() {
		// 兼容早期历史数据未写 created_at 的场景。
		if info, statErr := os.Stat(path); statErr == nil {
			snapshot.CreatedAt = info.ModTime().UTC()
		}
	}
	if snapshot.Modules == nil {
		snapshot.Modules = make(map[string]any)
	}
	return snapshot, int64(len(data)), nil
}

func buildSnapshotSummary(snapshot *model.Snapshot, sizeBytes int64) model.SnapshotSummary {
	modules := make([]string, 0, len(snapshot.Modules))
	for name := range snapshot.Modules {
		modules = append(modules, name)
	}
	sort.Strings(modules)
	return model.SnapshotSummary{
		ID:          snapshot.ID,
		Name:        snapshot.Name,
		Description: snapshot.Description,
		CreatedAt:   snapshot.CreatedAt,
		Host:        snapshot.Host,
		Platform:    snapshot.Platform,
		Modules:     modules,
		SizeBytes:   sizeBytes,
	}
}

func snapshotFilename(createdAt time.Time, id string) string {
	return createdAt.UTC().Format("20060102T150405Z") + "_" + id + ".json"
}

func normalizeSnapshotID(id string) (string, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return "", errs.InvalidArgument("快照 ID 不能为空")
	}
	for _, ch := range id {
		if unicode.IsDigit(ch) || unicode.IsLower(ch) || ch == '-' || ch == '_' {
			continue
		}
		return "", errs.InvalidArgument("快照 ID 仅支持小写字母、数字、-、_")
	}
	return id, nil
}

package filecollector

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syskit/internal/errs"
	"time"
)

// ParseHashMode 解析 hash 参数。
func ParseHashMode(raw string) (HashMode, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return HashSHA256, nil
	}
	switch HashMode(value) {
	case HashMD5, HashSHA256:
		return HashMode(value), nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--hash 仅支持 md5/sha256，当前为: %s", raw))
	}
}

// FindDuplicates 扫描重复文件。
func FindDuplicates(ctx context.Context, opts DupOptions) (*DupResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root := strings.TrimSpace(opts.Path)
	if root == "" {
		return nil, errs.InvalidArgument("扫描路径不能为空")
	}
	root = filepath.Clean(root)
	if opts.MinSize < 0 {
		return nil, errs.InvalidArgument("--min-size 不能小于 0")
	}
	hashMode, err := ParseHashMode(string(opts.Hash))
	if err != nil {
		return nil, err
	}

	scanned := 0
	candidate := 0
	infos := make(map[string]fs.FileInfo, 128)
	bySize := make(map[int64][]string, 64)
	warnings := make([]string, 0, 4)

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			warnings = append(warnings, fmt.Sprintf("扫描失败，已跳过: %s", path))
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if shouldExclude(path, opts.Exclude) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if shouldExclude(path, opts.Exclude) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("读取文件信息失败，已跳过: %s", path))
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		scanned++
		if info.Size() < opts.MinSize {
			return nil
		}
		candidate++
		infos[path] = info
		bySize[info.Size()] = append(bySize[info.Size()], path)
		return nil
	})
	if err != nil {
		return nil, mapContextError(err)
	}

	groups := make([]DuplicateGroup, 0, 16)
	duplicateCount := 0
	wastedBytes := int64(0)

	for size, files := range bySize {
		if len(files) < 2 {
			continue
		}
		byHash := make(map[string][]DuplicateEntry, len(files))
		for _, path := range files {
			hashValue, hashErr := fileHash(path, hashMode)
			if hashErr != nil {
				warnings = append(warnings, fmt.Sprintf("计算 hash 失败，已跳过: %s", path))
				continue
			}
			info := infos[path]
			byHash[hashValue] = append(byHash[hashValue], DuplicateEntry{
				Path:      path,
				SizeBytes: info.Size(),
				ModTime:   info.ModTime().UTC(),
				Hash:      hashValue,
			})
		}
		for hashValue, entries := range byHash {
			if len(entries) < 2 {
				continue
			}
			sort.Slice(entries, func(i int, j int) bool {
				if entries[i].ModTime.Equal(entries[j].ModTime) {
					return entries[i].Path < entries[j].Path
				}
				return entries[i].ModTime.After(entries[j].ModTime)
			})
			group := DuplicateGroup{
				Hash:      hashValue,
				SizeBytes: size,
				Count:     len(entries),
				Files:     entries,
			}
			groups = append(groups, group)
			duplicateCount += len(entries) - 1
			wastedBytes += int64(len(entries)-1) * size
		}
	}

	sort.Slice(groups, func(i int, j int) bool {
		if groups[i].Count != groups[j].Count {
			return groups[i].Count > groups[j].Count
		}
		if groups[i].SizeBytes != groups[j].SizeBytes {
			return groups[i].SizeBytes > groups[j].SizeBytes
		}
		return groups[i].Hash < groups[j].Hash
	})

	return &DupResult{
		Path:           root,
		Hash:           string(hashMode),
		MinSize:        opts.MinSize,
		ScannedFiles:   scanned,
		CandidateFiles: candidate,
		GroupCount:     len(groups),
		DuplicateCount: duplicateCount,
		WastedBytes:    wastedBytes,
		Groups:         groups,
		Warnings:       dedupeWarnings(warnings),
	}, nil
}

// BuildDedupPlan 生成重复文件清理计划。
func BuildDedupPlan(ctx context.Context, opts DupOptions) (*DedupPlan, error) {
	dup, err := FindDuplicates(ctx, opts)
	if err != nil {
		return nil, err
	}
	keep := make([]string, 0, len(dup.Groups))
	deleteFiles := make([]string, 0, dup.DuplicateCount)
	deleteBytes := int64(0)

	for _, group := range dup.Groups {
		if len(group.Files) == 0 {
			continue
		}
		keep = append(keep, group.Files[0].Path)
		for _, entry := range group.Files[1:] {
			deleteFiles = append(deleteFiles, entry.Path)
			deleteBytes += entry.SizeBytes
		}
	}
	sort.Strings(keep)
	sort.Strings(deleteFiles)

	return &DedupPlan{
		Path:         dup.Path,
		Hash:         dup.Hash,
		MinSize:      dup.MinSize,
		ScannedFiles: dup.ScannedFiles,
		Groups:       dup.Groups,
		KeepFiles:    keep,
		DeleteFiles:  deleteFiles,
		DeleteBytes:  deleteBytes,
		Warnings:     dup.Warnings,
	}, nil
}

// ExecuteDedupPlan 执行重复文件删除。
func ExecuteDedupPlan(ctx context.Context, plan *DedupPlan) (*DedupResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if plan == nil {
		return nil, errs.InvalidArgument("dedup 计划不能为空")
	}

	result := &DedupResult{
		Applied:  true,
		Failed:   []string{},
		Warnings: append([]string{}, plan.Warnings...),
	}
	for _, path := range plan.DeleteFiles {
		if err := ctx.Err(); err != nil {
			return nil, mapContextError(err)
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			result.Failed = append(result.Failed, path+": "+statErr.Error())
			continue
		}
		if err := os.Remove(path); err != nil {
			if isPermissionErr(err) {
				result.Failed = append(result.Failed, path+": permission denied")
				continue
			}
			result.Failed = append(result.Failed, path+": "+err.Error())
			continue
		}
		result.DeletedFiles++
		result.DeletedBytes += info.Size()
	}
	if len(result.Failed) > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("共有 %d 个文件删除失败", len(result.Failed)))
	}
	result.Warnings = dedupeWarnings(result.Warnings)
	return result, nil
}

// BuildArchivePlan 生成文件归档计划。
func BuildArchivePlan(ctx context.Context, opts ArchiveOptions) (*ArchivePlan, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root := strings.TrimSpace(opts.Path)
	if root == "" {
		return nil, errs.InvalidArgument("归档路径不能为空")
	}
	root = filepath.Clean(root)
	archivePath := strings.TrimSpace(opts.ArchivePath)
	if archivePath == "" {
		archivePath = filepath.Join(root, ".archive")
	}
	archivePath = filepath.Clean(archivePath)
	if opts.OlderThan <= 0 {
		return nil, errs.InvalidArgument("--older-than 必须大于 0")
	}
	compress := normalizeCompress(opts.Compress)
	if compress == "" {
		return nil, errs.InvalidArgument("--compress 仅支持 gzip/zip")
	}

	now := time.Now()
	scanned := 0
	candidates := make([]ArchiveCandidate, 0, 64)
	warnings := make([]string, 0, 4)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			warnings = append(warnings, "扫描失败，已跳过: "+path)
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(path, archivePath) {
				return filepath.SkipDir
			}
			if shouldExclude(path, opts.Exclude) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || shouldExclude(path, opts.Exclude) {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		scanned++
		age := now.Sub(info.ModTime())
		if age < opts.OlderThan {
			return nil
		}
		candidates = append(candidates, ArchiveCandidate{
			SourcePath: path,
			SizeBytes:  info.Size(),
			ModTime:    info.ModTime().UTC(),
			AgeSec:     int64(age.Seconds()),
		})
		return nil
	})
	if err != nil {
		return nil, mapContextError(err)
	}

	sort.Slice(candidates, func(i int, j int) bool {
		if candidates[i].SizeBytes != candidates[j].SizeBytes {
			return candidates[i].SizeBytes > candidates[j].SizeBytes
		}
		return candidates[i].SourcePath < candidates[j].SourcePath
	})

	totalBytes := int64(0)
	for _, item := range candidates {
		totalBytes += item.SizeBytes
	}
	return &ArchivePlan{
		Path:           root,
		ArchivePath:    archivePath,
		Compress:       compress,
		RetentionSec:   int64(opts.Retention.Seconds()),
		OlderThanSec:   int64(opts.OlderThan.Seconds()),
		ScannedFiles:   scanned,
		CandidateCount: len(candidates),
		CandidateBytes: totalBytes,
		Candidates:     candidates,
		Warnings:       dedupeWarnings(warnings),
	}, nil
}

// ExecuteArchivePlan 执行归档和源文件清理。
func ExecuteArchivePlan(ctx context.Context, plan *ArchivePlan) (*ArchiveResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if plan == nil {
		return nil, errs.InvalidArgument("archive 计划不能为空")
	}
	if err := os.MkdirAll(plan.ArchivePath, 0o755); err != nil {
		return nil, errs.ExecutionFailed("创建归档目录失败", err)
	}

	result := &ArchiveResult{
		Applied:  true,
		Failed:   []string{},
		Warnings: append([]string{}, plan.Warnings...),
	}
	for _, candidate := range plan.Candidates {
		if err := ctx.Err(); err != nil {
			return nil, mapContextError(err)
		}
		dest, err := buildArchiveDest(plan, candidate.SourcePath)
		if err != nil {
			result.Failed = append(result.Failed, candidate.SourcePath+": "+err.Error())
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			result.Failed = append(result.Failed, candidate.SourcePath+": "+err.Error())
			continue
		}

		if err := archiveOne(candidate.SourcePath, dest, plan.Compress); err != nil {
			result.Failed = append(result.Failed, candidate.SourcePath+": "+err.Error())
			continue
		}
		if err := os.Remove(candidate.SourcePath); err != nil {
			result.Failed = append(result.Failed, candidate.SourcePath+": "+err.Error())
			continue
		}
		result.ArchivedCount++
		result.ArchivedBytes += candidate.SizeBytes
	}

	if plan.RetentionSec > 0 {
		removed, err := removeExpiredArchives(ctx, plan.ArchivePath, time.Duration(plan.RetentionSec)*time.Second)
		if err != nil {
			result.Warnings = append(result.Warnings, "归档保留策略执行失败: "+err.Error())
		}
		result.RemovedCount = removed
	}

	if len(result.Failed) > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("共有 %d 个文件归档失败", len(result.Failed)))
	}
	result.Warnings = dedupeWarnings(result.Warnings)
	return result, nil
}

// BuildEmptyPlan 生成空目录清理计划。
func BuildEmptyPlan(ctx context.Context, root string, exclude []string) (*EmptyPlan, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" {
		return nil, errs.InvalidArgument("路径不能为空")
	}

	dirs := make([]string, 0, 64)
	scanned := 0
	warnings := make([]string, 0, 4)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			warnings = append(warnings, "扫描目录失败: "+path)
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if shouldExclude(path, exclude) && path != root {
			return filepath.SkipDir
		}
		scanned++
		dirs = append(dirs, path)
		return nil
	})
	if err != nil {
		return nil, mapContextError(err)
	}

	emptyDirs := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if dir == root {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			warnings = append(warnings, "读取目录失败: "+dir)
			continue
		}
		if len(entries) == 0 {
			emptyDirs = append(emptyDirs, dir)
		}
	}
	sort.Strings(emptyDirs)
	return &EmptyPlan{
		Path:           root,
		ScannedDirs:    scanned,
		EmptyDirs:      emptyDirs,
		CandidateCount: len(emptyDirs),
		Warnings:       dedupeWarnings(warnings),
	}, nil
}

// ExecuteEmptyPlan 执行空目录清理。
func ExecuteEmptyPlan(ctx context.Context, plan *EmptyPlan) (*EmptyResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if plan == nil {
		return nil, errs.InvalidArgument("empty 计划不能为空")
	}
	result := &EmptyResult{
		Applied:  true,
		Failed:   []string{},
		Warnings: append([]string{}, plan.Warnings...),
	}
	// 先删除更深层目录，减少父目录非空导致的失败。
	sorted := append([]string(nil), plan.EmptyDirs...)
	sort.Slice(sorted, func(i int, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	for _, dir := range sorted {
		if err := ctx.Err(); err != nil {
			return nil, mapContextError(err)
		}
		if err := os.Remove(dir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			result.Failed = append(result.Failed, dir+": "+err.Error())
			continue
		}
		result.DeletedDirs++
	}
	if len(result.Failed) > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("共有 %d 个目录删除失败", len(result.Failed)))
	}
	result.Warnings = dedupeWarnings(result.Warnings)
	return result, nil
}

func buildArchiveDest(plan *ArchivePlan, source string) (string, error) {
	rel, err := filepath.Rel(plan.Path, source)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = filepath.Base(source)
	}
	dest := filepath.Join(plan.ArchivePath, rel)
	switch plan.Compress {
	case "gzip":
		return dest + ".gz", nil
	case "zip":
		return dest + ".zip", nil
	default:
		return "", errs.InvalidArgument("不支持的压缩格式")
	}
}

func archiveOne(source string, dest string, compress string) error {
	switch compress {
	case "gzip":
		return writeGzip(source, dest)
	case "zip":
		return writeZip(source, dest)
	default:
		return errs.InvalidArgument("不支持的压缩格式")
	}
}

func writeGzip(source string, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		return err
	}
	return gz.Close()
}

func writeZip(source string, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	entry, err := zw.Create(filepath.Base(source))
	if err != nil {
		_ = zw.Close()
		return err
	}
	if _, err := io.Copy(entry, in); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func removeExpiredArchives(ctx context.Context, root string, retention time.Duration) (int, error) {
	if retention <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-retention)
	removed := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err == nil || errors.Is(err, os.ErrNotExist) {
				removed++
			}
		}
		return nil
	})
	if err != nil {
		return removed, mapContextError(err)
	}
	return removed, nil
}

func fileHash(path string, mode HashMode) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	switch mode {
	case HashMD5:
		sum := md5.New()
		if _, err := io.Copy(sum, file); err != nil {
			return "", err
		}
		return hex.EncodeToString(sum.Sum(nil)), nil
	default:
		sum := sha256.New()
		if _, err := io.Copy(sum, file); err != nil {
			return "", err
		}
		return hex.EncodeToString(sum.Sum(nil)), nil
	}
}

func shouldExclude(path string, excludes []string) bool {
	if len(excludes) == 0 {
		return false
	}
	lower := strings.ToLower(path)
	for _, item := range excludes {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if strings.Contains(lower, item) {
			return true
		}
	}
	return false
}

func normalizeCompress(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "gzip", "zip":
		if value == "" {
			return "gzip"
		}
		return value
	default:
		return ""
	}
}

func dedupeWarnings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		set[item] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for item := range set {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func mapContextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "文件治理命令执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "文件治理命令已取消")
	}
	if isPermissionErr(err) {
		return errs.PermissionDenied("文件治理操作权限不足", "请提升权限后重试")
	}
	return errs.ExecutionFailed("文件治理命令执行失败", err)
}

func isPermissionErr(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "permission denied") ||
		strings.Contains(text, "access denied") ||
		strings.Contains(text, "operation not permitted")
}

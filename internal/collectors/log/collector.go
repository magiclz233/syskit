package logcollector

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syskit/internal/errs"
	"time"
)

var allowedLevels = map[string]struct{}{
	"all":   {},
	"error": {},
	"warn":  {},
	"info":  {},
	"debug": {},
}

// NormalizeLevel 解析日志级别过滤参数。
func NormalizeLevel(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "all", nil
	}
	if _, ok := allowedLevels[value]; !ok {
		return "", errs.InvalidArgument(fmt.Sprintf("不支持的日志级别: %s", raw))
	}
	return value, nil
}

// Analyze 统计日志总览。
func Analyze(ctx context.Context, opts OverviewOptions) (*OverviewResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	level, err := NormalizeLevel(opts.Level)
	if err != nil {
		return nil, err
	}
	if opts.Top <= 0 {
		opts.Top = 20
	}

	files, warnings := normalizeFiles(opts.Files)
	result := &OverviewResult{
		Level:       level,
		SinceSec:    int64(opts.Since.Seconds()),
		Top:         opts.Top,
		Detail:      opts.Detail,
		FileCount:   len(files),
		LevelCounts: map[string]int{"error": 0, "warn": 0, "info": 0, "debug": 0, "other": 0},
		Files:       make([]OverviewFileStat, 0, len(files)),
		TopMessages: []TopMessage{},
		Samples:     []LogSample{},
		Warnings:    warnings,
	}

	if len(files) == 0 {
		result.Warnings = appendUnique(result.Warnings, "没有可分析的日志文件")
		return result, nil
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := time.Time{}
	if opts.Since > 0 {
		cutoff = now.Add(-opts.Since)
	}

	messageCount := make(map[string]int, 64)
	for _, path := range files {
		if err := ctx.Err(); err != nil {
			return nil, mapContextError(err)
		}
		stat, statErr := os.Stat(path)
		if statErr != nil {
			result.Warnings = appendUnique(result.Warnings, "读取日志文件失败: "+path)
			continue
		}
		if stat.IsDir() {
			result.Warnings = appendUnique(result.Warnings, "日志路径是目录，已跳过: "+path)
			continue
		}

		fileStat := OverviewFileStat{
			Path:       path,
			SizeBytes:  stat.Size(),
			ModifiedAt: stat.ModTime().UTC(),
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			if isPermissionErr(openErr) {
				return nil, errs.PermissionDenied("读取日志文件失败", "请提升权限后重试")
			}
			return nil, errs.ExecutionFailed("打开日志文件失败: "+path, openErr)
		}

		scanner := bufio.NewScanner(file)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			fileStat.TotalLines++
			result.TotalLines++

			logTime, hasTime := parseLogTime(line)
			if !inSinceWindow(cutoff, hasTime, logTime, stat.ModTime()) {
				continue
			}
			lvl := detectLevel(line)
			result.LevelCounts[levelKey(lvl)]++

			if !matchLevel(level, lvl) {
				continue
			}
			fileStat.MatchedLines++
			result.MatchedLines++

			switch lvl {
			case "error":
				fileStat.ErrorLines++
			case "warn":
				fileStat.WarnLines++
			case "info":
				fileStat.InfoLines++
			case "debug":
				fileStat.DebugLines++
			}

			msg := normalizeMessage(line)
			messageCount[msg]++
			if opts.Detail && len(result.Samples) < opts.Top {
				result.Samples = append(result.Samples, LogSample{
					File:      path,
					Line:      lineNo,
					Level:     lvl,
					Timestamp: logTime.UTC(),
					Text:      line,
				})
			}
		}
		_ = file.Close()
		if scanErr := scanner.Err(); scanErr != nil {
			result.Warnings = appendUnique(result.Warnings, "读取日志内容失败: "+path)
		}
		result.Files = append(result.Files, fileStat)
	}

	if result.MatchedLines > 0 {
		errorCount := result.LevelCounts["error"]
		result.ErrorRate = float64(errorCount) * 100 / float64(result.MatchedLines)
	}
	result.TopMessages = pickTopMessages(messageCount, opts.Top)
	sort.Slice(result.Files, func(i, j int) bool {
		if result.Files[i].MatchedLines != result.Files[j].MatchedLines {
			return result.Files[i].MatchedLines > result.Files[j].MatchedLines
		}
		return result.Files[i].Path < result.Files[j].Path
	})
	return result, nil
}

// Search 在日志中检索关键字。
func Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	keyword := strings.TrimSpace(opts.Keyword)
	if keyword == "" {
		return nil, errs.InvalidArgument("搜索关键字不能为空")
	}
	if opts.Context < 0 {
		return nil, errs.InvalidArgument("--context 不能小于 0")
	}

	files, warnings := normalizeFiles(opts.Files)
	result := &SearchResult{
		Keyword:      keyword,
		SinceSec:     int64(opts.Since.Seconds()),
		Context:      opts.Context,
		IgnoreCase:   opts.IgnoreCase,
		FileCount:    len(files),
		TotalMatches: 0,
		Matches:      []SearchMatch{},
		Warnings:     warnings,
	}
	if len(files) == 0 {
		result.Warnings = appendUnique(result.Warnings, "没有可搜索的日志文件")
		return result, nil
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := time.Time{}
	if opts.Since > 0 {
		cutoff = now.Add(-opts.Since)
	}

	matchText := keyword
	if opts.IgnoreCase {
		matchText = strings.ToLower(keyword)
	}

	for _, path := range files {
		if err := ctx.Err(); err != nil {
			return nil, mapContextError(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				result.Warnings = appendUnique(result.Warnings, "日志文件不存在: "+path)
				continue
			}
			if isPermissionErr(err) {
				return nil, errs.PermissionDenied("读取日志文件失败", "请提升权限后重试")
			}
			return nil, errs.ExecutionFailed("读取日志文件失败: "+path, err)
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		fileInfo, _ := os.Stat(path)
		modTime := time.Time{}
		if fileInfo != nil {
			modTime = fileInfo.ModTime()
		}

		for idx, line := range lines {
			logTime, hasTime := parseLogTime(line)
			if !inSinceWindow(cutoff, hasTime, logTime, modTime) {
				continue
			}
			needle := line
			if opts.IgnoreCase {
				needle = strings.ToLower(line)
			}
			if !strings.Contains(needle, matchText) {
				continue
			}

			match := SearchMatch{
				File:      path,
				Line:      idx + 1,
				Level:     detectLevel(line),
				Timestamp: logTime.UTC(),
				Text:      line,
			}
			if opts.Context > 0 {
				start := idx - opts.Context
				if start < 0 {
					start = 0
				}
				end := idx + opts.Context + 1
				if end > len(lines) {
					end = len(lines)
				}
				match.Before = append([]string{}, lines[start:idx]...)
				match.After = append([]string{}, lines[idx+1:end]...)
			}
			result.Matches = append(result.Matches, match)
		}
	}

	result.TotalMatches = len(result.Matches)
	return result, nil
}

// Watch 监控日志增长和错误率变化。
func Watch(ctx context.Context, opts WatchOptions) (*WatchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	files, warnings := normalizeFiles(opts.Files)
	if len(files) == 0 {
		return nil, errs.InvalidArgument("没有可监控的日志文件")
	}
	if opts.Interval <= 0 {
		return nil, errs.InvalidArgument("--interval 必须大于 0")
	}
	if opts.ThresholdSize < 0 {
		return nil, errs.InvalidArgument("--threshold-size 不能小于 0")
	}
	if opts.ThresholdError < 0 || opts.ThresholdError > 100 {
		return nil, errs.InvalidArgument("--threshold-error 必须在 0 到 100 之间")
	}

	states := make(map[string]int64, len(files))
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				states[file] = 0
				warnings = appendUnique(warnings, "日志文件不存在，将等待后续创建: "+file)
				continue
			}
			return nil, errs.ExecutionFailed("读取日志文件失败: "+file, err)
		}
		states[file] = info.Size()
	}

	result := &WatchResult{
		IntervalMs:       opts.Interval.Milliseconds(),
		ThresholdSize:    opts.ThresholdSize,
		ThresholdError:   opts.ThresholdError,
		Samples:          []WatchSample{},
		Alerts:           []WatchAlert{},
		Warnings:         warnings,
		StoppedReason:    "completed",
		TotalGrowthBytes: 0,
	}

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			result.StoppedReason = stopReason(ctx.Err())
			break loop
		case <-ticker.C:
		}

		sample := WatchSample{Timestamp: time.Now().UTC()}
		for _, file := range files {
			growth, newLines, errorLines, nextOffset, err := readFileGrowth(file, states[file])
			if err != nil {
				result.Warnings = appendUnique(result.Warnings, "读取日志增量失败: "+file)
				continue
			}
			states[file] = nextOffset
			sample.GrowthBytes += growth
			sample.NewLines += newLines
			sample.ErrorLines += errorLines
		}
		if sample.NewLines > 0 {
			sample.ErrorRate = float64(sample.ErrorLines) * 100 / float64(sample.NewLines)
		}

		result.Samples = append(result.Samples, sample)
		result.SampleCount++
		result.TotalGrowthBytes += sample.GrowthBytes
		result.TotalNewLines += sample.NewLines
		result.TotalErrorLines += sample.ErrorLines

		if opts.ThresholdSize > 0 && sample.GrowthBytes >= opts.ThresholdSize {
			result.Alerts = append(result.Alerts, WatchAlert{
				Type:      "growth",
				Summary:   fmt.Sprintf("日志增量达到阈值（%d bytes）", sample.GrowthBytes),
				Threshold: float64(opts.ThresholdSize),
				Value:     float64(sample.GrowthBytes),
				Timestamp: sample.Timestamp,
			})
		}
		if opts.ThresholdError > 0 && sample.ErrorRate >= opts.ThresholdError {
			result.Alerts = append(result.Alerts, WatchAlert{
				Type:      "error_rate",
				Summary:   fmt.Sprintf("错误率达到阈值（%.2f%%）", sample.ErrorRate),
				Threshold: opts.ThresholdError,
				Value:     sample.ErrorRate,
				Timestamp: sample.Timestamp,
			})
		}
		if opts.MaxSamples > 0 && result.SampleCount >= opts.MaxSamples {
			result.StoppedReason = "max_samples"
			break loop
		}
	}
	return result, nil
}

func readFileGrowth(path string, offset int64) (int64, int, int, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, 0, 0, 0, nil
		}
		return 0, 0, 0, offset, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, 0, 0, offset, err
	}
	size := info.Size()
	if size < offset {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return 0, 0, 0, offset, err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return 0, 0, 0, offset, err
	}
	growth := size - offset
	if growth < 0 {
		growth = 0
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	newLines := 0
	errorLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		newLines++
		if detectLevel(line) == "error" {
			errorLines++
		}
	}
	return growth, newLines, errorLines, size, nil
}

func normalizeFiles(raw []string) ([]string, []string) {
	seen := make(map[string]struct{}, len(raw))
	result := make([]string, 0, len(raw))
	warnings := make([]string, 0, 1)

	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		matches, err := filepath.Glob(item)
		if err != nil || len(matches) == 0 {
			// 对非 glob 路径保持兼容，按原始值入列，由后续读取逻辑给出具体错误。
			matches = []string{item}
		}
		for _, path := range matches {
			path = filepath.Clean(path)
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			result = append(result, path)
		}
	}
	if len(result) == 0 {
		warnings = append(warnings, "未解析到日志文件路径")
	}
	sort.Strings(result)
	return result, warnings
}

func parseLogTime(line string) (time.Time, bool) {
	text := strings.TrimSpace(line)
	if text == "" {
		return time.Time{}, false
	}
	token := text
	if idx := strings.IndexAny(text, " \t"); idx > 0 {
		token = text[:idx]
	}

	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	} {
		if parsed, err := time.Parse(layout, token); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func inSinceWindow(cutoff time.Time, hasLogTime bool, logTime time.Time, fileModTime time.Time) bool {
	if cutoff.IsZero() {
		return true
	}
	if hasLogTime {
		return !logTime.Before(cutoff)
	}
	return !fileModTime.Before(cutoff)
}

func detectLevel(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error"), strings.Contains(lower, "fatal"), strings.Contains(lower, "panic"):
		return "error"
	case strings.Contains(lower, "warn"):
		return "warn"
	case strings.Contains(lower, "info"):
		return "info"
	case strings.Contains(lower, "debug"), strings.Contains(lower, "trace"):
		return "debug"
	default:
		return "other"
	}
}

func levelKey(level string) string {
	switch level {
	case "error", "warn", "info", "debug":
		return level
	default:
		return "other"
	}
}

func matchLevel(filter string, level string) bool {
	if filter == "all" || filter == "" {
		return true
	}
	return filter == level
}

func normalizeMessage(line string) string {
	text := strings.TrimSpace(line)
	if len(text) > 120 {
		text = text[:120]
	}
	return text
}

func pickTopMessages(count map[string]int, top int) []TopMessage {
	if len(count) == 0 || top <= 0 {
		return []TopMessage{}
	}
	items := make([]TopMessage, 0, len(count))
	for message, c := range count {
		items = append(items, TopMessage{Message: message, Count: c})
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Message < items[j].Message
	})
	if len(items) > top {
		items = items[:top]
	}
	return items
}

func appendUnique(items []string, values ...string) []string {
	set := make(map[string]struct{}, len(items)+len(values))
	result := make([]string, 0, len(items)+len(values))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := set[item]; ok {
			continue
		}
		set[item] = struct{}{}
		result = append(result, item)
	}
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := set[item]; ok {
			continue
		}
		set[item] = struct{}{}
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func mapContextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "日志命令执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "日志命令已取消")
	}
	return errs.ExecutionFailed("日志命令执行失败", err)
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

func stopReason(err error) string {
	if err == nil {
		return "completed"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "interrupted"
}

// Package logcollector 提供日志体检、检索与监控能力。
package logcollector

import "time"

// OverviewOptions 定义 `log` 总览参数。
type OverviewOptions struct {
	Files  []string
	Since  time.Duration
	Level  string
	Top    int
	Detail bool
	Now    time.Time
}

// OverviewFileStat 是单文件统计结果。
type OverviewFileStat struct {
	Path         string    `json:"path"`
	SizeBytes    int64     `json:"size_bytes"`
	ModifiedAt   time.Time `json:"modified_at"`
	TotalLines   int       `json:"total_lines"`
	MatchedLines int       `json:"matched_lines"`
	ErrorLines   int       `json:"error_lines"`
	WarnLines    int       `json:"warn_lines"`
	InfoLines    int       `json:"info_lines"`
	DebugLines   int       `json:"debug_lines"`
}

// TopMessage 是高频日志摘要。
type TopMessage struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// LogSample 是日志样本行。
type LogSample struct {
	File      string    `json:"file"`
	Line      int       `json:"line"`
	Level     string    `json:"level"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Text      string    `json:"text"`
}

// OverviewResult 是 `log` 总览输出。
type OverviewResult struct {
	Level        string             `json:"level"`
	SinceSec     int64              `json:"since_sec"`
	Top          int                `json:"top"`
	Detail       bool               `json:"detail"`
	FileCount    int                `json:"file_count"`
	TotalLines   int                `json:"total_lines"`
	MatchedLines int                `json:"matched_lines"`
	ErrorRate    float64            `json:"error_rate"`
	LevelCounts  map[string]int     `json:"level_counts"`
	Files        []OverviewFileStat `json:"files"`
	TopMessages  []TopMessage       `json:"top_messages"`
	Samples      []LogSample        `json:"samples,omitempty"`
	Warnings     []string           `json:"warnings,omitempty"`
}

// SearchOptions 定义 `log search` 参数。
type SearchOptions struct {
	Files      []string
	Keyword    string
	Since      time.Duration
	IgnoreCase bool
	Context    int
	Now        time.Time
}

// SearchMatch 是单条匹配结果。
type SearchMatch struct {
	File      string    `json:"file"`
	Line      int       `json:"line"`
	Level     string    `json:"level"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Text      string    `json:"text"`
	Before    []string  `json:"before,omitempty"`
	After     []string  `json:"after,omitempty"`
}

// SearchResult 是 `log search` 输出。
type SearchResult struct {
	Keyword      string        `json:"keyword"`
	SinceSec     int64         `json:"since_sec"`
	Context      int           `json:"context"`
	IgnoreCase   bool          `json:"ignore_case"`
	FileCount    int           `json:"file_count"`
	TotalMatches int           `json:"total_matches"`
	Matches      []SearchMatch `json:"matches"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// WatchOptions 定义 `log watch` 参数。
type WatchOptions struct {
	Files          []string
	ThresholdSize  int64
	ThresholdError float64
	Interval       time.Duration
	MaxSamples     int
}

// WatchSample 是单次监控采样结果。
type WatchSample struct {
	Timestamp   time.Time `json:"timestamp"`
	GrowthBytes int64     `json:"growth_bytes"`
	NewLines    int       `json:"new_lines"`
	ErrorLines  int       `json:"error_lines"`
	ErrorRate   float64   `json:"error_rate"`
}

// WatchAlert 是触发告警事件。
type WatchAlert struct {
	Type      string    `json:"type"`
	Summary   string    `json:"summary"`
	Threshold float64   `json:"threshold"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// WatchResult 是 `log watch` 输出。
type WatchResult struct {
	IntervalMs       int64         `json:"interval_ms"`
	ThresholdSize    int64         `json:"threshold_size"`
	ThresholdError   float64       `json:"threshold_error"`
	SampleCount      int           `json:"sample_count"`
	StoppedReason    string        `json:"stopped_reason"`
	TotalGrowthBytes int64         `json:"total_growth_bytes"`
	TotalNewLines    int           `json:"total_new_lines"`
	TotalErrorLines  int           `json:"total_error_lines"`
	Alerts           []WatchAlert  `json:"alerts"`
	Samples          []WatchSample `json:"samples"`
	Warnings         []string      `json:"warnings,omitempty"`
}

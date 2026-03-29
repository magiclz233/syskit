// Package filecollector 提供文件治理能力。
package filecollector

import "time"

// HashMode 定义重复文件计算方式。
type HashMode string

const (
	// HashMD5 使用 MD5。
	HashMD5 HashMode = "md5"
	// HashSHA256 使用 SHA256。
	HashSHA256 HashMode = "sha256"
)

// DupOptions 定义 `file dup` 参数。
type DupOptions struct {
	Path    string
	MinSize int64
	Exclude []string
	Hash    HashMode
}

// DuplicateEntry 是重复文件条目。
type DuplicateEntry struct {
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	ModTime   time.Time `json:"mod_time"`
	Hash      string    `json:"hash"`
}

// DuplicateGroup 是同 hash 组。
type DuplicateGroup struct {
	Hash      string           `json:"hash"`
	SizeBytes int64            `json:"size_bytes"`
	Count     int              `json:"count"`
	Files     []DuplicateEntry `json:"files"`
}

// DupResult 是 `file dup` 输出。
type DupResult struct {
	Path           string           `json:"path"`
	Hash           string           `json:"hash"`
	MinSize        int64            `json:"min_size"`
	ScannedFiles   int              `json:"scanned_files"`
	CandidateFiles int              `json:"candidate_files"`
	GroupCount     int              `json:"group_count"`
	DuplicateCount int              `json:"duplicate_count"`
	WastedBytes    int64            `json:"wasted_bytes"`
	Groups         []DuplicateGroup `json:"groups"`
	Warnings       []string         `json:"warnings,omitempty"`
}

// DedupPlan 是 `file dedup` dry-run 计划。
type DedupPlan struct {
	Path         string           `json:"path"`
	Hash         string           `json:"hash"`
	MinSize      int64            `json:"min_size"`
	ScannedFiles int              `json:"scanned_files"`
	Groups       []DuplicateGroup `json:"groups"`
	KeepFiles    []string         `json:"keep_files"`
	DeleteFiles  []string         `json:"delete_files"`
	DeleteBytes  int64            `json:"delete_bytes"`
	Warnings     []string         `json:"warnings,omitempty"`
}

// DedupResult 是 `file dedup` apply 结果。
type DedupResult struct {
	Applied      bool     `json:"applied"`
	DeletedFiles int      `json:"deleted_files"`
	DeletedBytes int64    `json:"deleted_bytes"`
	Failed       []string `json:"failed,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

// ArchiveOptions 定义归档参数。
type ArchiveOptions struct {
	Path        string
	OlderThan   time.Duration
	ArchivePath string
	Compress    string
	Retention   time.Duration
	Exclude     []string
}

// ArchiveCandidate 表示待归档文件。
type ArchiveCandidate struct {
	SourcePath string    `json:"source_path"`
	SizeBytes  int64     `json:"size_bytes"`
	ModTime    time.Time `json:"mod_time"`
	AgeSec     int64     `json:"age_sec"`
}

// ArchivePlan 是归档计划。
type ArchivePlan struct {
	Path           string             `json:"path"`
	ArchivePath    string             `json:"archive_path"`
	Compress       string             `json:"compress"`
	RetentionSec   int64              `json:"retention_sec"`
	OlderThanSec   int64              `json:"older_than_sec"`
	ScannedFiles   int                `json:"scanned_files"`
	CandidateCount int                `json:"candidate_count"`
	CandidateBytes int64              `json:"candidate_bytes"`
	Candidates     []ArchiveCandidate `json:"candidates"`
	Warnings       []string           `json:"warnings,omitempty"`
}

// ArchiveResult 是归档执行结果。
type ArchiveResult struct {
	Applied       bool     `json:"applied"`
	ArchivedCount int      `json:"archived_count"`
	ArchivedBytes int64    `json:"archived_bytes"`
	RemovedCount  int      `json:"removed_count"`
	Failed        []string `json:"failed,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

// EmptyPlan 是空目录清理计划。
type EmptyPlan struct {
	Path           string   `json:"path"`
	ScannedDirs    int      `json:"scanned_dirs"`
	EmptyDirs      []string `json:"empty_dirs"`
	CandidateCount int      `json:"candidate_count"`
	Warnings       []string `json:"warnings,omitempty"`
}

// EmptyResult 是空目录清理结果。
type EmptyResult struct {
	Applied     bool     `json:"applied"`
	DeletedDirs int      `json:"deleted_dirs"`
	Failed      []string `json:"failed,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

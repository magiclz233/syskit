// Package scanner 提供扫描目录树所需的数据结构和核心扫描逻辑。
// 这个包放在 internal 下，表示它只供当前项目内部使用，不对外暴露为公共库。
package scanner

import (
	"io"
	"os"
	"time"
)

// FileInfo 描述一个文件在最终结果中需要展示的信息。
// 这里只保留“路径、大小、修改时间”三项，因为这三项已经足够支撑终端输出和 CSV/JSON 导出。
type FileInfo struct {
	// Path 是文件的完整路径或相对扫描根目录的路径表示。
	Path string `json:"path"`
	// Size 是文件的原始字节数，排序时直接用它做比较。
	Size int64 `json:"size"`
	// ModTime 是文件最后修改时间，导出 CSV 时会一起带出。
	ModTime time.Time `json:"mod_time"`
}

// DirInfo 描述一个目录的累计大小信息。
// 这里的“目录大小”不是目录条目自身占用多少，而是它整棵子树下所有文件大小之和。
type DirInfo struct {
	// Path 是目录路径。
	Path string `json:"path"`
	// TotalSize 是该目录下所有后代文件的累计总字节数。
	TotalSize int64 `json:"total_size"`
	// FileCount 和 DirCount 当前还没有参与结果计算，先保留字段以便未来扩展。
	FileCount int `json:"file_count"`
	DirCount  int `json:"dir_count"`
}

// ScanOptions 表示一次扫描任务的配置。
// 当前项目已经收敛成单一准确扫描模式，因此这里不再保留旧的“深度限制”“近似扫描”等参数。
type ScanOptions struct {
	// RootPath 是扫描起点，也就是整棵目录树的根路径。
	RootPath string
	// TopN 表示最终最多保留多少条目录和文件结果。
	TopN int
	// MinSizeBytes 只保留大于等于该字节数的文件和目录结果；0 表示不过滤结果。
	MinSizeBytes int64
	// MaxDepth 限制扫描深度；0 表示不限制。
	MaxDepth int
	// IncludeFiles 控制是否生成“最大文件”结果集。
	IncludeFiles bool
	// IncludeDirs 控制是否生成“最大子目录”结果集。
	IncludeDirs bool
	// ShowProgress 控制扫描过程中是否打印进度。
	ShowProgress bool
	// ShowBanner 控制扫描过程中是否打印扫描模式和路径信息。
	ShowBanner bool
	// LogOutput 控制扫描日志输出位置；为空时默认输出到 stdout。
	LogOutput io.Writer
	// ExcludeDirs 是要跳过的目录名列表，只按目录名匹配，不按完整路径匹配。
	ExcludeDirs []string
}

// ScanResult 表示一次扫描的完整输出结果。
// 这是 scanner 包和 main 包之间最重要的数据交换结构。
type ScanResult struct {
	// TopFiles 是按文件大小降序排列后的结果集。
	TopFiles []FileInfo `json:"top_files"`
	// TopDirs 是按目录累计大小降序排列后的结果集。
	TopDirs []DirInfo `json:"top_dirs"`
	// TotalSize 是扫描根目录下所有文件的总大小。
	TotalSize int64 `json:"total_size"`
	// TotalFiles 是实际处理到的文件数量。
	TotalFiles int `json:"total_files"`
	// TotalDirs 是实际处理到的目录数量，不包含扫描根目录本身。
	TotalDirs int `json:"total_dirs"`
	// ScanDuration 是本次扫描总耗时。
	ScanDuration time.Duration `json:"-"`
	// ScanDurationMs 是本次扫描总耗时，供结构化输出使用。
	ScanDurationMs int64 `json:"scan_duration_ms"`
	// ProcessedPath 是最终参与扫描的根路径。
	ProcessedPath string `json:"processed_path"`
}

// NewScanOptions 返回一组适合大多数场景的默认选项。
// 调用方通常先拿到这组默认值，再按命令行参数覆盖其中一部分字段。
func NewScanOptions(rootPath string) *ScanOptions {
	return &ScanOptions{
		RootPath:     rootPath,
		TopN:         10,
		MinSizeBytes: 0,
		MaxDepth:     0,
		IncludeFiles: true,
		IncludeDirs:  true,
		ShowProgress: true,
		ShowBanner:   true,
		LogOutput:    os.Stdout,
		ExcludeDirs:  []string{},
	}
}

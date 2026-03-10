// Package scanner 提供文件系统扫描功能
// 这是一个内部包（internal），只能被本项目导入，不能被外部项目使用
package scanner

import "time"

// FileInfo 文件信息结构体
// 存储单个文件的基本信息
type FileInfo struct {
	Path    string    // 文件完整路径（使用 filepath 包处理，跨平台兼容）
	Size    int64     // 文件大小（字节）
	ModTime time.Time // 最后修改时间
}

// DirInfo 目录信息结构体
// 存储目录的累计大小信息
type DirInfo struct {
	Path      string // 目录完整路径
	TotalSize int64  // 目录总大小（包含所有子文件和子目录）
	FileCount int    // 目录下的文件数量
	DirCount  int    // 目录下的子目录数量
}

// ScanOptions 扫描选项
// 配置扫描行为的所有参数
type ScanOptions struct {
	RootPath     string   // 根目录路径（要扫描的起始目录）
	TopN         int      // 保留 Top N 结果（默认 10）
	MinSize      int64    // 最小文件大小阈值（字节），小于此值的文件会被过滤
	MaxDepth     int      // 最大扫描深度（0 表示不限制）
	IncludeFiles bool     // 是否包含文件结果
	IncludeDirs  bool     // 是否包含目录结果
	ShowProgress bool     // 是否显示进度
	ExcludeDirs  []string // 要排除的目录名列表（如 node_modules, .git）
	IncludeExt   []string // 包含的文件扩展名（如 .log, .tmp）
	ExcludeExt   []string // 排除的文件扩展名
}

// ScanResult 扫描结果
// 包含扫描完成后的所有统计信息
type ScanResult struct {
	TopFiles      []FileInfo    // Top N 最大文件列表
	TopDirs       []DirInfo     // Top N 最大目录列表
	TotalSize     int64         // 扫描到的总大小
	TotalFiles    int           // 扫描到的总文件数
	TotalDirs     int           // 扫描到的总目录数
	ScanDuration  time.Duration // 扫描耗时
	ProcessedPath string        // 实际扫描的路径（可能经过清理和规范化）
}

// NewScanOptions 创建默认的扫描选项
// 这是一个工厂函数，返回带有合理默认值的 ScanOptions
func NewScanOptions(rootPath string) *ScanOptions {
	return &ScanOptions{
		RootPath:     rootPath,
		TopN:         10,           // 默认 Top 10
		MinSize:      0,            // 默认不过滤
		MaxDepth:     0,            // 默认不限制深度
		IncludeFiles: true,         // 默认包含文件
		IncludeDirs:  true,         // 默认包含目录
		ShowProgress: true,         // 默认显示进度
		ExcludeDirs:  []string{},   // 默认不排除任何目录
		IncludeExt:   []string{},   // 默认包含所有扩展名
		ExcludeExt:   []string{},   // 默认不排除任何扩展名
	}
}

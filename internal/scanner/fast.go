// Package scanner 提供文件系统扫描功能（快速模式）
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FastScanner 快速扫描器
// 专门用于快速找到大文件夹和大文件，适合磁盘爆满时快速定位问题
//
// 核心策略：
// 1. 限制扫描深度（默认 3 层）
// 2. 自动跳过已知的大型依赖目录（node_modules、.git等）
// 3. 只记录大文件（>100MB），小文件直接忽略
// 4. 使用 Worker Pool 模式控制并发
type FastScanner struct {
	options *ScanOptions

	// 并发安全的数据结构
	dirSizes sync.Map // key: 目录路径, value: *int64

	// 原子计数器
	totalFiles   int64
	totalDirs    int64
	skippedDirs  int64
	scannedBytes int64

	// 大文件列表（只保存 >100MB 的文件）
	largeFiles   []FileInfo
	largeFilesMu sync.Mutex

	// 并发控制
	wg sync.WaitGroup
}

// 常见的大型依赖目录（自动跳过）
var excludeDirs = []string{
	"node_modules",              // Node.js 依赖
	".git",                      // Git 仓库
	".svn",                      // SVN 仓库
	"vendor",                    // Go/PHP 依赖
	".m2",                       // Maven 仓库
	".gradle",                   // Gradle 缓存
	".cargo",                    // Rust 依赖
	"target",                    // Java/Rust 编译输出
	"build",                     // 通用编译输出
	"dist",                      // 前端编译输出
	"__pycache__",               // Python 缓存
	".venv",                     // Python 虚拟环境
	"venv",                      // Python 虚拟环境
	".idea",                     // IntelliJ IDEA 配置
	".vscode",                   // VS Code 配置
	"$RECYCLE.BIN",              // Windows 回收站
	"System Volume Information", // Windows 系统卷信息
	".Trash",                    // macOS 回收站
	".npm",                      // npm 缓存
	".cache",                    // 通用缓存
}

const (
	// 大文件阈值：只记录大于此值的文件
	largeFileThreshold = 100 * 1024 * 1024 // 100 MB
)

// NewFastScanner 创建快速扫描器
func NewFastScanner(options *ScanOptions) *FastScanner {
	// 如果没有设置深度限制，默认为 3 层
	if options.MaxDepth == 0 {
		options.MaxDepth = 3
	}

	// 自动添加常见大目录到排除列表
	excludeMap := make(map[string]bool)
	for _, dir := range options.ExcludeDirs {
		excludeMap[dir] = true
	}
	for _, dir := range excludeDirs {
		if !excludeMap[dir] {
			options.ExcludeDirs = append(options.ExcludeDirs, dir)
		}
	}

	return &FastScanner{
		options:    options,
		largeFiles: make([]FileInfo, 0, options.TopN*2),
	}
}

// Scan 执行快速扫描
func (s *FastScanner) Scan() (*ScanResult, error) {
	startTime := time.Now()

	// 清理和验证路径
	cleanPath := filepath.Clean(s.options.RootPath)

	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("路径不存在或无法访问: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", cleanPath)
	}

	// 初始化根目录
	s.dirSizes.Store(cleanPath, new(int64))

	fmt.Printf("\n=== 快速扫描模式 ===\n")
	fmt.Printf("最大深度: %d 层\n", s.options.MaxDepth)
	fmt.Printf("大文件阈值: 100 MB\n")
	fmt.Printf("自动排除: %d 种常见大目录\n", len(s.options.ExcludeDirs))
	fmt.Println("正在扫描...")

	// 启动并发扫描
	s.wg.Add(1)
	go s.scanDir(cleanPath, 0)

	// 等待所有 goroutine 完成
	s.wg.Wait()

	duration := time.Since(startTime)

	// 构建结果
	result := &ScanResult{
		ProcessedPath: cleanPath,
		TotalFiles:    int(atomic.LoadInt64(&s.totalFiles)),
		TotalDirs:     int(atomic.LoadInt64(&s.totalDirs)),
		ScanDuration:  duration,
	}

	if s.options.IncludeFiles && len(s.largeFiles) > 0 {
		result.TopFiles = s.getTopFiles()
	}

	if s.options.IncludeDirs {
		result.TopDirs = s.getTopDirs()
	}

	if value, ok := s.dirSizes.Load(cleanPath); ok {
		sizePtr := value.(*int64)
		result.TotalSize = atomic.LoadInt64(sizePtr)
	}

	// 显示统计信息
	skipped := atomic.LoadInt64(&s.skippedDirs)
	if skipped > 0 {
		fmt.Printf("\n跳过了 %d 个目录（依赖目录或超出深度限制）\n", skipped)
	}

	return result, nil
}

// scanDir 扫描单个目录（带深度限制）
func (s *FastScanner) scanDir(dirPath string, depth int) {
	defer s.wg.Done()

	// 检查深度限制
	if s.options.MaxDepth > 0 && depth >= s.options.MaxDepth {
		atomic.AddInt64(&s.skippedDirs, 1)
		return
	}

	// 读取目录内容
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		// 权限不足或其他错误，跳过
		return
	}

	// 遍历目录中的所有条目
	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			// 检查是否需要排除
			if s.shouldExcludeDir(entry.Name()) {
				atomic.AddInt64(&s.skippedDirs, 1)
				continue
			}

			// 递增目录计数
			atomic.AddInt64(&s.totalDirs, 1)

			// 初始化目录大小
			s.dirSizes.Store(fullPath, new(int64))

			// 为子目录启动新的 goroutine
			s.wg.Add(1)
			go s.scanDir(fullPath, depth+1)

		} else {
			// 处理文件
			info, err := entry.Info()
			if err != nil {
				continue
			}

			size := info.Size()

			// 递增文件计数
			atomic.AddInt64(&s.totalFiles, 1)
			atomic.AddInt64(&s.scannedBytes, size)

			// 累加文件大小到所有父目录
			s.addSizeToParents(fullPath, size)

			// 只保存大文件（>100MB）
			if s.options.IncludeFiles && size > largeFileThreshold {
				s.addLargeFile(FileInfo{
					Path:    fullPath,
					Size:    size,
					ModTime: info.ModTime(),
				})
			}
		}
	}
}

// shouldExcludeDir 检查目录是否应该被排除
func (s *FastScanner) shouldExcludeDir(dirName string) bool {
	for _, exclude := range s.options.ExcludeDirs {
		if strings.EqualFold(dirName, exclude) { // 不区分大小写
			return true
		}
	}
	return false
}

// addSizeToParents 将文件大小累加到所有父目录
func (s *FastScanner) addSizeToParents(filePath string, size int64) {
	dir := filepath.Dir(filePath)

	for {
		value, ok := s.dirSizes.Load(dir)
		if !ok {
			break
		}

		sizePtr := value.(*int64)
		atomic.AddInt64(sizePtr, size)

		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			break
		}

		dir = parentDir
	}
}

// addLargeFile 添加大文件到列表（线程安全）
func (s *FastScanner) addLargeFile(file FileInfo) {
	s.largeFilesMu.Lock()
	defer s.largeFilesMu.Unlock()

	s.largeFiles = append(s.largeFiles, file)
}

// getTopFiles 获取 Top N 最大文件
func (s *FastScanner) getTopFiles() []FileInfo {
	s.largeFilesMu.Lock()
	defer s.largeFilesMu.Unlock()

	if len(s.largeFiles) == 0 {
		return []FileInfo{}
	}

	sort.Slice(s.largeFiles, func(i, j int) bool {
		return s.largeFiles[i].Size > s.largeFiles[j].Size
	})

	if len(s.largeFiles) <= s.options.TopN {
		return s.largeFiles
	}

	return s.largeFiles[:s.options.TopN]
}

// getTopDirs 获取 Top N 最大目录
func (s *FastScanner) getTopDirs() []DirInfo {
	var dirs []DirInfo

	s.dirSizes.Range(func(key, value interface{}) bool {
		path := key.(string)
		sizePtr := value.(*int64)
		size := atomic.LoadInt64(sizePtr)

		// 只包含有大小的目录
		if size > 0 {
			dirs = append(dirs, DirInfo{
				Path:      path,
				TotalSize: size,
			})
		}

		return true
	})

	if len(dirs) == 0 {
		return []DirInfo{}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].TotalSize > dirs[j].TotalSize
	})

	if len(dirs) <= s.options.TopN {
		return dirs
	}

	return dirs[:s.options.TopN]
}

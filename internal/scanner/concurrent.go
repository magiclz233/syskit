// Package scanner 提供文件系统扫描功能（并发版本）
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrentScanner 并发文件系统扫描器
// 使用 goroutine 并发扫描，大幅提升性能
//
// Go 语言知识点：
// 1. 每个子目录启动独立的 goroutine 并发扫描
// 2. 使用 WaitGroup 等待所有 goroutine 完成
// 3. 使用 channel 作为信号量限制并发数
type ConcurrentScanner struct {
	options *ScanOptions

	// 并发安全的数据结构
	dirSizes sync.Map // key: 目录路径, value: *int64

	// 原子计数器
	totalFiles int64
	totalDirs  int64

	// 文件列表
	files   []FileInfo
	filesMu sync.Mutex

	// 并发控制
	// Go 语言知识点：WaitGroup 用于等待一组 goroutine 完成
	wg sync.WaitGroup

	// 信号量：限制并发 goroutine 数量
	// Go 语言知识点：使用 buffered channel 作为信号量
	semaphore chan struct{}
}

// NewConcurrentScanner 创建并发扫描器
func NewConcurrentScanner(options *ScanOptions) *ConcurrentScanner {
	// 设置并发数：CPU 核心数 * 4（经验值，可以充分利用 I/O 等待时间）
	// Go 语言知识点：runtime.NumCPU() 获取 CPU 核心数
	concurrency := 100 // 固定 100 个并发，适合大多数场景

	return &ConcurrentScanner{
		options:   options,
		files:     make([]FileInfo, 0, options.TopN*3),
		semaphore: make(chan struct{}, concurrency), // 创建容量为 concurrency 的 channel
	}
}

// Scan 执行并发扫描
func (s *ConcurrentScanner) Scan() (*ScanResult, error) {
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

	// 初始化根目录大小
	s.dirSizes.Store(cleanPath, new(int64))

	// 启动并发扫描
	// Go 语言知识点：启动第一个 goroutine 扫描根目录
	s.wg.Add(1)
	go s.scanDirConcurrent(cleanPath)

	// 等待所有 goroutine 完成
	// Go 语言知识点：Wait() 会阻塞，直到 WaitGroup 计数器归零
	s.wg.Wait()

	duration := time.Since(startTime)

	// 构建结果
	result := &ScanResult{
		ProcessedPath: cleanPath,
		TotalFiles:    int(atomic.LoadInt64(&s.totalFiles)),
		TotalDirs:     int(atomic.LoadInt64(&s.totalDirs)),
		ScanDuration:  duration,
	}

	if s.options.IncludeFiles && len(s.files) > 0 {
		result.TopFiles = s.getTopFiles()
	}

	if s.options.IncludeDirs {
		result.TopDirs = s.getTopDirs()
	}

	if value, ok := s.dirSizes.Load(cleanPath); ok {
		sizePtr := value.(*int64)
		result.TotalSize = atomic.LoadInt64(sizePtr)
	}

	return result, nil
}

// scanDirConcurrent 并发扫描单个目录
// 这是并发扫描的核心函数
//
// Go 语言知识点：
// 1. 每个目录在独立的 goroutine 中执行
// 2. 使用 defer 确保 WaitGroup 计数器正确递减
func (s *ConcurrentScanner) scanDirConcurrent(dirPath string) {
	// 确保函数返回时递减 WaitGroup 计数器
	// Go 语言知识点：defer 语句在函数返回前执行
	defer s.wg.Done()

	// 读取目录内容
	// Go 语言知识点：os.ReadDir 比 filepath.Walk 更快
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		// 权限不足或其他错误，跳过
		return
	}

	// 遍历目录中的所有条目
	for _, entry := range entries {
		// 构建完整路径
		// Go 语言知识点：filepath.Join 自动处理路径分隔符
		fullPath := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			// 检查是否需要排除
			if s.shouldExcludeDir(entry.Name()) {
				continue
			}

			// 递增目录计数
			atomic.AddInt64(&s.totalDirs, 1)

			// 初始化目录大小
			s.dirSizes.Store(fullPath, new(int64))

			// 为子目录启动新的 goroutine（不使用信号量，让 Go runtime 自动调度）
			// Go 语言知识点：go 关键字启动新的 goroutine
			s.wg.Add(1)
			go s.scanDirConcurrent(fullPath)

		} else {
			// 处理文件
			info, err := entry.Info()
			if err != nil {
				continue
			}

			size := info.Size()

			// 递增文件计数
			atomic.AddInt64(&s.totalFiles, 1)

			// 累加文件大小到所有父目录
			s.addSizeToParents(fullPath, size)

			// 保存文件信息
			if s.options.IncludeFiles {
				s.addFile(FileInfo{
					Path:    fullPath,
					Size:    size,
					ModTime: info.ModTime(),
				})
			}
		}
	}
}

// shouldExcludeDir 检查目录是否应该被排除
func (s *ConcurrentScanner) shouldExcludeDir(dirName string) bool {
	for _, exclude := range s.options.ExcludeDirs {
		if dirName == exclude {
			return true
		}
	}
	return false
}

// addSizeToParents 将文件大小累加到所有父目录
func (s *ConcurrentScanner) addSizeToParents(filePath string, size int64) {
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

// addFile 添加文件到列表（线程安全）
func (s *ConcurrentScanner) addFile(file FileInfo) {
	s.filesMu.Lock()
	defer s.filesMu.Unlock()

	s.files = append(s.files, file)
}

// getTopFiles 获取 Top N 最大文件
func (s *ConcurrentScanner) getTopFiles() []FileInfo {
	s.filesMu.Lock()
	defer s.filesMu.Unlock()

	if len(s.files) == 0 {
		return []FileInfo{}
	}

	sort.Slice(s.files, func(i, j int) bool {
		return s.files[i].Size > s.files[j].Size
	})

	if len(s.files) <= s.options.TopN {
		return s.files
	}

	return s.files[:s.options.TopN]
}

// getTopDirs 获取 Top N 最大目录
func (s *ConcurrentScanner) getTopDirs() []DirInfo {
	var dirs []DirInfo

	s.dirSizes.Range(func(key, value interface{}) bool {
		path := key.(string)
		sizePtr := value.(*int64)
		size := atomic.LoadInt64(sizePtr)

		dirs = append(dirs, DirInfo{
			Path:      path,
			TotalSize: size,
		})

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

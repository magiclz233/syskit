// Package scanner 提供文件系统扫描功能
package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Scanner 文件系统扫描器
// 这是核心扫描器结构体，负责遍历文件系统并收集统计信息
//
// Go 语言知识点：
// 1. 结构体字段首字母大写表示导出（public），小写表示私有（private）
// 2. sync.Map 是并发安全的 map，用于多个 goroutine 同时访问
type Scanner struct {
	options *ScanOptions // 扫描选项配置

	// 并发安全的数据结构
	// Go 语言知识点：sync.Map 是无锁的并发安全 map
	dirSizes sync.Map // 存储每个目录的累计大小，key: 目录路径, value: *int64

	// 原子计数器（用于并发统计）
	// Go 语言知识点：atomic 包提供原子操作，无需加锁
	totalFiles int64 // 总文件数（使用 atomic 操作）
	totalDirs  int64 // 总目录数（使用 atomic 操作）

	// 文件列表（用于保存 Top N）
	// Go 语言知识点：[]FileInfo 是切片（动态数组）
	files   []FileInfo   // 所有扫描到的文件
	filesMu sync.Mutex   // 保护 files 切片的互斥锁
}

// NewScanner 创建新的扫描器实例
// 这是一个构造函数（Go 的惯用模式）
//
// Go 语言知识点：
// 1. 函数名以 New 开头表示构造函数
// 2. 返回指针类型 *Scanner
func NewScanner(options *ScanOptions) *Scanner {
	return &Scanner{
		options: options,
		files:   make([]FileInfo, 0, options.TopN*3), // 预分配容量，避免频繁扩容
	}
}

// Scan 执行扫描
// 这是扫描器的主要方法，返回扫描结果和可能的错误
//
// Go 语言知识点：
// 1. (s *Scanner) 是方法接收者，表示这是 Scanner 的方法
// 2. 返回 (*ScanResult, error) 是 Go 的惯用模式
func (s *Scanner) Scan() (*ScanResult, error) {
	// 记录开始时间
	startTime := time.Now()

	// 清理和验证路径
	// Go 语言知识点：filepath.Clean 会清理路径，移除多余的分隔符
	cleanPath := filepath.Clean(s.options.RootPath)

	// 检查路径是否存在
	// Go 语言知识点：os.Stat 返回文件信息和错误
	info, err := os.Stat(cleanPath)
	if err != nil {
		// Go 语言知识点：fmt.Errorf 用于创建格式化的错误
		return nil, fmt.Errorf("路径不存在或无法访问: %w", err)
	}

	// 确保是目录
	if !info.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", cleanPath)
	}

	// 初始化根目录大小为 0
	// Go 语言知识点：new(int64) 创建一个 int64 指针，初始值为 0
	s.dirSizes.Store(cleanPath, new(int64))

	// 开始遍历文件系统
	// Go 语言知识点：filepath.WalkDir 是 Go 1.16+ 的新 API，比 Walk 更快
	err = filepath.WalkDir(cleanPath, s.walkFunc)
	if err != nil {
		return nil, fmt.Errorf("扫描过程中发生错误: %w", err)
	}

	// 计算扫描耗时
	duration := time.Since(startTime)

	// 构建并返回结果
	result := &ScanResult{
		ProcessedPath: cleanPath,
		TotalFiles:    int(atomic.LoadInt64(&s.totalFiles)),
		TotalDirs:     int(atomic.LoadInt64(&s.totalDirs)),
		ScanDuration:  duration,
	}

	// 处理文件结果：排序并获取 Top N
	if s.options.IncludeFiles && len(s.files) > 0 {
		result.TopFiles = s.getTopFiles()
	}

	// 处理目录结果：排序并获取 Top N
	if s.options.IncludeDirs {
		result.TopDirs = s.getTopDirs()
	}

	// 计算总大小（根目录的大小就是总大小）
	if value, ok := s.dirSizes.Load(cleanPath); ok {
		sizePtr := value.(*int64)
		result.TotalSize = atomic.LoadInt64(sizePtr)
	}

	return result, nil
}

// walkFunc 是 filepath.WalkDir 的回调函数
// 每遍历到一个文件或目录时都会调用此函数
//
// Go 语言知识点：
// 1. fs.DirEntry 是文件系统条目接口（Go 1.16+）
// 2. 返回 error 可以控制遍历行为（返回 nil 继续，返回错误停止）
func (s *Scanner) walkFunc(path string, d fs.DirEntry, err error) error {
	// 处理遍历过程中的错误
	if err != nil {
		// Go 语言知识点：os.IsPermission 检查是否是权限错误
		if os.IsPermission(err) {
			// 权限不足，跳过并继续
			fmt.Printf("警告: 权限不足，跳过: %s\n", path)
			return nil // 返回 nil 表示继续遍历
		}
		// 其他错误也跳过
		return nil
	}

	// 检查是否需要排除此目录
	if d.IsDir() && s.shouldExcludeDir(d.Name()) {
		// Go 语言知识点：filepath.SkipDir 是特殊错误，表示跳过此目录
		return filepath.SkipDir
	}

	// 获取文件信息
	// Go 语言知识点：d.Info() 返回 fs.FileInfo 接口
	info, err := d.Info()
	if err != nil {
		// 无法获取文件信息，跳过
		return nil
	}

	// 处理目录
	if d.IsDir() {
		// 原子递增目录计数
		// Go 语言知识点：atomic.AddInt64 是原子操作，线程安全
		atomic.AddInt64(&s.totalDirs, 1)

		// 初始化目录大小为 0
		s.dirSizes.Store(path, new(int64))
		return nil
	}

	// 处理文件
	size := info.Size()

	// 原子递增文件计数
	atomic.AddInt64(&s.totalFiles, 1)

	// 累加文件大小到所有父目录
	s.addSizeToParents(path, size)

	// 如果需要包含文件结果，保存文件信息
	if s.options.IncludeFiles {
		s.addFile(FileInfo{
			Path:    path,
			Size:    size,
			ModTime: info.ModTime(),
		})
	}

	return nil
}

// shouldExcludeDir 检查目录是否应该被排除
func (s *Scanner) shouldExcludeDir(dirName string) bool {
	// 遍历排除列表
	for _, exclude := range s.options.ExcludeDirs {
		if dirName == exclude {
			return true
		}
	}
	return false
}

// addSizeToParents 将文件大小累加到所有父目录
// 这是实现目录大小统计的关键函数
//
// Go 语言知识点：
// 1. filepath.Dir 获取父目录路径
// 2. 使用循环向上遍历所有父目录
func (s *Scanner) addSizeToParents(filePath string, size int64) {
	// 获取文件所在目录
	dir := filepath.Dir(filePath)

	// 向上遍历所有父目录，累加大小
	for {
		// 尝试加载目录的大小指针
		// Go 语言知识点：sync.Map.Load 返回 (value, ok)
		value, ok := s.dirSizes.Load(dir)
		if !ok {
			// 目录不存在（不应该发生），跳出循环
			break
		}

		// 类型断言：将 interface{} 转换为 *int64
		// Go 语言知识点：value.(*int64) 是类型断言
		sizePtr := value.(*int64)

		// 原子地增加目录大小
		// Go 语言知识点：atomic.AddInt64 保证并发安全
		atomic.AddInt64(sizePtr, size)

		// 获取父目录
		parentDir := filepath.Dir(dir)

		// 如果已经到达根目录，停止
		// Go 语言知识点：filepath.Dir 到达根目录后会返回自身
		if parentDir == dir {
			break
		}

		dir = parentDir
	}
}

// addFile 添加文件到列表（线程安全）
func (s *Scanner) addFile(file FileInfo) {
	// 加锁保护切片操作
	// Go 语言知识点：defer 确保函数返回前解锁
	s.filesMu.Lock()
	defer s.filesMu.Unlock()

	// 添加文件到列表
	s.files = append(s.files, file)

	// TODO: 后续会实现内存优化，只保留 Top N * 3 的文件
}

// getTopFiles 获取 Top N 最大文件
// 使用排序算法对文件按大小排序，返回前 N 个
//
// Go 语言知识点：
// 1. sort.Slice 可以对任意切片排序
// 2. 使用匿名函数定义排序规则
func (s *Scanner) getTopFiles() []FileInfo {
	s.filesMu.Lock()
	defer s.filesMu.Unlock()

	// 如果没有文件，返回空切片
	if len(s.files) == 0 {
		return []FileInfo{}
	}

	// 按文件大小降序排序
	// Go 语言知识点：sort.Slice 接受切片和比较函数
	sort.Slice(s.files, func(i, j int) bool {
		// 返回 true 表示 i 应该排在 j 前面
		// 我们要降序排序（大的在前），所以比较 Size 大于
		return s.files[i].Size > s.files[j].Size
	})

	// 如果文件数量少于 TopN，直接返回所有文件
	if len(s.files) <= s.options.TopN {
		return s.files
	}

	// 返回前 TopN 个文件
	// Go 语言知识点：切片操作 [start:end]
	return s.files[:s.options.TopN]
}

// getTopDirs 获取 Top N 最大目录
// 遍历 sync.Map，收集所有目录并排序
//
// Go 语言知识点：
// 1. sync.Map.Range 遍历所有键值对
// 2. 使用匿名函数作为回调
func (s *Scanner) getTopDirs() []DirInfo {
	// 收集所有目录信息
	var dirs []DirInfo

	// 遍历 sync.Map
	// Go 语言知识点：Range 接受一个函数，返回 false 可以提前终止遍历
	s.dirSizes.Range(func(key, value interface{}) bool {
		path := key.(string)
		sizePtr := value.(*int64)
		size := atomic.LoadInt64(sizePtr)

		// 创建目录信息
		dirs = append(dirs, DirInfo{
			Path:      path,
			TotalSize: size,
			// TODO: 后续添加 FileCount 和 DirCount
		})

		return true // 继续遍历
	})

	// 如果没有目录，返回空切片
	if len(dirs) == 0 {
		return []DirInfo{}
	}

	// 按目录大小降序排序
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].TotalSize > dirs[j].TotalSize
	})

	// 如果目录数量少于 TopN，直接返回所有目录
	if len(dirs) <= s.options.TopN {
		return dirs
	}

	// 返回前 TopN 个目录
	return dirs[:s.options.TopN]
}

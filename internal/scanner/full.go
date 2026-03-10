// Package scanner 提供文件系统扫描功能（完整扫描 - Worker Pool 模式）
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// FullScanner 完整扫描器（Worker Pool 模式）
// 使用固定数量的 worker goroutine，避免 goroutine 爆炸
// 适合全盘扫描，性能最优
//
// Go 语言知识点：
// 1. Worker Pool 模式：固定数量的 worker + 任务队列
// 2. 使用 channel 传递待扫描的目录
// 3. 避免为每个目录创建 goroutine（减少调度开销）
type FullScanner struct {
	options *ScanOptions

	// 并发安全的数据结构
	dirSizes sync.Map // key: 目录路径, value: *int64

	// 原子计数器
	totalFiles   int64
	totalDirs    int64
	scannedBytes int64
	activeTasks  int64 // 活跃任务计数

	// 文件列表
	files   []FileInfo
	filesMu sync.Mutex

	// Worker Pool
	// Go 语言知识点：使用 channel 作为任务队列
	taskQueue chan *scanTask // 待扫描目录队列
	workerWg  sync.WaitGroup // 等待所有 worker 完成

	// 进度显示
	lastProgress time.Time
	progressMu   sync.Mutex
}

// scanTask 扫描任务
type scanTask struct {
	path string // 目录路径
}

// NewFullScanner 创建完整扫描器
func NewFullScanner(options *ScanOptions) *FullScanner {
	// Worker 数量：CPU 核心数 * 8（I/O 密集型任务）
	// Go 语言知识点：runtime.NumCPU() 获取 CPU 核心数
	workerCount := runtime.NumCPU() * 8
	if workerCount > 100 {
		workerCount = 100 // 最多 100 个 worker
	}

	return &FullScanner{
		options:   options,
		files:     make([]FileInfo, 0, options.TopN*3),
		taskQueue: make(chan *scanTask, workerCount*2), // 缓冲队列
	}
}

// Scan 执行完整扫描
func (s *FullScanner) Scan() (*ScanResult, error) {
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

	// 计算 worker 数量
	workerCount := cap(s.taskQueue) / 2
	if workerCount < 1 {
		workerCount = 1
	}

	fmt.Printf("\n=== 完整扫描模式 ===\n")
	fmt.Printf("Worker 数量: %d\n", workerCount)
	fmt.Printf("扫描深度: 无限制\n")
	fmt.Printf("正在扫描: %s\n", cleanPath)
	fmt.Println()

	// 启动 worker pool
	// Go 语言知识点：启动固定数量的 worker goroutine
	s.workerWg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go s.worker()
	}

	// 提交第一个任务（根目录）
	atomic.AddInt64(&s.activeTasks, 1)
	s.taskQueue <- &scanTask{path: cleanPath}

	// 等待所有任务完成
	for {
		time.Sleep(10 * time.Millisecond)
		if atomic.LoadInt64(&s.activeTasks) == 0 {
			break
		}
	}

	// 关闭任务队列，通知 worker 退出
	close(s.taskQueue)

	// 等待所有 worker 退出
	s.workerWg.Wait()

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

// worker 工作协程
// Go 语言知识点：
// 1. 从 channel 中不断读取任务
// 2. channel 关闭后，for range 会自动退出
func (s *FullScanner) worker() {
	defer s.workerWg.Done()

	// 不断从任务队列中取任务
	// Go 语言知识点：for range channel 会阻塞等待，直到 channel 关闭
	for task := range s.taskQueue {
		s.scanDir(task.path)
		atomic.AddInt64(&s.activeTasks, -1)
	}
}

// scanDir 扫描单个目录
func (s *FullScanner) scanDir(dirPath string) {
	// 读取目录内容
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		// 权限不足或其他错误，跳过
		return
	}

	// 显示进度（每秒最多更新一次）
	s.showProgress()

	// 遍历目录中的所有条目
	for _, entry := range entries {
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

			// 提交新任务到队列（非阻塞）
			atomic.AddInt64(&s.activeTasks, 1)
			select {
			case s.taskQueue <- &scanTask{path: fullPath}:
				// 成功提交
			default:
				// 队列满了，使用 goroutine 异步提交
				go func(path string) {
					s.taskQueue <- &scanTask{path: path}
				}(fullPath)
			}

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

// showProgress 显示扫描进度
func (s *FullScanner) showProgress() {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()

	now := time.Now()
	if now.Sub(s.lastProgress) < time.Second {
		return // 1秒内不重复显示
	}
	s.lastProgress = now

	files := atomic.LoadInt64(&s.totalFiles)
	dirs := atomic.LoadInt64(&s.totalDirs)
	bytes := atomic.LoadInt64(&s.scannedBytes)

	// 计算速度
	fmt.Printf("\r进度: %d 个文件, %d 个目录, %.2f GB",
		files, dirs, float64(bytes)/(1024*1024*1024))
}

// shouldExcludeDir 检查目录是否应该被排除
func (s *FullScanner) shouldExcludeDir(dirName string) bool {
	for _, exclude := range s.options.ExcludeDirs {
		if dirName == exclude {
			return true
		}
	}
	return false
}

// addSizeToParents 将文件大小累加到所有父目录
func (s *FullScanner) addSizeToParents(filePath string, size int64) {
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
func (s *FullScanner) addFile(file FileInfo) {
	s.filesMu.Lock()
	defer s.filesMu.Unlock()

	s.files = append(s.files, file)
}

// getTopFiles 获取 Top N 最大文件
func (s *FullScanner) getTopFiles() []FileInfo {
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
func (s *FullScanner) getTopDirs() []DirInfo {
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

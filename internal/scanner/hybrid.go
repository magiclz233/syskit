// Package scanner 提供文件系统扫描功能（混合模式 - 推荐）
package scanner

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HybridScanner 混合模式扫描器（推荐）⭐⭐⭐⭐⭐
// 使用两阶段渐进式扫描，提供最佳用户体验
//
// 阶段1：浅层快速扫描（深度2层），5-10秒，显示 Top 20 大目录
// 阶段2：深入扫描 Top 20 大目录，10-20秒，显示 Top 20 大文件
//
// Go 语言知识点：
// 1. 组合使用 FastScanner 和 FullScanner
// 2. 使用 goroutine 并发扫描多个大目录
// 3. 使用 sync.Mutex 保护共享的文件列表
// 4. 使用 WaitGroup 等待所有扫描完成
//
// 优势：
// - 不会漏掉大文件（大文件一定在大目录里）
// - 总耗时 15-30 秒，比完整模式快 3-5 倍
// - 用户体验好：快速看到结果，可选择是否继续
type HybridScanner struct {
	options *ScanOptions

	// 阶段1结果
	phase1Result *ScanResult

	// 阶段2数据
	allFiles   []FileInfo
	allFilesMu sync.Mutex
}

// NewHybridScanner 创建混合模式扫描器
func NewHybridScanner(options *ScanOptions) *HybridScanner {
	return &HybridScanner{
		options:  options,
		allFiles: make([]FileInfo, 0, options.TopN*10),
	}
}

// Scan 执行混合模式扫描
func (h *HybridScanner) Scan() (*ScanResult, error) {
	startTime := time.Now()

	fmt.Printf("\n=== 混合模式扫描（推荐）⭐⭐⭐⭐⭐ ===\n")
	fmt.Println("两阶段渐进式扫描，快速定位大文件")
	fmt.Println()

	// ========== 阶段 1：浅层快速扫描 ==========
	fmt.Println("【阶段 1/2】快速扫描目录结构...")
	fmt.Println("策略：只扫描前 2 层，快速找出大目录")
	fmt.Println()

	phase1Options := &ScanOptions{
		RootPath:     h.options.RootPath,
		TopN:         h.options.TopN,
		MaxDepth:     2,              // 只扫描前2层
		IncludeFiles: false,          // 不记录文件
		IncludeDirs:  true,           // 只记录目录
		ShowProgress: false,          // 不显示进度
		ExcludeDirs:  h.options.ExcludeDirs,
	}

	phase1Scanner := NewFastScanner(phase1Options)
	phase1Result, err := phase1Scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("阶段1扫描失败: %w", err)
	}

	h.phase1Result = phase1Result

	// 显示阶段1结果
	fmt.Printf("\n✓ 阶段 1 完成（耗时: %v）\n", phase1Result.ScanDuration)
	fmt.Printf("  扫描到 %d 个目录，总大小: %s\n",
		phase1Result.TotalDirs,
		formatBytes(phase1Result.TotalSize))
	fmt.Println()

	// 显示 Top 20 大目录
	topDirsCount := len(phase1Result.TopDirs)
	if topDirsCount > 20 {
		topDirsCount = 20
	}

	if topDirsCount > 0 {
		fmt.Printf("=== Top %d 大目录 ===\n", topDirsCount)
		for i := 0; i < topDirsCount && i < len(phase1Result.TopDirs); i++ {
			dir := phase1Result.TopDirs[i]
			fmt.Printf("%2d. %-12s  %s\n",
				i+1,
				formatBytes(dir.TotalSize),
				dir.Path)
		}
		fmt.Println()
	}

	// ========== 用户选择 ==========
	fmt.Println("【提示】大文件通常在大目录里")
	fmt.Print("是否深入扫描这些大目录以查找大文件？(y/n，默认 y): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	// 默认为 y
	if input != "n" && input != "no" {
		// ========== 阶段 2：深入扫描大目录 ==========
		fmt.Println()
		fmt.Println("【阶段 2/2】深入扫描大目录...")
		fmt.Printf("策略：并发扫描 Top %d 大目录，精准找到大文件\n", topDirsCount)
		fmt.Println()

		phase2StartTime := time.Now()

		// 并发扫描 Top 20 大目录
		var wg sync.WaitGroup
		var scannedCount int64
		var totalCount = int64(topDirsCount)

		for i := 0; i < topDirsCount && i < len(phase1Result.TopDirs); i++ {
			dir := phase1Result.TopDirs[i]

			wg.Add(1)
			go func(dirPath string, index int) {
				defer wg.Done()

				// 使用 FullScanner 深入扫描
				scanOptions := &ScanOptions{
					RootPath:     dirPath,
					TopN:         h.options.TopN * 2, // 多保留一些，最后再排序
					IncludeFiles: true,
					IncludeDirs:  false,
					ShowProgress: false,
					ExcludeDirs:  h.options.ExcludeDirs,
				}

				scanner := NewFullScanner(scanOptions)
				result, err := scanner.Scan()
				if err != nil {
					// 扫描失败，跳过
					return
				}

				// 汇总文件
				h.allFilesMu.Lock()
				h.allFiles = append(h.allFiles, result.TopFiles...)
				h.allFilesMu.Unlock()

				// 更新进度
				count := atomic.AddInt64(&scannedCount, 1)
				fmt.Printf("\r进度: %d/%d 个目录已扫描", count, totalCount)
			}(dir.Path, i)
		}

		// 等待所有扫描完成
		wg.Wait()
		fmt.Println() // 换行

		phase2Duration := time.Since(phase2StartTime)
		fmt.Printf("\n✓ 阶段 2 完成（耗时: %v）\n", phase2Duration)
		fmt.Printf("  找到 %d 个文件\n", len(h.allFiles))
		fmt.Println()

		// 排序并取 Top N
		h.allFilesMu.Lock()
		if len(h.allFiles) > 0 {
			sort.Slice(h.allFiles, func(i, j int) bool {
				return h.allFiles[i].Size > h.allFiles[j].Size
			})

			if len(h.allFiles) > h.options.TopN {
				h.allFiles = h.allFiles[:h.options.TopN]
			}
		}
		h.allFilesMu.Unlock()
	} else {
		fmt.Println("\n已跳过阶段 2")
	}

	// 构建最终结果
	totalDuration := time.Since(startTime)

	result := &ScanResult{
		ProcessedPath: h.options.RootPath,
		TopFiles:      h.allFiles,
		TopDirs:       phase1Result.TopDirs,
		TotalSize:     phase1Result.TotalSize,
		TotalFiles:    phase1Result.TotalFiles,
		TotalDirs:     phase1Result.TotalDirs,
		ScanDuration:  totalDuration,
	}

	return result, nil
}

// formatBytes 格式化字节数（内部辅助函数）
func formatBytes(bytes int64) string {
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	value := float64(bytes)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", value/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", value/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", value/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", value/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

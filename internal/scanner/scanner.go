package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Scanner 是当前项目唯一保留的扫描器实现。
// 它执行的是“全量准确扫描”：
// 1. 遍历整个目录树；
// 2. 统计每个目录的累计大小；
// 3. 维护最大的子目录和最大的文件结果。
//
// 这里不再区分“快速模式”“混合模式”等分支，目的是保证输出结果始终完整可解释。
type Scanner struct {
	// options 保存本次扫描的配置项。
	options *ScanOptions

	// rootPath 是规范化后的扫描根目录路径。
	// 很多边界判断都需要以它为基准，例如：
	// - 是否跳过根目录本身的 Top 子目录展示；
	// - 向父目录累加大小时何时停止。
	rootPath string

	// dirSizes 记录每个目录的累计大小。
	// key 是目录路径，value 是目录下所有后代文件的总字节数。
	dirSizes map[string]int64
	// files 保存“最大文件”的候选集。
	// 为了避免超大目录树下把所有文件都永久留在内存中，后续会定期裁剪。
	files []FileInfo

	// 下面这些字段是扫描过程中的实时统计值。
	totalFiles   int
	totalDirs    int
	scannedBytes int64
	lastProgress time.Time
}

// NewScanner 创建一个新的准确扫描器实例。
// 这里会顺手初始化目录大小表和文件候选切片，避免后续频繁做零值判断。
func NewScanner(options *ScanOptions) *Scanner {
	return &Scanner{
		options:  options,
		dirSizes: make(map[string]int64),
		files:    make([]FileInfo, 0, options.TopN*3),
	}
}

// Scan 执行一次完整扫描，并返回最终结果。
// 这是扫描器的对外主入口。
func (s *Scanner) Scan() (*ScanResult, error) {
	startTime := time.Now()

	// 先把用户输入的路径清理成规范形式，再做存在性和目录类型校验。
	cleanPath := filepath.Clean(s.options.RootPath)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("路径不存在或无法访问: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", cleanPath)
	}

	s.rootPath = cleanPath
	// 根目录本身也要参与累计大小计算，所以先放一条初始记录。
	s.dirSizes[cleanPath] = 0

	fmt.Printf("\n=== 准确扫描模式 ===\n")
	fmt.Println("策略: 全量扫描整个目录树，结果完整")
	fmt.Printf("正在扫描: %s\n", cleanPath)
	fmt.Println()

	// filepath.WalkDir 会从根目录开始深度遍历整个目录树。
	// 每碰到一个目录或文件，都会回调到 walkFunc。
	err = filepath.WalkDir(cleanPath, s.walkFunc)
	if err != nil {
		return nil, fmt.Errorf("扫描过程中发生错误: %w", err)
	}

	// 如果启用了进度显示，扫描结束后补一个换行，
	// 否则最后一条 \r 覆盖式输出会和后面的结果挤在同一行。
	if s.options.ShowProgress {
		fmt.Println()
	}

	result := &ScanResult{
		ProcessedPath: cleanPath,
		TotalSize:     s.dirSizes[cleanPath],
		TotalFiles:    s.totalFiles,
		TotalDirs:     s.totalDirs,
		ScanDuration:  time.Since(startTime),
	}

	// 目录和文件结果可以分别关闭。
	// 这样用户可以只看目录，或者只看文件。
	if s.options.IncludeFiles {
		result.TopFiles = s.getTopFiles()
	}
	if s.options.IncludeDirs {
		result.TopDirs = s.getTopDirs()
	}

	return result, nil
}

// walkFunc 是 filepath.WalkDir 的回调函数。
// 它负责处理三类情况：
// 1. 遍历错误；
// 2. 目录条目；
// 3. 文件条目。
func (s *Scanner) walkFunc(path string, d fs.DirEntry, walkErr error) error {
	// WalkDir 有时会把访问错误通过 walkErr 传进来。
	// 这里选择“跳过并继续”，避免单个坏路径导致整个扫描失败。
	if walkErr != nil {
		return nil
	}

	// 符号链接和 reparse point 容易把扫描带回上层路径，造成循环或重复统计。
	// 因此这里统一跳过符号链接。
	if d.Type()&os.ModeSymlink != 0 {
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}

	if d.IsDir() {
		// 根目录本身不参与排除逻辑，也不计入“子目录数量”。
		if path != s.rootPath && s.shouldExcludeDir(d.Name()) {
			return filepath.SkipDir
		}
		if path != s.rootPath {
			s.totalDirs++
			// 目录一旦被遍历到，先在累计大小表里占一个位置，
			// 后面它下面的文件再通过 addSizeToParents 慢慢把大小加进来。
			s.dirSizes[path] = 0
			s.showProgress()
		}
		return nil
	}

	// 文件分支需要拿到完整 FileInfo，因为 DirEntry 里不直接带文件大小和修改时间。
	info, err := d.Info()
	if err != nil {
		return nil
	}

	size := info.Size()
	s.totalFiles++
	s.scannedBytes += size

	// 每发现一个文件，就把它的大小一路累加到当前目录、父目录、祖先目录，直到根目录。
	// 这一步是“目录累计大小统计”成立的核心。
	s.addSizeToParents(path, size)
	s.showProgress()

	if s.options.IncludeFiles {
		s.addFile(FileInfo{
			Path:    path,
			Size:    size,
			ModTime: info.ModTime(),
		})
	}

	return nil
}

// shouldExcludeDir 判断一个目录名是否在排除列表里。
// 使用 EqualFold 做大小写无关比较，这样在 Windows 上更符合直觉。
func (s *Scanner) shouldExcludeDir(dirName string) bool {
	for _, exclude := range s.options.ExcludeDirs {
		if strings.EqualFold(dirName, exclude) {
			return true
		}
	}
	return false
}

// addSizeToParents 把一个文件的大小加到它的所有父目录上。
// 例如：
//
//	文件: D:\a\b\c.txt
//
// 那么会依次累加到：
//
//	D:\a\b
//	D:\a
//	D:\
//
// 这样最后某个目录的累计大小，就是它整棵子树下所有文件之和。
func (s *Scanner) addSizeToParents(filePath string, size int64) {
	dir := filepath.Dir(filePath)
	for {
		s.dirSizes[dir] += size

		// 根目录已经加过之后就停止，不再往更高层路径外溢。
		if dir == s.rootPath {
			return
		}

		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			return
		}
		dir = parentDir
	}
}

// addFile 把文件加入“最大文件候选集”。
// 这里不会无限保留所有文件，而是定期裁剪，只留下更有希望进入最终 Top N 的那一批。
// 这样可以在文件非常多的时候显著降低内存占用。
func (s *Scanner) addFile(file FileInfo) {
	s.files = append(s.files, file)

	// limit 是触发裁剪的阈值，keep 是裁剪后保留的候选数。
	// 这里故意保留比 TopN 更多的文件，避免过早裁掉边缘候选。
	limit := s.options.TopN * 6
	if limit < 100 {
		limit = 100
	}
	keep := s.options.TopN * 3
	if keep < 50 {
		keep = 50
	}

	if len(s.files) > limit {
		sort.Slice(s.files, func(i, j int) bool {
			return s.files[i].Size > s.files[j].Size
		})
		// 这里重新分配一段切片，避免旧底层数组长期被引用，便于后续 GC 回收。
		s.files = append([]FileInfo(nil), s.files[:keep]...)
	}
}

// getTopFiles 返回最终的 Top N 最大文件列表。
// 真正输出前会再完整排序一次，确保顺序绝对正确。
func (s *Scanner) getTopFiles() []FileInfo {
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

// getTopDirs 返回最终的 Top N 最大子目录列表。
// 注意这里会显式跳过根目录本身，因为用户要看的通常是“这个目录下面哪个子目录最大”。
func (s *Scanner) getTopDirs() []DirInfo {
	dirs := make([]DirInfo, 0, len(s.dirSizes))
	for path, size := range s.dirSizes {
		if path == s.rootPath || size <= 0 {
			continue
		}

		dirs = append(dirs, DirInfo{
			Path:      path,
			TotalSize: size,
		})
	}

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

// showProgress 按固定时间间隔输出扫描进度。
// 这里故意不在每个文件上都打印，否则 I/O 会反过来拖慢扫描本身。
func (s *Scanner) showProgress() {
	if !s.options.ShowProgress {
		return
	}

	now := time.Now()
	if now.Sub(s.lastProgress) < time.Second {
		return
	}
	s.lastProgress = now

	fmt.Printf("\r进度: %d 个文件, %d 个目录, %.2f GB",
		s.totalFiles,
		s.totalDirs,
		float64(s.scannedBytes)/(1024*1024*1024))
}

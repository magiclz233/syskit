// Package main 是程序的入口点
// Go 程序必须有一个 main 包和 main 函数
package main

import (
	"encoding/csv"
	"encoding/json"
	"find-large-files/internal/scanner"
	"find-large-files/pkg/utils"
	"flag"
	"fmt"
	"os"
	"strings"
)

const version = "0.3.0"

// 命令行参数
var (
	mode         string
	topN         int
	maxDepth     int
	minSizeStr   string
	excludeDirs  string
	includeFiles bool
	includeDirs  bool
	format       string
	exportCSV    string
	showVersion  bool
	showHelp     bool
)

func init() {
	// 定义命令行参数
	flag.StringVar(&mode, "mode", "hybrid", "扫描模式: hybrid(混合), fast(快速), full(完整)")
	flag.StringVar(&mode, "m", "hybrid", "扫描模式（简写）")
	flag.IntVar(&topN, "top", 20, "显示 Top N 结果")
	flag.IntVar(&topN, "t", 20, "显示 Top N 结果（简写）")
	flag.IntVar(&maxDepth, "max-depth", 3, "最大扫描深度（仅快速模式，0=无限制）")
	flag.StringVar(&minSizeStr, "min-size", "100MB", "最小文件大小阈值（仅快速模式，如: 50MB, 1GB）")
	flag.StringVar(&excludeDirs, "exclude", "", "排除的目录（逗号分隔，如: node_modules,.git）")
	flag.BoolVar(&includeFiles, "include-files", true, "包含文件结果")
	flag.BoolVar(&includeDirs, "include-dirs", true, "包含目录结果")
	flag.StringVar(&format, "format", "table", "输出格式: table, json, csv")
	flag.StringVar(&exportCSV, "export-csv", "", "CSV 导出路径前缀")
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&showVersion, "v", false, "显示版本信息（简写）")
	flag.BoolVar(&showHelp, "help", false, "显示帮助信息")
	flag.BoolVar(&showHelp, "h", false, "显示帮助信息（简写）")

	// 自定义 Usage
	flag.Usage = printUsage
}

func main() {
	// 解析命令行参数
	flag.Parse()

	// 显示版本信息
	if showVersion {
		fmt.Printf("find-large-files version %s\n", version)
		fmt.Println("跨平台文件系统分析工具")
		os.Exit(0)
	}

	// 显示帮助信息
	if showHelp {
		printUsage()
		os.Exit(0)
	}

	// 获取扫描路径
	var scanPath string
	if flag.NArg() > 0 {
		scanPath = flag.Arg(0)
	} else {
		// 获取当前目录作为默认值
		currentDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("错误: 无法获取当前目录: %v\n", err)
			os.Exit(1)
		}

		// 提示用户输入路径
		fmt.Printf("请输入要扫描的目录路径（直接回车使用当前目录: %s）: ", currentDir)
		var input string
		fmt.Scanln(&input)

		// 如果用户直接回车，使用当前目录
		input = strings.TrimSpace(input)
		if input == "" {
			scanPath = currentDir
		} else {
			scanPath = input
		}
	}

	// 打印欢迎信息
	fmt.Println("=== 文件/文件夹大小分析工具 ===")
	fmt.Printf("版本: %s\n", version)
	fmt.Println("支持平台: Windows, Linux, macOS")
	fmt.Println()

	// 创建扫描选项
	options := scanner.NewScanOptions(scanPath)
	options.TopN = topN
	options.MaxDepth = maxDepth
	options.IncludeFiles = includeFiles
	options.IncludeDirs = includeDirs

	// 解析最小文件大小
	if minSizeStr != "" {
		minSize, err := utils.ParseSize(minSizeStr)
		if err != nil {
			fmt.Printf("错误: 无效的最小文件大小: %v\n", err)
			os.Exit(1)
		}
		options.MinSize = minSize
	}

	// 解析排除目录
	if excludeDirs != "" {
		options.ExcludeDirs = strings.Split(excludeDirs, ",")
		for i := range options.ExcludeDirs {
			options.ExcludeDirs[i] = strings.TrimSpace(options.ExcludeDirs[i])
		}
	}

	// 根据模式创建扫描器
	var result *scanner.ScanResult
	var err error

	switch strings.ToLower(mode) {
	case "hybrid":
		s := scanner.NewHybridScanner(options)
		result, err = s.Scan()
	case "fast":
		s := scanner.NewFastScanner(options)
		result, err = s.Scan()
	case "full":
		s := scanner.NewFullScanner(options)
		result, err = s.Scan()
	default:
		fmt.Printf("错误: 未知的扫描模式: %s（支持: hybrid, fast, full）\n", mode)
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("错误: 扫描失败: %v\n", err)
		os.Exit(1)
	}

	// 根据格式输出结果
	switch strings.ToLower(format) {
	case "json":
		outputJSON(result)
	case "csv":
		outputCSV(result, exportCSV)
	default:
		outputTable(result)
	}

	// 如果指定了 CSV 导出路径
	if exportCSV != "" && format != "csv" {
		outputCSV(result, exportCSV)
	}

	fmt.Println("\n扫描完成！")
}

// printUsage 打印使用帮助
func printUsage() {
	fmt.Println("用法: find-large-files [选项] [路径]")
	fmt.Println()
	fmt.Println("参数:")
	fmt.Println("  [路径]              要扫描的目录路径（默认: 当前目录）")
	fmt.Println()
	fmt.Println("选项:")
	fmt.Println("  -m, --mode          扫描模式:")
	fmt.Println("                        hybrid - 混合模式（默认，快速找大目录+深入扫描）")
	fmt.Println("                        fast   - 快速模式（跳过依赖目录，只记录大文件>100MB）")
	fmt.Println("                        full   - 完整模式（扫描所有文件，记录所有大小）")
	fmt.Println("  -t, --top           显示 Top N 结果（默认: 20）")
	fmt.Println("  --max-depth         最大扫描深度（可选，0=无限制）")
	fmt.Println("  --min-size          最小文件大小阈值（仅快速模式，默认: 100MB）")
	fmt.Println("  --exclude           排除的目录（逗号分隔，如: node_modules,.git）")
	fmt.Println("  --include-files     包含文件结果（默认: true）")
	fmt.Println("  --include-dirs      包含目录结果（默认: true）")
	fmt.Println("  --format            输出格式: table(默认), json, csv")
	fmt.Println("  --export-csv        CSV 导出路径前缀")
	fmt.Println("  -v, --version       显示版本信息")
	fmt.Println("  -h, --help          显示帮助信息")
	fmt.Println()
	fmt.Println("模式说明:")
	fmt.Println("  hybrid - 推荐用于大多数场景，快速找到大目录后深入扫描")
	fmt.Println("  fast   - 跳过依赖目录（node_modules等），只记录大文件，速度快")
	fmt.Println("  full   - 扫描所有文件并记录，统计最完整但速度较慢")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  # 快速扫描整个 D 盘（推荐，跳过依赖目录）")
	fmt.Println("  find-large-files --mode fast D:\\")
	fmt.Println()
	fmt.Println("  # 完整扫描整个 D 盘（最准确，但较慢）")
	fmt.Println("  find-large-files --mode full D:\\")
	fmt.Println()
	fmt.Println("  # 混合模式（默认，快速找大文件）")
	fmt.Println("  find-large-files D:\\")
	fmt.Println()
	fmt.Println("  # 快速模式 - 自定义配置")
	fmt.Println("  find-large-files --mode fast --max-depth 5 --min-size 50MB D:\\")
	fmt.Println()
	fmt.Println("  # 完整模式 - 排除常见大目录")
	fmt.Println("  find-large-files --mode full --exclude node_modules,.git D:\\")
	fmt.Println()
	fmt.Println("  # JSON 输出")
	fmt.Println("  find-large-files --format json D:\\")
	fmt.Println()
	fmt.Println("  # CSV 导出")
	fmt.Println("  find-large-files --export-csv D:\\result D:\\")
}

// outputTable 表格输出
func outputTable(result *scanner.ScanResult) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("扫描统计")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("  扫描路径: %s\n", result.ProcessedPath)
	fmt.Printf("  处理文件数: %s\n", utils.FormatNumber(result.TotalFiles))
	fmt.Printf("  处理目录数: %s\n", utils.FormatNumber(result.TotalDirs))
	fmt.Printf("  总大小: %s\n", utils.FormatBytes(result.TotalSize))
	fmt.Printf("  总耗时: %v\n", result.ScanDuration)
	fmt.Println()

	// 显示 Top N 目录
	if len(result.TopDirs) > 0 {
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("Top %d 目录（按累计大小排序）\n", len(result.TopDirs))
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("%-4s %-12s %s\n", "序号", "大小", "路径")
		fmt.Println(strings.Repeat("-", 80))
		for i, dir := range result.TopDirs {
			fmt.Printf("%-4d %-12s %s\n", i+1, utils.FormatBytes(dir.TotalSize), dir.Path)
		}
		fmt.Println()
	}

	// 显示 Top N 文件
	if len(result.TopFiles) > 0 {
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("Top %d 文件（按单文件大小排序）\n", len(result.TopFiles))
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("%-4s %-12s %s\n", "序号", "大小", "路径")
		fmt.Println(strings.Repeat("-", 80))
		for i, file := range result.TopFiles {
			fmt.Printf("%-4d %-12s %s\n", i+1, utils.FormatBytes(file.Size), file.Path)
		}
		fmt.Println()
	}
}

// outputJSON JSON 输出
func outputJSON(result *scanner.ScanResult) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("错误: JSON 序列化失败: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// outputCSV CSV 输出
func outputCSV(result *scanner.ScanResult, prefix string) {
	if prefix == "" {
		prefix = "result"
	}

	// 导出目录结果
	if len(result.TopDirs) > 0 {
		dirsFile := prefix + "_dirs.csv"
		f, err := os.Create(dirsFile)
		if err != nil {
			fmt.Printf("错误: 创建目录 CSV 文件失败: %v\n", err)
		} else {
			defer f.Close()
			writer := csv.NewWriter(f)
			defer writer.Flush()

			// 写入表头
			writer.Write([]string{"序号", "大小(字节)", "大小(格式化)", "路径"})

			// 写入数据
			for i, dir := range result.TopDirs {
				writer.Write([]string{
					fmt.Sprintf("%d", i+1),
					fmt.Sprintf("%d", dir.TotalSize),
					utils.FormatBytes(dir.TotalSize),
					dir.Path,
				})
			}

			fmt.Printf("✓ 目录结果已导出到: %s\n", dirsFile)
		}
	}

	// 导出文件结果
	if len(result.TopFiles) > 0 {
		filesFile := prefix + "_files.csv"
		f, err := os.Create(filesFile)
		if err != nil {
			fmt.Printf("错误: 创建文件 CSV 文件失败: %v\n", err)
		} else {
			defer f.Close()
			writer := csv.NewWriter(f)
			defer writer.Flush()

			// 写入表头
			writer.Write([]string{"序号", "大小(字节)", "大小(格式化)", "路径", "修改时间"})

			// 写入数据
			for i, file := range result.TopFiles {
				writer.Write([]string{
					fmt.Sprintf("%d", i+1),
					fmt.Sprintf("%d", file.Size),
					utils.FormatBytes(file.Size),
					file.Path,
					file.ModTime.Format("2006-01-02 15:04:05"),
				})
			}

			fmt.Printf("✓ 文件结果已导出到: %s\n", filesFile)
		}
	}
}

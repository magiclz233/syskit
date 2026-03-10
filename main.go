// Package main 是命令行程序的入口包。
// 这个文件负责两类事情：
// 1. 解析用户输入的命令行参数；
// 2. 调用扫描器并把结果格式化输出到终端或文件。
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

// version 是程序当前版本号。
// 发布脚本会直接修改这里的值，因此这里保持为一个简单常量，便于自动替换。
const version = "0.4.0"

// 这些变量对应命令行参数。
// 使用包级变量配合 flag 包，是 Go 命令行工具里最常见、最直接的写法。
var (
	// topN 控制最终展示多少条“最大文件”和“最大子目录”结果。
	topN int
	// excludeDirs 是逗号分隔的目录名列表，扫描时会整目录跳过。
	excludeDirs string
	// includeFiles 控制是否输出最大文件列表。
	includeFiles bool
	// includeDirs 控制是否输出最大子目录列表。
	includeDirs bool
	// format 控制最终输出格式，支持 table / json / csv。
	format string
	// exportCSV 是 CSV 导出文件名前缀，例如 report 会导出 report_dirs.csv 和 report_files.csv。
	exportCSV string
	// showVersion 为 true 时只打印版本并退出。
	showVersion bool
	// showHelp 为 true 时只打印帮助并退出。
	showHelp bool
)

// init 在 main 之前执行。
// 这里集中注册所有命令行参数，并把默认帮助输出替换为自定义版本。
func init() {
	flag.IntVar(&topN, "top", 20, "显示 Top N 结果")
	flag.IntVar(&topN, "t", 20, "显示 Top N 结果（简写）")
	flag.StringVar(&excludeDirs, "exclude", "", "排除的目录（逗号分隔，如: node_modules,.git）")
	flag.BoolVar(&includeFiles, "include-files", true, "包含文件结果")
	flag.BoolVar(&includeDirs, "include-dirs", true, "包含目录结果")
	flag.StringVar(&format, "format", "table", "输出格式: table, json, csv")
	flag.StringVar(&exportCSV, "export-csv", "", "CSV 导出路径前缀")
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&showVersion, "v", false, "显示版本信息（简写）")
	flag.BoolVar(&showHelp, "help", false, "显示帮助信息")
	flag.BoolVar(&showHelp, "h", false, "显示帮助信息（简写）")

	// 替换默认帮助文本，确保帮助内容和项目的真实行为一致。
	flag.Usage = printUsage
}

// main 是程序主流程。
// 整个流程很直白：
// 1. 解析参数；
// 2. 确定扫描路径；
// 3. 构造扫描选项；
// 4. 执行准确扫描；
// 5. 根据用户要求输出结果。
func main() {
	flag.Parse()

	if showVersion {
		fmt.Printf("find-large-files version %s\n", version)
		fmt.Println("跨平台文件系统分析工具")
		os.Exit(0)
	}

	if showHelp {
		printUsage()
		os.Exit(0)
	}

	// scanPath 是本次扫描的根路径。
	// 如果用户在命令行里已经给了位置参数，就直接使用；
	// 否则交互式询问，回车默认使用当前工作目录。
	var scanPath string
	if flag.NArg() > 0 {
		scanPath = flag.Arg(0)
	} else {
		currentDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("错误: 无法获取当前目录: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("请输入要扫描的目录路径（直接回车使用当前目录: %s）: ", currentDir)
		var input string
		fmt.Scanln(&input)

		input = strings.TrimSpace(input)
		if input == "" {
			scanPath = currentDir
		} else {
			scanPath = input
		}
	}

	fmt.Println("=== 文件/文件夹大小分析工具 ===")
	fmt.Printf("版本: %s\n", version)
	fmt.Println("支持平台: Windows, Linux, macOS")
	fmt.Println()

	// NewScanOptions 会先给出一组默认值，再由 CLI 参数覆盖需要自定义的部分。
	options := scanner.NewScanOptions(scanPath)
	options.TopN = topN
	options.IncludeFiles = includeFiles
	options.IncludeDirs = includeDirs

	// --exclude 传进来的是一个字符串，这里把它拆成切片。
	// 同时做 TrimSpace，避免用户写成 "node_modules, .git" 时把空格也带进去。
	if excludeDirs != "" {
		options.ExcludeDirs = strings.Split(excludeDirs, ",")
		for i := range options.ExcludeDirs {
			options.ExcludeDirs[i] = strings.TrimSpace(options.ExcludeDirs[i])
		}
	}

	// 当前项目只保留一套“全量准确扫描”实现。
	// 这样可以保证输出的最大文件和最大子目录是完整结果，而不是近似估算。
	s := scanner.NewScanner(options)
	result, err := s.Scan()
	if err != nil {
		fmt.Printf("错误: 扫描失败: %v\n", err)
		os.Exit(1)
	}

	// 根据 --format 选择最终输出方式。
	// table 适合终端直接查看，json/csv 适合后续处理或导入其它工具。
	switch strings.ToLower(format) {
	case "json":
		outputJSON(result)
	case "csv":
		outputCSV(result, exportCSV)
	default:
		outputTable(result)
	}

	// 如果用户显式指定了 --export-csv，同时当前主输出格式又不是 csv，
	// 那么除了终端输出之外，再额外导出一份 CSV 文件。
	if exportCSV != "" && format != "csv" {
		outputCSV(result, exportCSV)
	}

	fmt.Println("\n扫描完成！")
}

// printUsage 输出帮助文本。
// 这里不依赖 flag 默认生成的帮助，是因为我们希望把“输出语义”和“常用示例”一起写清楚。
func printUsage() {
	fmt.Println("用法: find-large-files [选项] [路径]")
	fmt.Println()
	fmt.Println("参数:")
	fmt.Println("  [路径]              要扫描的目录路径（默认: 当前目录）")
	fmt.Println()
	fmt.Println("选项:")
	fmt.Println("  -t, --top           显示 Top N 结果（默认: 20）")
	fmt.Println("  --exclude           排除的目录（逗号分隔，如: node_modules,.git）")
	fmt.Println("  --include-files     包含文件结果（默认: true）")
	fmt.Println("  --include-dirs      包含目录结果（默认: true）")
	fmt.Println("  --format            输出格式: table(默认), json, csv")
	fmt.Println("  --export-csv        CSV 导出路径前缀")
	fmt.Println("  -v, --version       显示版本信息")
	fmt.Println("  -h, --help          显示帮助信息")
	fmt.Println()
	fmt.Println("说明:")
	fmt.Println("  程序始终执行全量准确扫描，返回目录树中最大的子目录和最大的文件。")
	fmt.Println("  结果中的目录列表不包含根目录本身，只显示其下扫描到的子目录。")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  # 扫描整个 D 盘")
	fmt.Println("  find-large-files D:\\")
	fmt.Println()
	fmt.Println("  # 排除常见依赖目录")
	fmt.Println("  find-large-files --exclude node_modules,.git,vendor D:\\")
	fmt.Println()
	fmt.Println("  # JSON 输出")
	fmt.Println("  find-large-files --format json D:\\")
	fmt.Println()
	fmt.Println("  # CSV 导出")
	fmt.Println("  find-large-files --export-csv D:\\result D:\\")
}

// outputTable 负责人类可读的终端表格输出。
// 这是默认输出方式，重点是让用户直接看清：
// 1. 扫描规模；
// 2. 最大的子目录；
// 3. 最大的文件。
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

	if len(result.TopDirs) > 0 {
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("Top %d 子目录（按累计大小排序）\n", len(result.TopDirs))
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("%-4s %-12s %s\n", "序号", "大小", "路径")
		fmt.Println(strings.Repeat("-", 80))
		for i, dir := range result.TopDirs {
			fmt.Printf("%-4d %-12s %s\n", i+1, utils.FormatBytes(dir.TotalSize), dir.Path)
		}
		fmt.Println()
	}

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

// outputJSON 把扫描结果序列化成缩进后的 JSON。
// 这种格式适合喂给脚本、其它程序，或者直接保存成报告文件。
func outputJSON(result *scanner.ScanResult) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("错误: JSON 序列化失败: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// outputCSV 把目录和文件结果分别导出成两个 CSV 文件。
// 这么做比塞进一个文件更清晰，因为两类数据字段并不完全一致。
func outputCSV(result *scanner.ScanResult, prefix string) {
	if prefix == "" {
		prefix = "result"
	}

	if len(result.TopDirs) > 0 {
		dirsFile := prefix + "_dirs.csv"
		f, err := os.Create(dirsFile)
		if err != nil {
			fmt.Printf("错误: 创建目录 CSV 文件失败: %v\n", err)
		} else {
			defer f.Close()
			writer := csv.NewWriter(f)
			defer writer.Flush()

			// 目录结果导出“原始字节数 + 格式化大小 + 路径”，
			// 这样既方便程序处理，也方便人直接打开阅读。
			writer.Write([]string{"序号", "大小(字节)", "大小(格式化)", "子目录路径"})

			for i, dir := range result.TopDirs {
				writer.Write([]string{
					fmt.Sprintf("%d", i+1),
					fmt.Sprintf("%d", dir.TotalSize),
					utils.FormatBytes(dir.TotalSize),
					dir.Path,
				})
			}

			fmt.Printf("✓ 子目录结果已导出到: %s\n", dirsFile)
		}
	}

	if len(result.TopFiles) > 0 {
		filesFile := prefix + "_files.csv"
		f, err := os.Create(filesFile)
		if err != nil {
			fmt.Printf("错误: 创建文件 CSV 文件失败: %v\n", err)
		} else {
			defer f.Close()
			writer := csv.NewWriter(f)
			defer writer.Flush()

			writer.Write([]string{"序号", "大小(字节)", "大小(格式化)", "路径", "修改时间"})

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

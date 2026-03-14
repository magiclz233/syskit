package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syskit/internal/errs"
	"syskit/internal/scanner"
	"syskit/pkg/utils"

	"github.com/spf13/cobra"
)

func runLegacyScan(cmd *cobra.Command, args []string, version string, opts *rootFlags, global *globalOptions) error {
	consoleOut := cmd.OutOrStdout()
	resultOut := consoleOut

	if opts.showVersion {
		fmt.Fprintf(consoleOut, "syskit version %s\n", version)
		fmt.Fprintln(consoleOut, "跨平台本地系统运维 CLI 工具")
		return nil
	}

	closeOutput, err := global.configureOutputWriter(&resultOut)
	if err != nil {
		return err
	}
	if closeOutput != nil {
		defer closeOutput()
	}

	scanPath, err := resolveScanPath(cmd, args)
	if err != nil {
		return err
	}

	if !global.quiet {
		fmt.Fprintln(consoleOut, "=== 文件/文件夹大小分析工具 ===")
		fmt.Fprintf(consoleOut, "版本: %s\n", version)
		fmt.Fprintln(consoleOut, "支持平台: Windows, Linux, macOS")
		fmt.Fprintln(consoleOut)
	}

	options := scanner.NewScanOptions(scanPath)
	options.TopN = opts.topN
	options.IncludeFiles = opts.includeFiles
	options.IncludeDirs = opts.includeDirs

	if opts.excludeDirs != "" {
		options.ExcludeDirs = strings.Split(opts.excludeDirs, ",")
		for i := range options.ExcludeDirs {
			options.ExcludeDirs[i] = strings.TrimSpace(options.ExcludeDirs[i])
		}
	}

	s := scanner.NewScanner(options)
	result, err := s.Scan()
	if err != nil {
		return errs.Wrap(errs.ExitExecutionFailed, err, "扫描失败")
	}

	switch global.format {
	case "json":
		if err := outputJSON(resultOut, result); err != nil {
			return err
		}
	case "csv":
		if err := outputCSV(consoleOut, result, global.csvPrefix(opts.exportCSV)); err != nil {
			return err
		}
	case "markdown":
		outputMarkdown(resultOut, result)
	case "table":
		outputTable(resultOut, result)
	default:
		return errs.New(errs.ExitInvalidArgument, fmt.Sprintf("不支持的输出格式: %s", global.format))
	}

	if opts.exportCSV != "" && global.format != "csv" {
		if err := outputCSV(consoleOut, result, opts.exportCSV); err != nil {
			return err
		}
	}

	if global.outputPath != "" && global.format != "csv" && !global.quiet {
		fmt.Fprintf(consoleOut, "✓ 输出已写入: %s\n", global.outputPath)
	}

	if !global.quiet {
		fmt.Fprintln(consoleOut, "\n扫描完成！")
	}
	return nil
}

func resolveScanPath(cmd *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", errs.Wrap(errs.ExitExecutionFailed, err, "无法获取当前目录")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "请输入要扫描的目录路径（直接回车使用当前目录: %s）: ", currentDir)
	reader := bufio.NewReader(cmd.InOrStdin())
	input, readErr := reader.ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", errs.Wrap(errs.ExitExecutionFailed, readErr, "读取输入失败")
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return currentDir, nil
	}

	return input, nil
}

func outputTable(w io.Writer, result *scanner.ScanResult) {
	fmt.Fprintln(w, "\n"+strings.Repeat("=", 80))
	fmt.Fprintln(w, "扫描统计")
	fmt.Fprintln(w, strings.Repeat("=", 80))
	fmt.Fprintf(w, "  扫描路径: %s\n", result.ProcessedPath)
	fmt.Fprintf(w, "  处理文件数: %s\n", utils.FormatNumber(result.TotalFiles))
	fmt.Fprintf(w, "  处理目录数: %s\n", utils.FormatNumber(result.TotalDirs))
	fmt.Fprintf(w, "  总大小: %s\n", utils.FormatBytes(result.TotalSize))
	fmt.Fprintf(w, "  总耗时: %v\n", result.ScanDuration)
	fmt.Fprintln(w)

	if len(result.TopDirs) > 0 {
		fmt.Fprintln(w, strings.Repeat("=", 80))
		fmt.Fprintf(w, "Top %d 子目录（按累计大小排序）\n", len(result.TopDirs))
		fmt.Fprintln(w, strings.Repeat("=", 80))
		fmt.Fprintf(w, "%-4s %-12s %s\n", "序号", "大小", "路径")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		for i, dir := range result.TopDirs {
			fmt.Fprintf(w, "%-4d %-12s %s\n", i+1, utils.FormatBytes(dir.TotalSize), dir.Path)
		}
		fmt.Fprintln(w)
	}

	if len(result.TopFiles) > 0 {
		fmt.Fprintln(w, strings.Repeat("=", 80))
		fmt.Fprintf(w, "Top %d 文件（按单文件大小排序）\n", len(result.TopFiles))
		fmt.Fprintln(w, strings.Repeat("=", 80))
		fmt.Fprintf(w, "%-4s %-12s %s\n", "序号", "大小", "路径")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		for i, file := range result.TopFiles {
			fmt.Fprintf(w, "%-4d %-12s %s\n", i+1, utils.FormatBytes(file.Size), file.Path)
		}
		fmt.Fprintln(w)
	}
}

func outputJSON(w io.Writer, result *scanner.ScanResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errs.Wrap(errs.ExitExecutionFailed, err, "JSON 序列化失败")
	}

	fmt.Fprintln(w, string(data))
	return nil
}

func outputCSV(w io.Writer, result *scanner.ScanResult, prefix string) error {
	if prefix == "" {
		prefix = "result"
	}

	if len(result.TopDirs) > 0 {
		dirsFile := prefix + "_dirs.csv"
		if err := writeDirsCSV(dirsFile, result); err != nil {
			return err
		}
		fmt.Fprintf(w, "✓ 子目录结果已导出到: %s\n", dirsFile)
	}

	if len(result.TopFiles) > 0 {
		filesFile := prefix + "_files.csv"
		if err := writeFilesCSV(filesFile, result); err != nil {
			return err
		}
		fmt.Fprintf(w, "✓ 文件结果已导出到: %s\n", filesFile)
	}

	return nil
}

func writeDirsCSV(path string, result *scanner.ScanResult) error {
	f, err := os.Create(path)
	if err != nil {
		return errs.Wrap(errs.ExitExecutionFailed, err, "创建目录 CSV 文件失败")
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if err := writer.Write([]string{"序号", "大小(字节)", "大小(格式化)", "子目录路径"}); err != nil {
		return errs.Wrap(errs.ExitExecutionFailed, err, "写入目录 CSV 表头失败")
	}

	for i, dir := range result.TopDirs {
		record := []string{
			fmt.Sprintf("%d", i+1),
			fmt.Sprintf("%d", dir.TotalSize),
			utils.FormatBytes(dir.TotalSize),
			dir.Path,
		}
		if err := writer.Write(record); err != nil {
			return errs.Wrap(errs.ExitExecutionFailed, err, "写入目录 CSV 数据失败")
		}
	}

	return nil
}

func writeFilesCSV(path string, result *scanner.ScanResult) error {
	f, err := os.Create(path)
	if err != nil {
		return errs.Wrap(errs.ExitExecutionFailed, err, "创建文件 CSV 文件失败")
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if err := writer.Write([]string{"序号", "大小(字节)", "大小(格式化)", "路径", "修改时间"}); err != nil {
		return errs.Wrap(errs.ExitExecutionFailed, err, "写入文件 CSV 表头失败")
	}

	for i, file := range result.TopFiles {
		record := []string{
			fmt.Sprintf("%d", i+1),
			fmt.Sprintf("%d", file.Size),
			utils.FormatBytes(file.Size),
			file.Path,
			file.ModTime.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(record); err != nil {
			return errs.Wrap(errs.ExitExecutionFailed, err, "写入文件 CSV 数据失败")
		}
	}

	return nil
}

func outputMarkdown(w io.Writer, result *scanner.ScanResult) {
	fmt.Fprintln(w, "# 扫描统计")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| 项目 | 值 |")
	fmt.Fprintln(w, "|---|---|")
	fmt.Fprintf(w, "| 扫描路径 | %s |\n", result.ProcessedPath)
	fmt.Fprintf(w, "| 处理文件数 | %s |\n", utils.FormatNumber(result.TotalFiles))
	fmt.Fprintf(w, "| 处理目录数 | %s |\n", utils.FormatNumber(result.TotalDirs))
	fmt.Fprintf(w, "| 总大小 | %s |\n", utils.FormatBytes(result.TotalSize))
	fmt.Fprintf(w, "| 总耗时 | %v |\n", result.ScanDuration)
	fmt.Fprintln(w)

	if len(result.TopDirs) > 0 {
		fmt.Fprintln(w, "## Top 子目录")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| 序号 | 大小 | 路径 |")
		fmt.Fprintln(w, "|---|---|---|")
		for i, dir := range result.TopDirs {
			fmt.Fprintf(w, "| %d | %s | %s |\n", i+1, utils.FormatBytes(dir.TotalSize), filepath.ToSlash(dir.Path))
		}
		fmt.Fprintln(w)
	}

	if len(result.TopFiles) > 0 {
		fmt.Fprintln(w, "## Top 文件")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| 序号 | 大小 | 路径 |")
		fmt.Fprintln(w, "|---|---|---|")
		for i, file := range result.TopFiles {
			fmt.Fprintf(w, "| %d | %s | %s |\n", i+1, utils.FormatBytes(file.Size), filepath.ToSlash(file.Path))
		}
	}
}

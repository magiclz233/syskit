// Package output 中的 ScanPresenter 负责渲染目录扫描结果。
package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syskit/internal/scanner"
	"syskit/pkg/utils"
)

// ScanPresenter 把 scanner.ScanResult 渲染成 table/markdown/csv。
type ScanPresenter struct {
	result *scanner.ScanResult
}

// NewScanPresenter 创建扫描结果 presenter。
func NewScanPresenter(result *scanner.ScanResult) *ScanPresenter {
	return &ScanPresenter{result: result}
}

// RenderTable 以终端友好的文本表格输出扫描统计、Top 目录和 Top 文件。
func (p *ScanPresenter) RenderTable(w io.Writer) error {
	fmt.Fprintln(w, "\n"+strings.Repeat("=", 80))
	fmt.Fprintln(w, "扫描统计")
	fmt.Fprintln(w, strings.Repeat("=", 80))
	fmt.Fprintf(w, "  扫描路径: %s\n", p.result.ProcessedPath)
	fmt.Fprintf(w, "  处理文件数: %s\n", utils.FormatNumber(p.result.TotalFiles))
	fmt.Fprintf(w, "  处理目录数: %s\n", utils.FormatNumber(p.result.TotalDirs))
	fmt.Fprintf(w, "  总大小: %s\n", utils.FormatBytes(p.result.TotalSize))
	fmt.Fprintf(w, "  总耗时: %v\n", p.result.ScanDuration)
	fmt.Fprintln(w)

	if len(p.result.TopDirs) > 0 {
		fmt.Fprintln(w, strings.Repeat("=", 80))
		fmt.Fprintf(w, "Top %d 子目录（按累计大小排序）\n", len(p.result.TopDirs))
		fmt.Fprintln(w, strings.Repeat("=", 80))
		fmt.Fprintf(w, "%-4s %-12s %s\n", "序号", "大小", "路径")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		for i, dir := range p.result.TopDirs {
			fmt.Fprintf(w, "%-4d %-12s %s\n", i+1, utils.FormatBytes(dir.TotalSize), dir.Path)
		}
		fmt.Fprintln(w)
	}

	if len(p.result.TopFiles) > 0 {
		fmt.Fprintln(w, strings.Repeat("=", 80))
		fmt.Fprintf(w, "Top %d 文件（按单文件大小排序）\n", len(p.result.TopFiles))
		fmt.Fprintln(w, strings.Repeat("=", 80))
		fmt.Fprintf(w, "%-4s %-12s %s\n", "序号", "大小", "路径")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		for i, file := range p.result.TopFiles {
			fmt.Fprintf(w, "%-4d %-12s %s\n", i+1, utils.FormatBytes(file.Size), file.Path)
		}
		fmt.Fprintln(w)
	}

	return nil
}

// RenderMarkdown 以 Markdown 表格形式输出扫描结果。
func (p *ScanPresenter) RenderMarkdown(w io.Writer) error {
	fmt.Fprintln(w, "# 扫描统计")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| 项目 | 值 |")
	fmt.Fprintln(w, "|---|---|")
	fmt.Fprintf(w, "| 扫描路径 | %s |\n", filepath.ToSlash(p.result.ProcessedPath))
	fmt.Fprintf(w, "| 处理文件数 | %s |\n", utils.FormatNumber(p.result.TotalFiles))
	fmt.Fprintf(w, "| 处理目录数 | %s |\n", utils.FormatNumber(p.result.TotalDirs))
	fmt.Fprintf(w, "| 总大小 | %s |\n", utils.FormatBytes(p.result.TotalSize))
	fmt.Fprintf(w, "| 总耗时 | %v |\n", p.result.ScanDuration)
	fmt.Fprintln(w)

	if len(p.result.TopDirs) > 0 {
		fmt.Fprintln(w, "## Top 子目录")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| 序号 | 大小 | 路径 |")
		fmt.Fprintln(w, "|---|---|---|")
		for i, dir := range p.result.TopDirs {
			fmt.Fprintf(w, "| %d | %s | %s |\n", i+1, utils.FormatBytes(dir.TotalSize), filepath.ToSlash(dir.Path))
		}
		fmt.Fprintln(w)
	}

	if len(p.result.TopFiles) > 0 {
		fmt.Fprintln(w, "## Top 文件")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| 序号 | 大小 | 路径 |")
		fmt.Fprintln(w, "|---|---|---|")
		for i, file := range p.result.TopFiles {
			fmt.Fprintf(w, "| %d | %s | %s |\n", i+1, utils.FormatBytes(file.Size), filepath.ToSlash(file.Path))
		}
	}

	return nil
}

// RenderCSV 将目录和文件结果分别导出为两个 CSV 文件。
// 这样做比把两类记录强行塞进一个表更容易后续处理。
func (p *ScanPresenter) RenderCSV(w io.Writer, prefix string) error {
	if prefix == "" {
		prefix = "result"
	}

	if len(p.result.TopDirs) > 0 {
		dirsFile := prefix + "_dirs.csv"
		if err := writeDirsCSV(dirsFile, p.result); err != nil {
			return err
		}
		fmt.Fprintf(w, "✓ 子目录结果已导出到: %s\n", dirsFile)
	}

	if len(p.result.TopFiles) > 0 {
		filesFile := prefix + "_files.csv"
		if err := writeFilesCSV(filesFile, p.result); err != nil {
			return err
		}
		fmt.Fprintf(w, "✓ 文件结果已导出到: %s\n", filesFile)
	}

	return nil
}

// writeDirsCSV 写出目录结果 CSV。
func writeDirsCSV(path string, result *scanner.ScanResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if err := writer.Write([]string{"序号", "大小(字节)", "大小(格式化)", "子目录路径"}); err != nil {
		return err
	}

	for i, dir := range result.TopDirs {
		record := []string{
			fmt.Sprintf("%d", i+1),
			fmt.Sprintf("%d", dir.TotalSize),
			utils.FormatBytes(dir.TotalSize),
			dir.Path,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// writeFilesCSV 写出文件结果 CSV。
func writeFilesCSV(path string, result *scanner.ScanResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if err := writer.Write([]string{"序号", "大小(字节)", "大小(格式化)", "路径", "修改时间"}); err != nil {
		return err
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
			return err
		}
	}

	return nil
}

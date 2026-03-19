// Package cliutil 中的扫描执行器用于复用目录扫描命令的公共流程。
// 当前正式扫描入口已经收敛为 `disk scan`，这里保留的是扫描执行与输出的共享实现。
package cliutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syskit/internal/errs"
	"syskit/internal/output"
	"syskit/internal/scanner"
	"time"
)

// ScanRunOptions 描述一次扫描任务本身的业务参数。
type ScanRunOptions struct {
	// Path 是扫描根路径。
	Path string
	// Version 只在交互式 table 输出时展示给用户。
	Version string
	// Title 用于交互式横幅标题。
	Title string
	// TopN 决定最大文件和目录结果保留多少条。
	TopN int
	// IncludeFiles 控制是否输出文件结果。
	IncludeFiles bool
	// IncludeDirs 控制是否输出目录结果。
	IncludeDirs bool
	// ExcludeDirs 是按目录名匹配的排除列表。
	ExcludeDirs []string
	// ExportCSV 指定 CSV 导出的前缀。
	ExportCSV string
	// MinSizeBytes 用于过滤小于阈值的文件和目录。
	MinSizeBytes int64
	// MaxDepth 限制扫描深度；0 表示不限制。
	MaxDepth int
	// ShowBanner 控制是否输出交互式横幅。
	ShowBanner bool
}

// ScanOutputOptions 描述输出层参数。
type ScanOutputOptions struct {
	// Format 是最终输出格式。
	Format string
	// OutputPath 是结构化输出文件路径；CSV 例外，由 ExportCSV 处理。
	OutputPath string
	// Quiet 为 true 时不输出额外提示行。
	Quiet bool
}

// RunScan 负责执行完整扫描流程：
// 1. 根据输出模式决定是否显示交互式横幅；
// 2. 构造 scanner 选项并执行扫描；
// 3. 使用统一结果模型和 presenter 输出结果；
// 4. 必要时额外导出 CSV 文件。
func RunScan(consoleOut io.Writer, resultOut io.Writer, runOpts ScanRunOptions, outputOpts ScanOutputOptions) error {
	startedAt := time.Now()
	interactiveTable := outputOpts.Format == "table" && outputOpts.OutputPath == "" && !outputOpts.Quiet

	if interactiveTable && runOpts.ShowBanner {
		title := runOpts.Title
		if strings.TrimSpace(title) == "" {
			title = "文件/文件夹大小分析工具"
		}
		fmt.Fprintf(consoleOut, "=== %s ===\n", title)
		if runOpts.Version != "" {
			fmt.Fprintf(consoleOut, "版本: %s\n", runOpts.Version)
		}
		fmt.Fprintln(consoleOut, "支持平台: Windows, Linux, macOS")
		fmt.Fprintln(consoleOut)
	}

	options := scanner.NewScanOptions(runOpts.Path)
	options.TopN = runOpts.TopN
	options.IncludeFiles = runOpts.IncludeFiles
	options.IncludeDirs = runOpts.IncludeDirs
	options.ExcludeDirs = append([]string(nil), runOpts.ExcludeDirs...)
	options.MinSizeBytes = runOpts.MinSizeBytes
	options.MaxDepth = runOpts.MaxDepth
	options.ShowBanner = interactiveTable
	options.ShowProgress = interactiveTable
	options.LogOutput = io.Discard
	if interactiveTable {
		options.LogOutput = consoleOut
	}

	s := scanner.NewScanner(options)
	result, err := s.Scan()
	if err != nil {
		return errs.ExecutionFailed("扫描失败", err)
	}

	commandResult := output.NewSuccessResult("扫描完成", result, startedAt)
	presenter := output.NewScanPresenter(result)

	renderOut := resultOut
	if outputOpts.Format == "csv" {
		renderOut = consoleOut
	}

	if err := output.Render(renderOut, outputOpts.Format, commandResult, presenter, csvPrefix(outputOpts.OutputPath, runOpts.ExportCSV)); err != nil {
		return err
	}

	if runOpts.ExportCSV != "" && outputOpts.Format != "csv" {
		if err := output.Render(consoleOut, "csv", commandResult, presenter, runOpts.ExportCSV); err != nil {
			return err
		}
	}

	if outputOpts.OutputPath != "" && outputOpts.Format != "csv" && !outputOpts.Quiet {
		fmt.Fprintf(consoleOut, "✓ 输出已写入: %s\n", outputOpts.OutputPath)
	}

	if interactiveTable && runOpts.ShowBanner {
		fmt.Fprintln(consoleOut, "\n扫描完成！")
	}

	return nil
}

// ConfigureOutputWriter 在非 CSV 模式下把输出重定向到文件。
// 返回的关闭函数由调用方负责 defer 执行。
func ConfigureOutputWriter(format string, outputPath string, out *io.Writer) (func(), error) {
	if outputPath == "" || format == "csv" {
		return nil, nil
	}

	dir := filepath.Dir(outputPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, errs.ExecutionFailed("创建输出目录失败", err)
		}
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return nil, errs.ExecutionFailed("创建输出文件失败", err)
	}

	*out = file
	return func() {
		_ = file.Close()
	}, nil
}

// csvPrefix 计算 CSV 导出时的文件名前缀。
// 优先使用命令显式传入的 fallback，其次从结构化输出路径中推导。
func csvPrefix(outputPath string, fallback string) string {
	if fallback != "" {
		return fallback
	}
	if outputPath == "" {
		return ""
	}

	ext := filepath.Ext(outputPath)
	if ext == "" {
		return outputPath
	}

	return strings.TrimSuffix(outputPath, ext)
}

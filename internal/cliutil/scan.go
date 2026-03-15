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

type ScanRunOptions struct {
	Path         string
	Version      string
	Title        string
	TopN         int
	IncludeFiles bool
	IncludeDirs  bool
	ExcludeDirs  []string
	ExportCSV    string
	MinSizeBytes int64
	MaxDepth     int
	ShowBanner   bool
}

type ScanOutputOptions struct {
	Format     string
	OutputPath string
	Quiet      bool
}

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

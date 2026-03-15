package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syskit/internal/cliutil"
	"syskit/internal/errs"

	"github.com/spf13/cobra"
)

func runLegacyScan(cmd *cobra.Command, args []string, version string, opts *rootFlags, global *globalOptions) error {
	// 根命令当前仍保留“兼容扫描模式”。
	// 这段逻辑后续会逐步弱化，但在 P0 期间仍用于兼容旧用法。
	consoleOut := cmd.OutOrStdout()
	resultOut := consoleOut

	if opts.showVersion {
		fmt.Fprintf(consoleOut, "syskit version %s\n", version)
		fmt.Fprintln(consoleOut, "跨平台本地系统运维 CLI 工具")
		return nil
	}

	closeOutput, err := cliutil.ConfigureOutputWriter(global.format, global.outputPath, &resultOut)
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

	excludeDirs := cliutil.SplitCSV(opts.excludeDirs)
	return cliutil.RunScan(consoleOut, resultOut, cliutil.ScanRunOptions{
		Path:         scanPath,
		Version:      version,
		Title:        "文件/文件夹大小分析工具",
		TopN:         opts.topN,
		IncludeFiles: opts.includeFiles,
		IncludeDirs:  opts.includeDirs,
		ExcludeDirs:  excludeDirs,
		ExportCSV:    opts.exportCSV,
		ShowBanner:   true,
	}, cliutil.ScanOutputOptions{
		Format:     global.format,
		OutputPath: global.outputPath,
		Quiet:      global.quiet,
	})
}

// resolveScanPath 负责兼容旧交互：如果根命令没有给路径，就提示用户输入。
// 正式的 `disk scan` 不会走这条逻辑，而是要求显式传入路径。
func resolveScanPath(cmd *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", errs.ExecutionFailed("无法获取当前目录", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "请输入要扫描的目录路径（直接回车使用当前目录: %s）: ", currentDir)
	reader := bufio.NewReader(cmd.InOrStdin())
	input, readErr := reader.ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", errs.ExecutionFailed("读取输入失败", readErr)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return currentDir, nil
	}

	return input, nil
}

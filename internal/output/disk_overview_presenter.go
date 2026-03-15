package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	collector "syskit/internal/collectors/disk"
	"syskit/internal/errs"
	"syskit/pkg/utils"
)

// DiskOverviewPresenter 负责渲染 `disk` 总览结果。
type DiskOverviewPresenter struct {
	overview *collector.Overview
	detail   bool
}

// NewDiskOverviewPresenter 创建磁盘总览 presenter。
func NewDiskOverviewPresenter(overview *collector.Overview, detail bool) *DiskOverviewPresenter {
	return &DiskOverviewPresenter{
		overview: overview,
		detail:   detail,
	}
}

// RenderTable 以文本表格形式渲染磁盘总览。
func (p *DiskOverviewPresenter) RenderTable(w io.Writer) error {
	if p.overview == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "磁盘总览结果为空")
	}

	fmt.Fprintln(w, strings.Repeat("=", 80))
	fmt.Fprintln(w, "磁盘总览")
	fmt.Fprintln(w, strings.Repeat("=", 80))
	fmt.Fprintf(w, "  分区数量: %d\n", p.overview.Summary.PartitionCount)
	fmt.Fprintf(w, "  总容量: %s\n", formatBytes(p.overview.Summary.TotalBytes))
	fmt.Fprintf(w, "  已用容量: %s\n", formatBytes(p.overview.Summary.UsedBytes))
	fmt.Fprintf(w, "  可用容量: %s\n", formatBytes(p.overview.Summary.FreeBytes))
	fmt.Fprintf(w, "  整体使用率: %.2f%%\n", p.overview.Summary.UsagePercent)
	fmt.Fprintln(w)

	if p.detail {
		fmt.Fprintf(w, "%-10s %-8s %-12s %-12s %-12s %-12s %s\n", "挂载点", "使用率", "已用", "可用", "总量", "文件系统", "设备")
	} else {
		fmt.Fprintf(w, "%-10s %-8s %-12s %-12s\n", "挂载点", "使用率", "已用", "总量")
	}
	fmt.Fprintln(w, strings.Repeat("-", 80))

	for _, partition := range p.overview.Partitions {
		if p.detail {
			fmt.Fprintf(
				w,
				"%-10s %-7.2f%% %-12s %-12s %-12s %-12s %s\n",
				partition.MountPoint,
				partition.UsagePercent,
				formatBytes(partition.UsedBytes),
				formatBytes(partition.FreeBytes),
				formatBytes(partition.TotalBytes),
				displayValue(partition.FileSystem, "-"),
				displayValue(partition.Device, "-"),
			)
			continue
		}

		fmt.Fprintf(
			w,
			"%-10s %-7.2f%% %-12s %-12s\n",
			partition.MountPoint,
			partition.UsagePercent,
			formatBytes(partition.UsedBytes),
			formatBytes(partition.TotalBytes),
		)
	}

	if len(p.overview.Warnings) > 0 {
		fmt.Fprintln(w, "\n采集提示")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		for _, warning := range p.overview.Warnings {
			fmt.Fprintf(w, "- %s\n", warning)
		}
	}

	return nil
}

// RenderMarkdown 以 Markdown 格式渲染磁盘总览。
func (p *DiskOverviewPresenter) RenderMarkdown(w io.Writer) error {
	if p.overview == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "磁盘总览结果为空")
	}

	fmt.Fprintln(w, "# 磁盘总览")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| 项目 | 值 |")
	fmt.Fprintln(w, "|---|---|")
	fmt.Fprintf(w, "| 分区数量 | %d |\n", p.overview.Summary.PartitionCount)
	fmt.Fprintf(w, "| 总容量 | %s |\n", formatBytes(p.overview.Summary.TotalBytes))
	fmt.Fprintf(w, "| 已用容量 | %s |\n", formatBytes(p.overview.Summary.UsedBytes))
	fmt.Fprintf(w, "| 可用容量 | %s |\n", formatBytes(p.overview.Summary.FreeBytes))
	fmt.Fprintf(w, "| 整体使用率 | %.2f%% |\n", p.overview.Summary.UsagePercent)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## 分区明细")
	fmt.Fprintln(w)

	if p.detail {
		fmt.Fprintln(w, "| 挂载点 | 使用率 | 已用 | 可用 | 总量 | 文件系统 | 设备 |")
		fmt.Fprintln(w, "|---|---|---|---|---|---|---|")
		for _, partition := range p.overview.Partitions {
			fmt.Fprintf(
				w,
				"| %s | %.2f%% | %s | %s | %s | %s | %s |\n",
				filepath.ToSlash(partition.MountPoint),
				partition.UsagePercent,
				formatBytes(partition.UsedBytes),
				formatBytes(partition.FreeBytes),
				formatBytes(partition.TotalBytes),
				displayValue(partition.FileSystem, "-"),
				displayValue(partition.Device, "-"),
			)
		}
	} else {
		fmt.Fprintln(w, "| 挂载点 | 使用率 | 已用 | 总量 |")
		fmt.Fprintln(w, "|---|---|---|---|")
		for _, partition := range p.overview.Partitions {
			fmt.Fprintf(
				w,
				"| %s | %.2f%% | %s | %s |\n",
				filepath.ToSlash(partition.MountPoint),
				partition.UsagePercent,
				formatBytes(partition.UsedBytes),
				formatBytes(partition.TotalBytes),
			)
		}
	}

	if len(p.overview.Warnings) > 0 {
		fmt.Fprintln(w, "\n## 采集提示")
		fmt.Fprintln(w)
		for _, warning := range p.overview.Warnings {
			fmt.Fprintf(w, "- %s\n", warning)
		}
	}

	return nil
}

// RenderCSV 把分区信息导出到 CSV 文件，便于后续二次分析。
func (p *DiskOverviewPresenter) RenderCSV(w io.Writer, prefix string) error {
	if p.overview == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "磁盘总览结果为空")
	}

	if prefix == "" {
		prefix = "result"
	}

	partitionsFile := prefix + "_disk.csv"
	if err := writeDiskPartitionsCSV(partitionsFile, p.overview); err != nil {
		return err
	}
	fmt.Fprintf(w, "✓ 磁盘分区结果已导出到: %s\n", partitionsFile)

	if len(p.overview.Warnings) > 0 {
		warningsFile := prefix + "_disk_warnings.csv"
		if err := writeDiskWarningsCSV(warningsFile, p.overview.Warnings); err != nil {
			return err
		}
		fmt.Fprintf(w, "✓ 磁盘采集提示已导出到: %s\n", warningsFile)
	}

	return nil
}

func writeDiskPartitionsCSV(path string, overview *collector.Overview) error {
	file, err := os.Create(path)
	if err != nil {
		return errs.ExecutionFailed("创建磁盘 CSV 文件失败", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{
		"挂载点",
		"设备",
		"卷名",
		"文件系统",
		"总容量(字节)",
		"已用容量(字节)",
		"可用容量(字节)",
		"使用率(%)",
		"只读",
	}); err != nil {
		return errs.ExecutionFailed("写入磁盘 CSV 表头失败", err)
	}

	for _, partition := range overview.Partitions {
		record := []string{
			partition.MountPoint,
			partition.Device,
			partition.VolumeName,
			partition.FileSystem,
			strconv.FormatUint(partition.TotalBytes, 10),
			strconv.FormatUint(partition.UsedBytes, 10),
			strconv.FormatUint(partition.FreeBytes, 10),
			fmt.Sprintf("%.2f", partition.UsagePercent),
			strconv.FormatBool(partition.ReadOnly),
		}
		if err := writer.Write(record); err != nil {
			return errs.ExecutionFailed("写入磁盘 CSV 内容失败", err)
		}
	}

	return nil
}

func writeDiskWarningsCSV(path string, warnings []string) error {
	file, err := os.Create(path)
	if err != nil {
		return errs.ExecutionFailed("创建磁盘告警 CSV 文件失败", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"告警信息"}); err != nil {
		return errs.ExecutionFailed("写入磁盘告警 CSV 表头失败", err)
	}

	for _, warning := range warnings {
		if err := writer.Write([]string{warning}); err != nil {
			return errs.ExecutionFailed("写入磁盘告警 CSV 内容失败", err)
		}
	}

	return nil
}

func formatBytes(value uint64) string {
	if value > math.MaxInt64 {
		return fmt.Sprintf("%d B", value)
	}
	return utils.FormatBytes(int64(value))
}

func displayValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

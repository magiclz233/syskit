// Package disk 提供磁盘总览采集能力，供 `disk` 与后续 `doctor disk` 复用。
package disk

import (
	"sort"
	"time"
)

// Partition 表示单个分区（或卷）的容量快照。
type Partition struct {
	Device       string  `json:"device"`
	VolumeName   string  `json:"volume_name,omitempty"`
	MountPoint   string  `json:"mount_point"`
	FileSystem   string  `json:"file_system,omitempty"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	ReadOnly     bool    `json:"read_only"`
}

// Summary 是磁盘总览的聚合统计。
type Summary struct {
	PartitionCount int     `json:"partition_count"`
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
}

// Overview 是 `disk` 命令的核心结构化输出。
// 当前 P0 先聚焦“容量总览”，增长趋势会在快照/规则闭环落地后补齐。
type Overview struct {
	CollectedAt time.Time   `json:"collected_at"`
	Summary     Summary     `json:"summary"`
	Partitions  []Partition `json:"partitions"`
	Warnings    []string    `json:"warnings,omitempty"`
}

// CollectOverview 采集磁盘总览信息。
func CollectOverview() (*Overview, error) {
	partitions, warnings, err := collectPartitions()
	if err != nil {
		return nil, err
	}

	// 统一按使用率降序排序，便于第一眼发现高风险分区。
	sort.Slice(partitions, func(i, j int) bool {
		if partitions[i].UsagePercent == partitions[j].UsagePercent {
			return partitions[i].MountPoint < partitions[j].MountPoint
		}
		return partitions[i].UsagePercent > partitions[j].UsagePercent
	})

	return &Overview{
		CollectedAt: time.Now().UTC(),
		Summary:     buildSummary(partitions),
		Partitions:  partitions,
		Warnings:    warnings,
	}, nil
}

// buildSummary 计算整体容量统计，作为 `disk` 命令的顶部摘要。
func buildSummary(partitions []Partition) Summary {
	summary := Summary{
		PartitionCount: len(partitions),
	}

	for _, partition := range partitions {
		summary.TotalBytes += partition.TotalBytes
		summary.UsedBytes += partition.UsedBytes
		summary.FreeBytes += partition.FreeBytes
	}

	summary.UsagePercent = usagePercent(summary.TotalBytes, summary.UsedBytes)
	return summary
}

// usagePercent 返回保留两位小数的使用率百分比。
func usagePercent(totalBytes uint64, usedBytes uint64) float64 {
	if totalBytes == 0 {
		return 0
	}

	return float64(int((float64(usedBytes)/float64(totalBytes))*10000+0.5)) / 100
}

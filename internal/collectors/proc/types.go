// Package proc 提供进程相关采集和执行能力，供 `proc` 命令集复用。
package proc

import (
	"fmt"
	"strings"
	"time"

	"syskit/internal/errs"
)

// SortBy 定义 `proc top` 的排序维度。
type SortBy string

const (
	// SortByCPU 按 CPU 占用排序。
	SortByCPU SortBy = "cpu"
	// SortByMem 按 RSS 内存占用排序。
	SortByMem SortBy = "mem"
	// SortByIO 按 I/O 总字节排序。
	SortByIO SortBy = "io"
	// SortByFD 按文件描述符数量排序。
	SortByFD SortBy = "fd"
)

// ParseSortBy 解析并校验排序维度。
func ParseSortBy(raw string) (SortBy, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch SortBy(normalized) {
	case SortByCPU, SortByMem, SortByIO, SortByFD:
		return SortBy(normalized), nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--by 仅支持 cpu/mem/io/fd，当前为: %s", raw))
	}
}

// ProcessSnapshot 是进程采样后的统一结构。
// 后续 `proc top/tree/info` 均以该结构衍生结果，避免重复采集字段。
type ProcessSnapshot struct {
	PID          int32     `json:"pid"`
	PPID         int32     `json:"ppid"`
	Name         string    `json:"name"`
	User         string    `json:"user,omitempty"`
	Command      string    `json:"command,omitempty"`
	Executable   string    `json:"executable,omitempty"`
	StartTime    time.Time `json:"start_time,omitempty"`
	CPUPercent   float64   `json:"cpu_percent"`
	CPUSeconds   float64   `json:"cpu_seconds"`
	RSSBytes     uint64    `json:"rss_bytes"`
	VMSBytes     uint64    `json:"vms_bytes"`
	IOReadBytes  uint64    `json:"io_read_bytes"`
	IOWriteBytes uint64    `json:"io_write_bytes"`
	FDCount      int32     `json:"fd_count"`
	ThreadCount  int32     `json:"thread_count"`
}

// IOBytes 返回该进程累计 I/O 字节总量。
func (p ProcessSnapshot) IOBytes() uint64 {
	return p.IOReadBytes + p.IOWriteBytes
}

// TopOptions 是 `proc top` 输入参数。
type TopOptions struct {
	By    SortBy
	TopN  int
	User  string
	Name  string
	Watch bool
}

// TopResult 是 `proc top` 的结构化输出。
type TopResult struct {
	By           SortBy            `json:"by"`
	TopN         int               `json:"top_n"`
	Watch        bool              `json:"watch"`
	TotalMatched int               `json:"total_matched"`
	Processes    []ProcessSnapshot `json:"processes"`
	Warnings     []string          `json:"warnings,omitempty"`
}

// TreeOptions 是 `proc tree` 输入参数。
type TreeOptions struct {
	RootPID *int32
	Detail  bool
	Full    bool
}

// TreeNode 表示进程树中的一个节点。
type TreeNode struct {
	PID        int32       `json:"pid"`
	PPID       int32       `json:"ppid"`
	Name       string      `json:"name"`
	User       string      `json:"user,omitempty"`
	Command    string      `json:"command,omitempty"`
	CPUPercent float64     `json:"cpu_percent,omitempty"`
	RSSBytes   uint64      `json:"rss_bytes,omitempty"`
	Children   []*TreeNode `json:"children,omitempty"`
}

// TreeResult 是 `proc tree` 的结构化输出。
type TreeResult struct {
	RootPID   *int32      `json:"root_pid,omitempty"`
	Detail    bool        `json:"detail"`
	Full      bool        `json:"full"`
	Truncated bool        `json:"truncated"`
	Nodes     []*TreeNode `json:"nodes"`
	Warnings  []string    `json:"warnings,omitempty"`
}

// InfoResult 是 `proc info` 的结构化输出。
type InfoResult struct {
	Process     ProcessSnapshot   `json:"process"`
	Parent      *ProcessSnapshot  `json:"parent,omitempty"`
	Children    []ProcessSnapshot `json:"children,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Warnings    []string          `json:"warnings,omitempty"`
}

// KillTarget 是 kill 计划中的单个目标。
type KillTarget struct {
	PID   int32  `json:"pid"`
	Name  string `json:"name"`
	Depth int    `json:"depth"`
}

// KillPlan 表示 `proc kill` 规划阶段的输出。
type KillPlan struct {
	RootPID  int32        `json:"root_pid"`
	RootName string       `json:"root_name"`
	Force    bool         `json:"force"`
	Tree     bool         `json:"tree"`
	Targets  []KillTarget `json:"targets"`
	Steps    []string     `json:"steps"`
	Warnings []string     `json:"warnings,omitempty"`
}

// KillTargetResult 表示单个目标的执行结果。
type KillTargetResult struct {
	PID     int32  `json:"pid"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// KillResult 是 `proc kill` 执行阶段的结构化输出。
type KillResult struct {
	Plan       *KillPlan          `json:"plan"`
	Applied    bool               `json:"applied"`
	Verified   bool               `json:"verified"`
	Results    []KillTargetResult `json:"results"`
	Warnings   []string           `json:"warnings,omitempty"`
	FailedPIDs []int32            `json:"failed_pids,omitempty"`
}

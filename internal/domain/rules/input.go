package rules

import "time"

// DiagnoseOptions 是规则执行期的可调参数。
type DiagnoseOptions struct {
	Thresholds RuleThresholds `json:"thresholds,omitempty"`
	Excludes   RuleExcludes   `json:"excludes,omitempty"`
	Policy     RulePolicy     `json:"policy,omitempty"`
}

// RuleThresholds 定义 P0 规则使用的阈值集合。
type RuleThresholds struct {
	CPUPercent          float64 `json:"cpu_percent,omitempty"`
	MemPercent          float64 `json:"mem_percent,omitempty"`
	DiskPercent         float64 `json:"disk_percent,omitempty"`
	ConnectionCount     int     `json:"connection_count,omitempty"`
	FileSizeGB          float64 `json:"file_size_gb,omitempty"`
	SwapPercent         float64 `json:"swap_percent,omitempty"`
	FileGrowthMBPerHour float64 `json:"file_growth_mb_per_hour,omitempty"`
}

// RuleExcludes 定义规则判定时应忽略的端口和进程。
type RuleExcludes struct {
	Ports     []int    `json:"ports,omitempty"`
	Processes []string `json:"processes,omitempty"`
}

// RulePolicy 定义规则判定所需的策略输入。
type RulePolicy struct {
	CriticalPorts        []int    `json:"critical_ports,omitempty"`
	AllowPublicListen    []string `json:"allow_public_listen,omitempty"`
	RequiredServices     []string `json:"required_services,omitempty"`
	RequiredStartupItems []string `json:"required_startup_items,omitempty"`
}

// DiagnoseSnapshots 是规则引擎可直接消费的归一化快照。
type DiagnoseSnapshots struct {
	Ports        []PortSnapshot          `json:"ports,omitempty"`
	Processes    []ProcessSnapshot       `json:"processes,omitempty"`
	CPU          *CPUOverviewSnapshot    `json:"cpu,omitempty"`
	Memory       *MemoryOverview         `json:"memory,omitempty"`
	MemoryTop    []MemoryProcess         `json:"memory_top,omitempty"`
	Disk         []DiskPartition         `json:"disk,omitempty"`
	DiskGrowth   []DiskGrowthSample      `json:"disk_growth,omitempty"`
	Files        []FileObservation       `json:"files,omitempty"`
	PathEntries  []string                `json:"path_entries,omitempty"`
	Connections  []ConnectionObservation `json:"connections,omitempty"`
	Services     []ServiceObservation    `json:"services,omitempty"`
	StartupItems []StartupObservation    `json:"startup_items,omitempty"`
	Log          *LogObservation         `json:"log,omitempty"`
}

// PortSnapshot 表示端口监听记录。
type PortSnapshot struct {
	Port        int    `json:"port"`
	LocalAddr   string `json:"local_addr"`
	PID         int32  `json:"pid"`
	ProcessName string `json:"process_name,omitempty"`
	Command     string `json:"command,omitempty"`
	ParentPID   int32  `json:"parent_pid,omitempty"`
}

// ProcessSnapshot 表示进程资源快照。
type ProcessSnapshot struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	Command    string  `json:"command,omitempty"`
	CPUPercent float64 `json:"cpu_percent"`
	RSSBytes   uint64  `json:"rss_bytes"`
	VMSBytes   uint64  `json:"vms_bytes"`
}

// CPUOverviewSnapshot 表示 CPU 概览快照。
type CPUOverviewSnapshot struct {
	CPUCores     int               `json:"cpu_cores"`
	UsagePercent float64           `json:"usage_percent"`
	Load1        float64           `json:"load1,omitempty"`
	Load5        float64           `json:"load5,omitempty"`
	Load15       float64           `json:"load15,omitempty"`
	TopProcesses []ProcessSnapshot `json:"top_processes,omitempty"`
}

// MemoryOverview 表示内存概览快照。
type MemoryOverview struct {
	TotalBytes       uint64  `json:"total_bytes"`
	AvailableBytes   uint64  `json:"available_bytes"`
	UsagePercent     float64 `json:"usage_percent"`
	SwapUsagePercent float64 `json:"swap_usage_percent"`
}

// MemoryProcess 表示进程内存快照。
type MemoryProcess struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	Command    string  `json:"command,omitempty"`
	MemPercent float64 `json:"mem_percent"`
	RSSBytes   uint64  `json:"rss_bytes"`
	SwapBytes  uint64  `json:"swap_bytes"`
}

// DiskPartition 表示分区容量快照。
type DiskPartition struct {
	MountPoint   string  `json:"mount_point"`
	UsagePercent float64 `json:"usage_percent"`
	FreeBytes    uint64  `json:"free_bytes"`
}

// DiskGrowthSample 表示分区增长速率样本。
type DiskGrowthSample struct {
	MountPoint         string  `json:"mount_point"`
	GrowthRateGBPerDay float64 `json:"growth_rate_gb_per_day"`
	BaselineGBPerDay   float64 `json:"baseline_gb_per_day"`
	WindowDays         int     `json:"window_days"`
}

// FileObservation 表示文件体积和增长观测数据。
type FileObservation struct {
	Path             string    `json:"path"`
	SizeBytes        int64     `json:"size_bytes"`
	GrowthMBPerHour  float64   `json:"growth_mb_per_hour"`
	LastModifiedTime time.Time `json:"last_modified_time,omitempty"`
}

// ConnectionObservation 表示网络连接观测。
type ConnectionObservation struct {
	LocalAddr  string `json:"local_addr,omitempty"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	State      string `json:"state,omitempty"`
	Process    string `json:"process,omitempty"`
}

// ServiceObservation 表示服务状态观测。
type ServiceObservation struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Startup string `json:"startup,omitempty"`
}

// StartupObservation 表示启动项观测。
type StartupObservation struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Risk       bool   `json:"risk"`
	RiskReason string `json:"risk_reason,omitempty"`
	Path       string `json:"path,omitempty"`
}

// LogObservation 表示日志错误观测。
type LogObservation struct {
	ErrorRate       float64 `json:"error_rate"`
	GrowthMBPerHour float64 `json:"growth_mb_per_hour"`
	ErrorLines      int     `json:"error_lines"`
	TotalLines      int     `json:"total_lines"`
}

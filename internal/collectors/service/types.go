// Package service 提供系统服务采集能力，供 `service` 命令组复用。
package service

// ServiceEntry 表示单个系统服务的标准化结构。
// 不同平台原始字段差异较大，这里统一收敛为 Name/State/Startup 等核心字段。
type ServiceEntry struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	State       string `json:"state"`
	Startup     string `json:"startup"`
	PID         int32  `json:"pid,omitempty"`
	Description string `json:"description,omitempty"`
	Platform    string `json:"platform,omitempty"`
}

// ListOptions 定义 `service list` 的过滤参数。
type ListOptions struct {
	State   string
	Startup string
	Name    string
}

// ListResult 是 `service list` 的结构化输出。
type ListResult struct {
	Platform      string         `json:"platform"`
	StateFilter   []string       `json:"state_filter,omitempty"`
	StartupFilter []string       `json:"startup_filter,omitempty"`
	NameFilter    string         `json:"name_filter,omitempty"`
	Total         int            `json:"total"`
	Services      []ServiceEntry `json:"services"`
	Warnings      []string       `json:"warnings,omitempty"`
}

// CheckOptions 定义 `service check` 的行为开关。
type CheckOptions struct {
	All    bool
	Detail bool
}

// CheckResult 是 `service check` 的结构化输出。
type CheckResult struct {
	Platform string         `json:"platform"`
	Name     string         `json:"name"`
	All      bool           `json:"all"`
	Detail   bool           `json:"detail"`
	Found    bool           `json:"found"`
	Healthy  bool           `json:"healthy"`
	Matched  int            `json:"matched"`
	Running  int            `json:"running"`
	Summary  string         `json:"summary"`
	Services []ServiceEntry `json:"services"`
	Warnings []string       `json:"warnings,omitempty"`
}

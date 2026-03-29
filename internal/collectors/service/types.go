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

// Action 定义服务写操作类型。
type Action string

const (
	// ActionStart 表示启动服务。
	ActionStart Action = "start"
	// ActionStop 表示停止服务。
	ActionStop Action = "stop"
	// ActionRestart 表示重启服务。
	ActionRestart Action = "restart"
	// ActionEnable 表示启用开机自启。
	ActionEnable Action = "enable"
	// ActionDisable 表示禁用开机自启。
	ActionDisable Action = "disable"
)

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

// ActionPlan 是服务写操作的 dry-run 计划。
type ActionPlan struct {
	Action   Action       `json:"action"`
	Name     string       `json:"name"`
	Platform string       `json:"platform"`
	Found    bool         `json:"found"`
	Current  ServiceEntry `json:"current,omitempty"`
	Steps    []string     `json:"steps"`
	Warnings []string     `json:"warnings,omitempty"`
}

// ActionResult 是服务写操作真实执行结果。
type ActionResult struct {
	Action   Action       `json:"action"`
	Name     string       `json:"name"`
	Platform string       `json:"platform"`
	Applied  bool         `json:"applied"`
	Success  bool         `json:"success"`
	Summary  string       `json:"summary"`
	Before   ServiceEntry `json:"before,omitempty"`
	After    ServiceEntry `json:"after,omitempty"`
	Warnings []string     `json:"warnings,omitempty"`
}

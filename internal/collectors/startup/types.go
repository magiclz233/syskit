// Package startup 提供系统启动项采集与管理能力。
package startup

// Item 表示单个启动项。
type Item struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Command    string `json:"command,omitempty"`
	Location   string `json:"location,omitempty"`
	User       string `json:"user,omitempty"`
	Enabled    bool   `json:"enabled"`
	Risk       bool   `json:"risk"`
	RiskReason string `json:"risk_reason,omitempty"`
	Platform   string `json:"platform,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
}

// ListOptions 是启动项列表参数。
type ListOptions struct {
	OnlyRisk bool
	User     string
}

// ListResult 是 `startup list` 输出。
type ListResult struct {
	Platform string   `json:"platform"`
	OnlyRisk bool     `json:"only_risk"`
	User     string   `json:"user,omitempty"`
	Total    int      `json:"total"`
	Items    []Item   `json:"items"`
	Warnings []string `json:"warnings,omitempty"`
}

// Action 定义启动项写操作。
type Action string

const (
	// ActionEnable 启用启动项。
	ActionEnable Action = "enable"
	// ActionDisable 禁用启动项。
	ActionDisable Action = "disable"
)

// ActionPlan 是启动项写操作计划。
type ActionPlan struct {
	Action   Action   `json:"action"`
	ID       string   `json:"id"`
	Platform string   `json:"platform"`
	Found    bool     `json:"found"`
	Current  Item     `json:"current,omitempty"`
	Steps    []string `json:"steps"`
	Warnings []string `json:"warnings,omitempty"`
}

// ActionResult 是启动项写操作结果。
type ActionResult struct {
	Action   Action   `json:"action"`
	ID       string   `json:"id"`
	Platform string   `json:"platform"`
	Applied  bool     `json:"applied"`
	Success  bool     `json:"success"`
	Summary  string   `json:"summary"`
	Before   Item     `json:"before,omitempty"`
	After    Item     `json:"after,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

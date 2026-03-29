// Package fixrun 提供 `fix run` 剧本执行能力。
package fixrun

import "time"

// StepPlan 是单个剧本步骤计划。
type StepPlan struct {
	Name    string `json:"name"`
	Builtin bool   `json:"builtin"`
	Action  string `json:"action"`
}

// Plan 是 `fix run` dry-run 计划。
type Plan struct {
	Script   string     `json:"script"`
	Apply    bool       `json:"apply"`
	OnFail   string     `json:"on_fail"`
	Steps    []StepPlan `json:"steps"`
	Warnings []string   `json:"warnings,omitempty"`
}

// StepResult 是单个步骤执行结果。
type StepResult struct {
	Name       string    `json:"name"`
	Builtin    bool      `json:"builtin"`
	Applied    bool      `json:"applied"`
	Success    bool      `json:"success"`
	DurationMs int64     `json:"duration_ms"`
	Summary    string    `json:"summary"`
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
}

// Result 是 `fix run` 执行结果。
type Result struct {
	Applied   bool         `json:"applied"`
	Success   bool         `json:"success"`
	OnFail    string       `json:"on_fail"`
	StepCount int          `json:"step_count"`
	Succeeded int          `json:"succeeded"`
	Failed    int          `json:"failed"`
	Steps     []StepResult `json:"steps"`
	Summary   string       `json:"summary"`
	Warnings  []string     `json:"warnings,omitempty"`
}

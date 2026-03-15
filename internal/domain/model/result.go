// Package model 定义命令统一输出协议的核心数据结构。
package model

import "time"

// CommandResult 是所有命令最终输出的统一包裹结构。
// 无论是 JSON 成功结果还是 JSON 错误结果，都应尽量保持这个外层形状稳定。
type CommandResult struct {
	Code     int        `json:"code"`
	Msg      string     `json:"msg"`
	Data     any        `json:"data"`
	Error    *ErrorInfo `json:"error,omitempty"`
	Metadata Metadata   `json:"metadata"`
}

// Metadata 保存一次命令执行的上下文信息。
// 这些字段既方便排错，也为后续快照、审计、报告等能力预留了关联键。
type Metadata struct {
	SchemaVersion string    `json:"schema_version"`
	Timestamp     time.Time `json:"timestamp"`
	Host          string    `json:"host"`
	Command       string    `json:"command"`
	ExecutionMs   int64     `json:"execution_ms"`
	Platform      string    `json:"platform"`
	TraceID       string    `json:"trace_id"`
}

// ErrorInfo 描述结构化错误信息。
type ErrorInfo struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Suggestion   string `json:"suggestion"`
}

package model

import "time"

type CommandResult struct {
	Code     int        `json:"code"`
	Msg      string     `json:"msg"`
	Data     any        `json:"data"`
	Error    *ErrorInfo `json:"error,omitempty"`
	Metadata Metadata   `json:"metadata"`
}

type Metadata struct {
	SchemaVersion string    `json:"schema_version"`
	Timestamp     time.Time `json:"timestamp"`
	Host          string    `json:"host"`
	Command       string    `json:"command"`
	ExecutionMs   int64     `json:"execution_ms"`
	Platform      string    `json:"platform"`
	TraceID       string    `json:"trace_id"`
}

type ErrorInfo struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Suggestion   string `json:"suggestion"`
}

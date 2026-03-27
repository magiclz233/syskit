// Package net 提供网络连接审计、监听列表和带宽测速能力。
package net

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"syskit/internal/collectors/port"
	"syskit/internal/errs"
)

var allowedStates = map[string]struct{}{
	"listen":      {},
	"established": {},
	"time_wait":   {},
	"close_wait":  {},
	"syn_sent":    {},
	"syn_recv":    {},
	"fin_wait1":   {},
	"fin_wait2":   {},
	"last_ack":    {},
	"closing":     {},
	"closed":      {},
	"none":        {},
}

// SpeedMode 表示 `net speed` 的测速模式。
type SpeedMode string

const (
	// SpeedModeFull 同时执行延迟、下载、上传测试。
	SpeedModeFull SpeedMode = "full"
	// SpeedModeDownload 仅执行下载测速。
	SpeedModeDownload SpeedMode = "download"
	// SpeedModeUpload 仅执行上传测速。
	SpeedModeUpload SpeedMode = "upload"
)

// ConnectionEntry 是网络连接与监听列表的统一输出条目。
type ConnectionEntry struct {
	Protocol    string `json:"protocol"`
	State       string `json:"state"`
	LocalAddr   string `json:"local_addr"`
	RemoteAddr  string `json:"remote_addr,omitempty"`
	PID         int32  `json:"pid"`
	ProcessName string `json:"process_name,omitempty"`
	User        string `json:"user,omitempty"`
	Command     string `json:"command,omitempty"`
}

// ConnOptions 是 `net conn` 参数集合。
type ConnOptions struct {
	PID      int32
	States   []string
	Protocol port.Protocol
	Remote   string
}

// ConnResult 是 `net conn` 输出结构。
type ConnResult struct {
	PID         int32             `json:"pid,omitempty"`
	States      []string          `json:"states,omitempty"`
	Protocol    string            `json:"protocol,omitempty"`
	Remote      string            `json:"remote,omitempty"`
	Total       int               `json:"total"`
	Connections []ConnectionEntry `json:"connections"`
	Warnings    []string          `json:"warnings,omitempty"`
}

// ListenOptions 是 `net listen` 参数集合。
type ListenOptions struct {
	Protocol port.Protocol
	Addr     string
}

// ListenResult 是 `net listen` 输出结构。
type ListenResult struct {
	Protocol string            `json:"protocol,omitempty"`
	Addr     string            `json:"addr,omitempty"`
	Total    int               `json:"total"`
	Listen   []ConnectionEntry `json:"listen"`
	Warnings []string          `json:"warnings,omitempty"`
}

// SpeedOptions 是 `net speed` 参数集合。
type SpeedOptions struct {
	Server  string
	Mode    SpeedMode
	Timeout time.Duration
}

// SpeedPingStats 表示延迟测试统计。
type SpeedPingStats struct {
	Count        int     `json:"count"`
	SuccessCount int     `json:"success_count"`
	FailureCount int     `json:"failure_count"`
	LossRate     float64 `json:"loss_rate"`
	MinMs        float64 `json:"min_ms"`
	AvgMs        float64 `json:"avg_ms"`
	MaxMs        float64 `json:"max_ms"`
	JitterMs     float64 `json:"jitter_ms"`
}

// SpeedSample 表示下载或上传的速度样本。
type SpeedSample struct {
	Bytes      int64   `json:"bytes"`
	DurationMs float64 `json:"duration_ms"`
	Mbps       float64 `json:"mbps"`
}

// SpeedResult 是 `net speed` 输出结构。
type SpeedResult struct {
	Server     string          `json:"server"`
	Mode       string          `json:"mode"`
	PublicIP   string          `json:"public_ip,omitempty"`
	Ping       *SpeedPingStats `json:"ping,omitempty"`
	Download   *SpeedSample    `json:"download,omitempty"`
	Upload     *SpeedSample    `json:"upload,omitempty"`
	DurationMs float64         `json:"duration_ms"`
	Warnings   []string        `json:"warnings,omitempty"`
}

// ParseStateFilter 解析 `net conn --state` 参数，支持逗号分隔。
func ParseStateFilter(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	set := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		state := strings.ToLower(strings.TrimSpace(part))
		if state == "" {
			continue
		}
		if _, ok := allowedStates[state]; !ok {
			return nil, errs.InvalidArgument(fmt.Sprintf("--state 包含不支持的状态: %s", part))
		}
		set[state] = struct{}{}
	}
	if len(set) == 0 {
		return nil, errs.InvalidArgument("--state 不能为空")
	}

	result := make([]string, 0, len(set))
	for state := range set {
		result = append(result, state)
	}
	slices.Sort(result)
	return result, nil
}

// ParseSpeedMode 解析 `net speed --mode` 参数。
func ParseSpeedMode(raw string) (SpeedMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return SpeedModeFull, nil
	}
	switch SpeedMode(normalized) {
	case SpeedModeFull, SpeedModeDownload, SpeedModeUpload:
		return SpeedMode(normalized), nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--mode 仅支持 full/download/upload，当前为: %s", raw))
	}
}

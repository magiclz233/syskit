// Package networkprobe 提供主机连通性与路由跟踪能力。
package networkprobe

import (
	"fmt"
	"strings"
	"syskit/internal/errs"
	"time"
)

// TraceProtocol 表示 traceroute 使用的探测协议。
type TraceProtocol string

const (
	// TraceProtocolICMP 表示 ICMP 探测。
	TraceProtocolICMP TraceProtocol = "icmp"
	// TraceProtocolTCP 表示 TCP 探测。
	TraceProtocolTCP TraceProtocol = "tcp"
)

// ParseTraceProtocol 解析 `--proto` 参数。
func ParseTraceProtocol(raw string) (TraceProtocol, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return TraceProtocolICMP, nil
	}
	switch TraceProtocol(normalized) {
	case TraceProtocolICMP, TraceProtocolTCP:
		return TraceProtocol(normalized), nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--proto 仅支持 icmp/tcp，当前为: %s", raw))
	}
}

// PingOptions 是 `ping` 参数集合。
type PingOptions struct {
	Target   string
	Count    int
	Interval time.Duration
	Timeout  time.Duration
	Size     int
}

// PingAttempt 表示单次 Ping 结果。
type PingAttempt struct {
	Seq       int     `json:"seq"`
	Success   bool    `json:"success"`
	LatencyMs float64 `json:"latency_ms"`
	TTL       int     `json:"ttl,omitempty"`
	Error     string  `json:"error,omitempty"`
}

// PingResult 是 `ping` 输出结构。
type PingResult struct {
	Target       string        `json:"target"`
	Count        int           `json:"count"`
	IntervalMs   int64         `json:"interval_ms"`
	TimeoutMs    int64         `json:"timeout_ms"`
	Size         int           `json:"size"`
	SuccessCount int           `json:"success_count"`
	FailureCount int           `json:"failure_count"`
	LossRate     float64       `json:"loss_rate"`
	AvgLatencyMs float64       `json:"avg_latency_ms"`
	MinLatencyMs float64       `json:"min_latency_ms"`
	MaxLatencyMs float64       `json:"max_latency_ms"`
	JitterMs     float64       `json:"jitter_ms"`
	Attempts     []PingAttempt `json:"attempts"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// TracerouteOptions 是 `traceroute` 参数集合。
type TracerouteOptions struct {
	Target   string
	MaxHops  int
	Timeout  time.Duration
	Protocol TraceProtocol
}

// TracerouteHop 表示一跳路由结果。
type TracerouteHop struct {
	Hop          int       `json:"hop"`
	Host         string    `json:"host,omitempty"`
	IP           string    `json:"ip,omitempty"`
	RTTsMs       []float64 `json:"rtts_ms,omitempty"`
	AvgLatencyMs float64   `json:"avg_latency_ms,omitempty"`
	Timeout      bool      `json:"timeout"`
}

// TracerouteResult 是 `traceroute` 输出结构。
type TracerouteResult struct {
	Target    string          `json:"target"`
	Protocol  string          `json:"protocol"`
	MaxHops   int             `json:"max_hops"`
	TimeoutMs int64           `json:"timeout_ms"`
	HopCount  int             `json:"hop_count"`
	Reached   bool            `json:"reached"`
	Hops      []TracerouteHop `json:"hops"`
	Warnings  []string        `json:"warnings,omitempty"`
}

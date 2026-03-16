// Package port 提供端口查询、列表和释放所需的数据模型与采集执行能力。
package port

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syskit/internal/errs"
)

// Protocol 表示传输层协议。
type Protocol string

const (
	// ProtocolTCP 表示 TCP。
	ProtocolTCP Protocol = "tcp"
	// ProtocolUDP 表示 UDP。
	ProtocolUDP Protocol = "udp"
)

// ParseProtocol 解析协议过滤参数。
func ParseProtocol(raw string) (Protocol, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "", nil
	}
	switch Protocol(normalized) {
	case ProtocolTCP, ProtocolUDP:
		return Protocol(normalized), nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--protocol 仅支持 tcp/udp，当前为: %s", raw))
	}
}

// ParseSortBy 解析 `port list --by` 参数。
func ParseSortBy(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "port", nil
	}
	switch normalized {
	case "pid", "port":
		return normalized, nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--by 仅支持 pid/port，当前为: %s", raw))
	}
}

// ParsePortExpression 解析 `80,443,8080-8090` 形式的端口表达式。
func ParsePortExpression(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errs.InvalidArgument("端口表达式不能为空")
	}

	parts := strings.Split(raw, ",")
	set := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errs.InvalidArgument(fmt.Sprintf("无效端口表达式: %s", raw))
		}

		if strings.Contains(part, "-") {
			startRaw, endRaw, ok := strings.Cut(part, "-")
			if !ok {
				return nil, errs.InvalidArgument(fmt.Sprintf("无效端口范围: %s", part))
			}
			start, err := parsePortValue(startRaw)
			if err != nil {
				return nil, err
			}
			end, err := parsePortValue(endRaw)
			if err != nil {
				return nil, err
			}
			if start > end {
				return nil, errs.InvalidArgument(fmt.Sprintf("端口范围起始不能大于结束: %s", part))
			}
			for value := start; value <= end; value++ {
				set[value] = struct{}{}
			}
			continue
		}

		value, err := parsePortValue(part)
		if err != nil {
			return nil, err
		}
		set[value] = struct{}{}
	}

	if len(set) == 0 {
		return nil, errs.InvalidArgument("端口表达式解析结果为空")
	}

	ports := make([]int, 0, len(set))
	for value := range set {
		ports = append(ports, value)
	}
	sort.Ints(ports)
	return ports, nil
}

func parsePortValue(raw string) (int, error) {
	text := strings.TrimSpace(raw)
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效端口: %s", raw))
	}
	if value <= 0 || value > 65535 {
		return 0, errs.InvalidArgument(fmt.Sprintf("端口超出范围(1-65535): %d", value))
	}
	return value, nil
}

// PortEntry 是端口查询和列表的统一条目结构。
type PortEntry struct {
	Port        int      `json:"port"`
	Protocol    Protocol `json:"protocol"`
	LocalAddr   string   `json:"local_addr"`
	Status      string   `json:"status"`
	PID         int32    `json:"pid"`
	ProcessName string   `json:"process_name,omitempty"`
	User        string   `json:"user,omitempty"`
	Command     string   `json:"command,omitempty"`
	ParentPID   int32    `json:"parent_pid,omitempty"`
}

// QueryResult 是 `port <port[,port]|range>` 输出结构。
type QueryResult struct {
	RequestedPorts []int       `json:"requested_ports"`
	FoundPorts     []int       `json:"found_ports"`
	MissingPorts   []int       `json:"missing_ports"`
	Entries        []PortEntry `json:"entries"`
	Warnings       []string    `json:"warnings,omitempty"`
}

// ListOptions 是 `port list` 参数集合。
type ListOptions struct {
	By       string
	Protocol Protocol
	Listen   string
}

// ListResult 是 `port list` 输出结构。
type ListResult struct {
	By       string      `json:"by"`
	Protocol string      `json:"protocol,omitempty"`
	Listen   string      `json:"listen,omitempty"`
	Total    int         `json:"total"`
	Entries  []PortEntry `json:"entries"`
	Warnings []string    `json:"warnings,omitempty"`
}

// KillOptions 是 `port kill` 执行参数。
type KillOptions struct {
	Port     int
	Force    bool
	KillTree bool
}

// KillTarget 表示一个待终止的进程目标。
type KillTarget struct {
	PID         int32      `json:"pid"`
	ProcessName string     `json:"process_name,omitempty"`
	Protocols   []Protocol `json:"protocols,omitempty"`
}

// KillPlan 表示 `port kill` 的 discover/plan 结果。
type KillPlan struct {
	Port       int          `json:"port"`
	Force      bool         `json:"force"`
	KillTree   bool         `json:"kill_tree"`
	Targets    []KillTarget `json:"targets"`
	Steps      []string     `json:"steps"`
	Warnings   []string     `json:"warnings,omitempty"`
	Connection []PortEntry  `json:"connection"`
}

// KillProcessResult 表示单个目标进程的执行结果。
type KillProcessResult struct {
	PID         int32  `json:"pid"`
	ProcessName string `json:"process_name,omitempty"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

// KillResult 是 `port kill` apply 阶段输出。
type KillResult struct {
	Plan          *KillPlan           `json:"plan"`
	Applied       bool                `json:"applied"`
	Released      bool                `json:"released"`
	ProcessResult []KillProcessResult `json:"process_result"`
	Remaining     []PortEntry         `json:"remaining,omitempty"`
	Warnings      []string            `json:"warnings,omitempty"`
}

// Package dns 提供 DNS 解析与性能测试所需的数据模型。
package dns

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"syskit/internal/errs"
)

// ResolveType 表示 DNS 记录类型。
type ResolveType string

const (
	ResolveTypeA     ResolveType = "A"
	ResolveTypeAAAA  ResolveType = "AAAA"
	ResolveTypeCNAME ResolveType = "CNAME"
	ResolveTypeMX    ResolveType = "MX"
	ResolveTypeNS    ResolveType = "NS"
	ResolveTypeTXT   ResolveType = "TXT"
)

// ResolveOptions 是 `dns resolve` 参数集合。
type ResolveOptions struct {
	Domain    string
	Type      ResolveType
	DNSServer string
	Timeout   time.Duration
}

// ResolveRecord 表示一条解析记录。
type ResolveRecord struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ResolveResult 是 `dns resolve` 输出结构。
type ResolveResult struct {
	Domain     string          `json:"domain"`
	QueryType  string          `json:"query_type"`
	DNSServer  string          `json:"dns_server,omitempty"`
	DurationMs float64         `json:"duration_ms"`
	Count      int             `json:"count"`
	Records    []ResolveRecord `json:"records"`
	Warnings   []string        `json:"warnings,omitempty"`
}

// BenchOptions 是 `dns bench` 参数集合。
type BenchOptions struct {
	Domain    string
	Type      ResolveType
	DNSServer string
	Count     int
	Timeout   time.Duration
}

// BenchAttempt 表示单次 DNS 请求结果。
type BenchAttempt struct {
	Seq        int     `json:"seq"`
	Success    bool    `json:"success"`
	DurationMs float64 `json:"duration_ms"`
	RecordCnt  int     `json:"record_count"`
	Error      string  `json:"error,omitempty"`
}

// BenchResult 是 `dns bench` 输出结构。
type BenchResult struct {
	Domain       string         `json:"domain"`
	QueryType    string         `json:"query_type"`
	DNSServer    string         `json:"dns_server,omitempty"`
	Count        int            `json:"count"`
	SuccessCount int            `json:"success_count"`
	FailureCount int            `json:"failure_count"`
	AvgMs        float64        `json:"avg_duration_ms"`
	MinMs        float64        `json:"min_duration_ms"`
	MaxMs        float64        `json:"max_duration_ms"`
	Attempts     []BenchAttempt `json:"attempts"`
	Warnings     []string       `json:"warnings,omitempty"`
}

// ParseResolveType 解析 `--type` 参数。
func ParseResolveType(raw string) (ResolveType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	if normalized == "" {
		return ResolveTypeA, nil
	}
	switch ResolveType(normalized) {
	case ResolveTypeA, ResolveTypeAAAA, ResolveTypeCNAME, ResolveTypeMX, ResolveTypeNS, ResolveTypeTXT:
		return ResolveType(normalized), nil
	default:
		return "", errs.InvalidArgument(fmt.Sprintf("--type 仅支持 A/AAAA/CNAME/MX/NS/TXT，当前为: %s", raw))
	}
}

func normalizeDNSServer(raw string) (string, error) {
	server := strings.TrimSpace(raw)
	if server == "" {
		return "", nil
	}
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server, nil
	}
	if strings.Count(server, ":") > 1 {
		// IPv6 未带端口。
		return net.JoinHostPort(server, "53"), nil
	}
	if strings.Contains(server, ":") {
		// host:port 解析失败时视为非法输入。
		return "", errs.InvalidArgument(fmt.Sprintf("--dns 无效，期望 host 或 host:port，当前为: %s", raw))
	}
	return net.JoinHostPort(server, "53"), nil
}

func uniqueSortedRecords(records []ResolveRecord) []ResolveRecord {
	if len(records) == 0 {
		return nil
	}
	set := make(map[string]ResolveRecord, len(records))
	for _, item := range records {
		key := strings.ToUpper(strings.TrimSpace(item.Type)) + "|" + strings.TrimSpace(item.Value)
		if key == "|" {
			continue
		}
		set[key] = ResolveRecord{
			Type:  strings.ToUpper(strings.TrimSpace(item.Type)),
			Value: strings.TrimSpace(item.Value),
		}
	}
	result := make([]ResolveRecord, 0, len(set))
	for _, item := range set {
		result = append(result, item)
	}
	sort.Slice(result, func(i int, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type < result[j].Type
		}
		return result[i].Value < result[j].Value
	})
	return result
}

func durationMs(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

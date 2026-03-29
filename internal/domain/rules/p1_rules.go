package rules

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"syskit/internal/domain/model"
)

const phaseP1 = "P1"

const (
	defaultConnectionThreshold = 1000
	defaultLogErrorRate        = 20.0
	defaultLogGrowthMBPerHour  = 200.0
)

// NewP1Rules 返回 P1 规则集合。
func NewP1Rules() []Rule {
	return []Rule{
		newP1Rule("NET-001", "net", checkNET001),
		newP1Rule("SVC-001", "service", checkSVC001),
		newP1Rule("STARTUP-001", "startup", checkSTARTUP001),
		newP1Rule("LOG-001", "log", checkLOG001),
	}
}

type p1Rule struct {
	id     string
	module string
	check  func(in DiagnoseInput) *model.Issue
}

func newP1Rule(id string, module string, check func(in DiagnoseInput) *model.Issue) *p1Rule {
	return &p1Rule{id: id, module: module, check: check}
}

func (r *p1Rule) ID() string {
	return r.id
}

func (r *p1Rule) Phase() string {
	return phaseP1
}

func (r *p1Rule) Module() string {
	return r.module
}

func (r *p1Rule) Check(ctx context.Context, in DiagnoseInput) (*model.Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.check == nil {
		return nil, nil
	}
	return r.check(in), nil
}

func checkNET001(in DiagnoseInput) *model.Issue {
	connectionCount := len(in.Snapshots.Connections)
	if connectionCount == 0 {
		return nil
	}
	threshold := in.Options.Thresholds.ConnectionCount
	if threshold <= 0 {
		threshold = defaultConnectionThreshold
	}
	baseline := baselineConnectionCount(in.ModuleData)

	hitByThreshold := connectionCount >= threshold
	hitByBaseline := baseline > 0 && float64(connectionCount) >= float64(baseline)*3
	if !hitByThreshold && !hitByBaseline {
		return nil
	}

	topHosts := topRemoteHosts(in.Snapshots.Connections, 5)
	summary := fmt.Sprintf("连接数 %d 超过阈值 %d", connectionCount, threshold)
	if hitByBaseline {
		summary = fmt.Sprintf("连接数 %d 相对基线 %d 出现异常突增", connectionCount, baseline)
	}
	return &model.Issue{
		RuleID:   "NET-001",
		Severity: model.SeverityHigh,
		Summary:  summary,
		Evidence: map[string]any{
			"connection_count": connectionCount,
			"threshold":        threshold,
			"baseline_count":   baseline,
			"top_remote_hosts": topHosts,
		},
		Impact:      "连接数异常会导致资源占用上升，可能引发连接泄漏或异常流量风险",
		Suggestion:  "检查连接来源、远端主机分布和异常进程外联行为",
		FixCommand:  "syskit net conn --format json",
		AutoFixable: false,
		Confidence:  85,
		Scope:       model.ScopeSystem,
	}
}

func checkSVC001(in DiagnoseInput) *model.Issue {
	required := normalizedRequiredServices(in.Options.Policy.RequiredServices)
	if len(required) == 0 || len(in.Snapshots.Services) == 0 {
		return nil
	}
	running := make(map[string]ServiceObservation, len(in.Snapshots.Services))
	for _, item := range in.Snapshots.Services {
		name := normalizeServiceName(item.Name)
		if name == "" {
			continue
		}
		running[name] = item
	}

	missing := make([]string, 0, len(required))
	for _, service := range required {
		item, ok := running[service]
		if !ok || strings.ToLower(strings.TrimSpace(item.State)) != "running" {
			missing = append(missing, service)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return &model.Issue{
		RuleID:   "SVC-001",
		Severity: model.SeverityCritical,
		Summary:  fmt.Sprintf("关键服务未运行: %s", strings.Join(missing, ",")),
		Evidence: map[string]any{
			"required_services": required,
			"missing_services":  missing,
		},
		Impact:      "关键服务未运行会导致核心能力不可用",
		Suggestion:  "先检查服务依赖和日志，再执行服务重启",
		FixCommand:  "syskit service restart <name> --apply --yes",
		AutoFixable: true,
		Confidence:  90,
		Scope:       model.ScopeSystem,
	}
}

func checkSTARTUP001(in DiagnoseInput) *model.Issue {
	if len(in.Snapshots.StartupItems) == 0 {
		return nil
	}
	allow := stringSet(in.Options.Policy.RequiredStartupItems)
	offenders := make([]StartupObservation, 0, 8)
	for _, item := range in.Snapshots.StartupItems {
		if !item.Risk {
			continue
		}
		if containsNormalized(allow, item.Name) || containsNormalized(allow, item.ID) {
			continue
		}
		offenders = append(offenders, item)
	}
	if len(offenders) == 0 {
		return nil
	}
	sort.Slice(offenders, func(i int, j int) bool {
		if offenders[i].Name != offenders[j].Name {
			return offenders[i].Name < offenders[j].Name
		}
		return offenders[i].ID < offenders[j].ID
	})
	first := offenders[0]
	return &model.Issue{
		RuleID:   "STARTUP-001",
		Severity: model.SeverityMedium,
		Summary:  fmt.Sprintf("发现可疑启动项: %s", fallbackText(first.Name, first.ID)),
		Evidence: map[string]any{
			"count":     len(offenders),
			"offenders": offenders,
		},
		Impact:      "可疑启动项可能增加开机负担并带来潜在安全风险",
		Suggestion:  "确认启动项来源后禁用或移除可疑项",
		FixCommand:  "syskit startup disable " + first.ID + " --apply --yes",
		AutoFixable: true,
		Confidence:  82,
		Scope:       model.ScopeSystem,
	}
}

func checkLOG001(in DiagnoseInput) *model.Issue {
	logObs := in.Snapshots.Log
	if logObs == nil {
		return nil
	}
	errorRateThreshold := defaultLogErrorRate
	growthThreshold := defaultLogGrowthMBPerHour
	if logObs.ErrorRate < errorRateThreshold && logObs.GrowthMBPerHour < growthThreshold {
		return nil
	}
	return &model.Issue{
		RuleID:   "LOG-001",
		Severity: model.SeverityHigh,
		Summary:  fmt.Sprintf("日志错误率或增长异常（error_rate=%.2f%% growth=%.2fMB/h）", logObs.ErrorRate, logObs.GrowthMBPerHour),
		Evidence: map[string]any{
			"error_rate":         logObs.ErrorRate,
			"growth_mb_per_hour": logObs.GrowthMBPerHour,
			"error_lines":        logObs.ErrorLines,
			"total_lines":        logObs.TotalLines,
			"error_threshold":    errorRateThreshold,
			"growth_threshold":   growthThreshold,
		},
		Impact:      "错误日志持续增长会占用磁盘并掩盖真正故障",
		Suggestion:  "聚类错误关键字并优先修复高频异常",
		FixCommand:  "syskit log search error --since 24h",
		AutoFixable: false,
		Confidence:  84,
		Scope:       model.ScopeSystem,
	}
}

func baselineConnectionCount(moduleData map[string]any) int {
	if moduleData == nil {
		return 0
	}
	value, ok := moduleData["net_baseline_count"]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func topRemoteHosts(connections []ConnectionObservation, top int) []map[string]any {
	if top <= 0 {
		top = 5
	}
	counter := make(map[string]int, len(connections))
	for _, conn := range connections {
		host := strings.TrimSpace(conn.RemoteAddr)
		if host == "" {
			continue
		}
		counter[host]++
	}
	items := make([]map[string]any, 0, len(counter))
	for host, count := range counter {
		items = append(items, map[string]any{
			"host":  host,
			"count": count,
		})
	}
	sort.Slice(items, func(i int, j int) bool {
		left := items[i]["count"].(int)
		right := items[j]["count"].(int)
		if left != right {
			return left > right
		}
		return items[i]["host"].(string) < items[j]["host"].(string)
	})
	if len(items) > top {
		items = items[:top]
	}
	return items
}

func normalizedRequiredServices(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = normalizeServiceName(item)
		if item == "" {
			continue
		}
		if _, ok := set[item]; ok {
			continue
		}
		set[item] = struct{}{}
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func normalizeServiceName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, ".service")
	return name
}

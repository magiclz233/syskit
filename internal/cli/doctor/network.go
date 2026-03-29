package doctor

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"syskit/internal/cliutil"
	dnscollector "syskit/internal/collectors/dns"
	networkprobe "syskit/internal/collectors/networkprobe"
	"syskit/internal/domain/model"
	"syskit/internal/domain/rules"
	"syskit/internal/errs"

	"github.com/spf13/cobra"
)

const (
	defaultNetworkTarget       = "1.1.1.1"
	defaultNetworkProbeTimeout = 2 * time.Second
	defaultNetworkProbeCount   = 3
)

type networkOptions struct {
	target string
}

// newNetworkCommand 创建 `doctor network` 命令。
func newNetworkCommand() *cobra.Command {
	opts := &networkOptions{target: defaultNetworkTarget}
	cmd := &cobra.Command{
		Use:   "network",
		Short: "执行网络链路专项诊断",
		Long: "doctor network 用于执行 DNS→网关→目标链路的网络专项诊断，并统一输出风险项、跳过项和退出码。" +
			"\n\n该命令优先给出“不可达、抖动、丢包、路径异常”的根因线索，适合和 `net speed` 配合定位网络质量问题。",
		Example: "  syskit doctor network\n" +
			"  syskit doctor network --target 1.1.1.1\n" +
			"  syskit doctor network --target example.com --fail-on medium --format json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorNetwork(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.target, "target", defaultNetworkTarget, "诊断目标地址（域名或 IP）")
	return cmd
}

func runDoctorNetwork(cmd *cobra.Command, opts *networkOptions) error {
	startedAt := time.Now()
	target, dnsDomain, err := normalizeNetworkTarget(opts.target)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	issues := make([]model.Issue, 0, 4)
	skipped := make([]model.SkippedModule, 0, 3)
	warnings := make([]string, 0, 8)
	totalChecks := 2 // ping + traceroute
	skippedChecks := 0

	if dnsDomain != "" {
		totalChecks++
		resolveResult, benchResult, dnsErr := runNetworkDNSStage(ctx, dnsDomain)
		if dnsErr != nil {
			stageIssue, stageSkipped := classifyNetworkStageError("dns", dnsErr)
			if stageIssue != nil {
				issues = append(issues, *stageIssue)
			}
			if stageSkipped != nil {
				skipped = append(skipped, *stageSkipped)
				skippedChecks++
			}
			warnings = append(warnings, fmt.Sprintf("DNS 阶段失败: %s", errs.Message(dnsErr)))
		} else {
			warnings = append(warnings, fmt.Sprintf(
				"DNS 阶段完成: domain=%s records=%d success=%d/%d avg=%.2fms",
				dnsDomain,
				resolveResult.Count,
				benchResult.SuccessCount,
				benchResult.Count,
				benchResult.AvgMs,
			))
			if issue := analyzeDNSStage(dnsDomain, resolveResult, benchResult); issue != nil {
				issues = append(issues, *issue)
			}
		}
	} else {
		warnings = append(warnings, "目标为 IP，已跳过 DNS 阶段")
	}

	pingResult, pingErr := networkprobe.Ping(ctx, networkprobe.PingOptions{
		Target:   target,
		Count:    defaultNetworkProbeCount,
		Interval: 300 * time.Millisecond,
		Timeout:  defaultNetworkProbeTimeout,
		Size:     32,
	})
	if pingErr != nil {
		stageIssue, stageSkipped := classifyNetworkStageError("ping", pingErr)
		if stageIssue != nil {
			issues = append(issues, *stageIssue)
		}
		if stageSkipped != nil {
			skipped = append(skipped, *stageSkipped)
			skippedChecks++
		}
		warnings = append(warnings, fmt.Sprintf("Ping 阶段失败: %s", errs.Message(pingErr)))
	} else {
		warnings = append(warnings, fmt.Sprintf(
			"Ping 阶段完成: target=%s success=%d/%d loss=%.1f%% avg=%.2fms",
			target,
			pingResult.SuccessCount,
			pingResult.Count,
			pingResult.LossRate,
			pingResult.AvgLatencyMs,
		))
		if issue := analyzePingStage(target, pingResult); issue != nil {
			issues = append(issues, *issue)
		}
	}

	traceResult, traceErr := networkprobe.Traceroute(ctx, networkprobe.TracerouteOptions{
		Target:   target,
		MaxHops:  8,
		Timeout:  defaultNetworkProbeTimeout,
		Protocol: networkprobe.TraceProtocolICMP,
	})
	if traceErr != nil {
		stageIssue, stageSkipped := classifyNetworkStageError("traceroute", traceErr)
		if stageIssue != nil {
			issues = append(issues, *stageIssue)
		}
		if stageSkipped != nil {
			skipped = append(skipped, *stageSkipped)
			skippedChecks++
		}
		warnings = append(warnings, fmt.Sprintf("Traceroute 阶段失败: %s", errs.Message(traceErr)))
	} else {
		warnings = append(warnings, fmt.Sprintf(
			"Traceroute 阶段完成: target=%s reached=%t hops=%d",
			target,
			traceResult.Reached,
			traceResult.HopCount,
		))
		issues = append(issues, analyzeTracerouteStage(target, traceResult)...)
	}

	sortDoctorIssues(issues)
	warnings = uniqueSortedWarnings(warnings)

	coverage := coverageOf(totalChecks, skippedChecks)
	failOn := cliutil.ResolveStringFlag(cmd, "fail-on")
	score, level := rules.NewScorer().Score(issues, coverage)
	exitCode := rules.ResolveDoctorExitCode(issues, failOn)
	if exitCode == errs.ExitSuccess && len(skipped) > 0 {
		exitCode = errs.ExitWarning
	}

	resultData := &doctorOutput{
		Scope:         "module",
		Module:        "network",
		HealthScore:   score,
		HealthLevel:   level,
		Coverage:      coverage,
		FailOn:        strings.ToLower(strings.TrimSpace(failOn)),
		FailOnMatched: exitCode == errs.ExitFailOnMatched,
		Issues:        issues,
		Skipped:       skipped,
		Warnings:      warnings,
	}
	return renderDoctorResult(cmd, startedAt, resultData, exitCode, "网络专项诊断完成")
}

func normalizeNetworkTarget(raw string) (string, string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", "", errs.InvalidArgument("--target 不能为空")
	}
	if ip := net.ParseIP(target); ip != nil {
		return target, "", nil
	}

	// 用户可能输入了 host:port；doctor network 只关注主机链路，端口信息在这里会干扰 ping/traceroute。
	host, _, splitErr := net.SplitHostPort(target)
	if splitErr == nil && strings.TrimSpace(host) != "" {
		normalizedHost := strings.TrimSpace(host)
		return normalizedHost, normalizedHost, nil
	}
	return target, target, nil
}

func runNetworkDNSStage(ctx context.Context, domain string) (*dnscollector.ResolveResult, *dnscollector.BenchResult, error) {
	resolveResult, err := dnscollector.ResolveDomain(ctx, dnscollector.ResolveOptions{
		Domain:  domain,
		Type:    dnscollector.ResolveTypeA,
		Timeout: defaultNetworkProbeTimeout,
	})
	if err != nil {
		return nil, nil, err
	}
	benchResult, err := dnscollector.BenchDomain(ctx, dnscollector.BenchOptions{
		Domain:  domain,
		Type:    dnscollector.ResolveTypeA,
		Count:   defaultNetworkProbeCount,
		Timeout: defaultNetworkProbeTimeout,
	})
	if err != nil {
		return nil, nil, err
	}
	return resolveResult, benchResult, nil
}

func analyzeDNSStage(domain string, resolveResult *dnscollector.ResolveResult, benchResult *dnscollector.BenchResult) *model.Issue {
	if resolveResult == nil || benchResult == nil {
		return nil
	}
	if benchResult.SuccessCount == 0 {
		return &model.Issue{
			RuleID:      "NET-001",
			Severity:    model.SeverityHigh,
			Summary:     fmt.Sprintf("域名 %s DNS 解析不可用", domain),
			Evidence:    map[string]any{"domain": domain, "success_count": benchResult.SuccessCount, "failure_count": benchResult.FailureCount, "attempts": benchResult.Attempts},
			Impact:      "DNS 不可用会导致域名目标无法建立连接",
			Suggestion:  "检查本机 DNS 配置、上游解析服务和本地网络出口策略",
			FixCommand:  "syskit dns resolve " + domain,
			AutoFixable: false,
			Confidence:  92,
			Scope:       model.ScopeSystem,
		}
	}
	if benchResult.FailureCount > 0 {
		return &model.Issue{
			RuleID:      "NET-001",
			Severity:    model.SeverityMedium,
			Summary:     fmt.Sprintf("域名 %s DNS 解析存在波动（失败 %d/%d）", domain, benchResult.FailureCount, benchResult.Count),
			Evidence:    map[string]any{"domain": domain, "success_count": benchResult.SuccessCount, "failure_count": benchResult.FailureCount, "avg_ms": benchResult.AvgMs},
			Impact:      "间歇性解析失败会引发连接偶发超时",
			Suggestion:  "检查 DNS 服务可用性，并对关键域名设置稳定的解析策略",
			FixCommand:  "syskit dns bench " + domain,
			AutoFixable: false,
			Confidence:  86,
			Scope:       model.ScopeSystem,
		}
	}
	if benchResult.AvgMs >= 200 {
		return &model.Issue{
			RuleID:      "NET-001",
			Severity:    model.SeverityMedium,
			Summary:     fmt.Sprintf("域名 %s DNS 响应偏慢（平均 %.1fms）", domain, benchResult.AvgMs),
			Evidence:    map[string]any{"domain": domain, "avg_ms": benchResult.AvgMs, "max_ms": benchResult.MaxMs, "min_ms": benchResult.MinMs},
			Impact:      "DNS 查询慢会放大请求建立延迟，影响首包时间",
			Suggestion:  "更换低延迟 DNS，或排查链路抖动和本地解析缓存",
			FixCommand:  "syskit dns bench " + domain,
			AutoFixable: false,
			Confidence:  80,
			Scope:       model.ScopeSystem,
		}
	}
	return nil
}

func analyzePingStage(target string, pingResult *networkprobe.PingResult) *model.Issue {
	if pingResult == nil {
		return nil
	}
	if pingResult.SuccessCount == 0 {
		return &model.Issue{
			RuleID:      "NET-003",
			Severity:    model.SeverityCritical,
			Summary:     fmt.Sprintf("目标 %s 不可达（Ping 全部失败）", target),
			Evidence:    map[string]any{"target": target, "success_count": pingResult.SuccessCount, "failure_count": pingResult.FailureCount, "loss_rate": pingResult.LossRate},
			Impact:      "目标链路不可达会直接导致网络请求失败",
			Suggestion:  "检查本机路由、出口 ACL、防火墙策略和目标可达性",
			FixCommand:  "syskit ping " + target,
			AutoFixable: false,
			Confidence:  95,
			Scope:       model.ScopeSystem,
		}
	}
	if pingResult.LossRate >= 40 {
		return &model.Issue{
			RuleID:      "NET-003",
			Severity:    model.SeverityHigh,
			Summary:     fmt.Sprintf("目标 %s 丢包严重（%.1f%%）", target, pingResult.LossRate),
			Evidence:    map[string]any{"target": target, "loss_rate": pingResult.LossRate, "avg_latency_ms": pingResult.AvgLatencyMs, "attempts": pingResult.Attempts},
			Impact:      "高丢包会导致请求重试激增和连接不稳定",
			Suggestion:  "检查链路质量、网卡状态和中间网络设备错误计数",
			FixCommand:  "syskit ping " + target + " --count 10",
			AutoFixable: false,
			Confidence:  90,
			Scope:       model.ScopeSystem,
		}
	}
	if pingResult.AvgLatencyMs >= 200 {
		return &model.Issue{
			RuleID:      "NET-003",
			Severity:    model.SeverityMedium,
			Summary:     fmt.Sprintf("目标 %s 时延偏高（平均 %.1fms）", target, pingResult.AvgLatencyMs),
			Evidence:    map[string]any{"target": target, "avg_latency_ms": pingResult.AvgLatencyMs, "jitter_ms": pingResult.JitterMs, "loss_rate": pingResult.LossRate},
			Impact:      "高时延会放大接口调用耗时，影响交互体验",
			Suggestion:  "检查出口拥塞、跨区域链路和 QoS 限速策略",
			FixCommand:  "syskit traceroute " + target,
			AutoFixable: false,
			Confidence:  82,
			Scope:       model.ScopeSystem,
		}
	}
	return nil
}

func analyzeTracerouteStage(target string, traceResult *networkprobe.TracerouteResult) []model.Issue {
	if traceResult == nil {
		return nil
	}
	issues := make([]model.Issue, 0, 2)

	if len(traceResult.Hops) > 0 {
		firstHop := traceResult.Hops[0]
		if firstHop.Timeout {
			issues = append(issues, model.Issue{
				RuleID:      "NET-002",
				Severity:    model.SeverityMedium,
				Summary:     "网关路径探测超时，首跳未响应",
				Evidence:    map[string]any{"target": target, "first_hop": firstHop, "hop_count": traceResult.HopCount},
				Impact:      "网关路径异常会导致到公网目标的链路不稳定",
				Suggestion:  "检查默认网关、局域网交换链路和本机网络配置",
				FixCommand:  "syskit traceroute " + target,
				AutoFixable: false,
				Confidence:  78,
				Scope:       model.ScopeSystem,
			})
		} else if firstHop.AvgLatencyMs >= 150 {
			issues = append(issues, model.Issue{
				RuleID:      "NET-002",
				Severity:    model.SeverityMedium,
				Summary:     fmt.Sprintf("网关路径时延偏高（首跳 %.1fms）", firstHop.AvgLatencyMs),
				Evidence:    map[string]any{"target": target, "first_hop": firstHop, "hop_count": traceResult.HopCount},
				Impact:      "首跳时延异常通常意味着本地网络链路拥塞或设备异常",
				Suggestion:  "检查网关负载、局域网带宽占用和本机网卡状态",
				FixCommand:  "syskit traceroute " + target,
				AutoFixable: false,
				Confidence:  76,
				Scope:       model.ScopeSystem,
			})
		}
	}

	if !traceResult.Reached {
		issues = append(issues, model.Issue{
			RuleID:      "NET-004",
			Severity:    model.SeverityHigh,
			Summary:     fmt.Sprintf("到目标 %s 的路由未在最大跳数内闭合", target),
			Evidence:    map[string]any{"target": target, "max_hops": traceResult.MaxHops, "hop_count": traceResult.HopCount, "reached": traceResult.Reached},
			Impact:      "路由不闭合会导致跨网络访问失败或大面积超时",
			Suggestion:  "结合网络策略检查中间跳点是否丢弃 ICMP/TCP 探测包",
			FixCommand:  "syskit traceroute " + target + " --max-hops 30",
			AutoFixable: false,
			Confidence:  88,
			Scope:       model.ScopeSystem,
		})
	}
	return issues
}

func classifyNetworkStageError(stage string, err error) (*model.Issue, *model.SkippedModule) {
	if err == nil {
		return nil, nil
	}

	moduleName := "network-" + strings.ToLower(strings.TrimSpace(stage))
	if degraded := rules.ClassifyModuleError(moduleName, err); degraded != nil {
		skipped := degraded.ToSkippedModule()
		return nil, &skipped
	}
	if errs.ErrorCode(err) == errs.CodeDependencyMissing {
		return nil, &model.SkippedModule{
			Module:             moduleName,
			Reason:             model.SkipReasonUnsupported,
			RequiredPermission: "",
			Impact:             fmt.Sprintf("%s阶段依赖命令缺失，未参与本轮诊断", networkStageName(stage)),
			Suggestion:         fallbackSuggestion(errs.Suggestion(err), "请确认系统网络工具在 PATH 中可执行"),
		}
	}
	return buildNetworkStageIssue(stage, err), nil
}

func buildNetworkStageIssue(stage string, err error) *model.Issue {
	return &model.Issue{
		RuleID:      networkStageRuleID(stage),
		Severity:    model.SeverityHigh,
		Summary:     fmt.Sprintf("%s阶段执行失败: %s", networkStageName(stage), errs.Message(err)),
		Evidence:    map[string]any{"stage": stage, "error_code": errs.ErrorCode(err), "message": errs.Message(err)},
		Impact:      "网络链路诊断不完整，可能遗漏关键根因",
		Suggestion:  fallbackSuggestion(errs.Suggestion(err), "请修复该阶段错误后重试"),
		FixCommand:  "syskit doctor network --target " + defaultNetworkTarget,
		AutoFixable: false,
		Confidence:  75,
		Scope:       model.ScopeSystem,
	}
}

func networkStageRuleID(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "dns":
		return "NET-001"
	case "ping":
		return "NET-003"
	case "traceroute":
		return "NET-004"
	default:
		return "NET-004"
	}
}

func networkStageName(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "dns":
		return "DNS"
	case "ping":
		return "Ping"
	case "traceroute":
		return "Traceroute"
	default:
		return "网络"
	}
}

func sortDoctorIssues(issues []model.Issue) {
	sort.Slice(issues, func(i int, j int) bool {
		left := model.SeverityRank(issues[i].Severity)
		right := model.SeverityRank(issues[j].Severity)
		if left != right {
			return left > right
		}
		if issues[i].RuleID != issues[j].RuleID {
			return issues[i].RuleID < issues[j].RuleID
		}
		return issues[i].Summary < issues[j].Summary
	})
}

func uniqueSortedWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(warnings))
	for _, warning := range warnings {
		normalized := strings.TrimSpace(warning)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	items := make([]string, 0, len(set))
	for item := range set {
		items = append(items, item)
	}
	sort.Strings(items)
	return items
}

func fallbackSuggestion(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}

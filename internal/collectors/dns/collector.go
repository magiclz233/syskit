package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"syskit/internal/errs"
	"time"
)

const (
	defaultBenchCount    = 5
	defaultLookupTimeout = time.Second
)

// ResolveDomain 执行单次 DNS 解析。
func ResolveDomain(ctx context.Context, opts ResolveOptions) (*ResolveResult, error) {
	cfg, resolver, err := normalizeResolve(ctx, opts)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	records, err := lookupRecords(cfg.ctx, resolver, cfg.Domain, cfg.Type)
	if err != nil {
		return nil, mapLookupError(cfg.Domain, err)
	}
	records = uniqueSortedRecords(records)
	if len(records) == 0 {
		return nil, errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, fmt.Sprintf("域名 %s 未返回 %s 记录", cfg.Domain, cfg.Type))
	}

	return &ResolveResult{
		Domain:     cfg.Domain,
		QueryType:  string(cfg.Type),
		DNSServer:  cfg.DNSServer,
		DurationMs: durationMs(time.Since(startedAt)),
		Count:      len(records),
		Records:    records,
	}, nil
}

// BenchDomain 执行多次 DNS 查询并输出时延统计。
func BenchDomain(ctx context.Context, opts BenchOptions) (*BenchResult, error) {
	cfg, resolver, err := normalizeBench(ctx, opts)
	if err != nil {
		return nil, err
	}

	result := &BenchResult{
		Domain:    cfg.Domain,
		QueryType: string(cfg.Type),
		DNSServer: cfg.DNSServer,
		Count:     cfg.Count,
		Attempts:  make([]BenchAttempt, 0, cfg.Count),
	}
	durations := make([]float64, 0, cfg.Count)

	for i := 1; i <= cfg.Count; i++ {
		if err := cfg.ctx.Err(); err != nil {
			if timeoutErr := mapTimeoutError(err); timeoutErr != nil {
				return nil, timeoutErr
			}
			return nil, errs.ExecutionFailed("DNS bench 被取消", err)
		}

		attempt := BenchAttempt{Seq: i}
		startedAt := time.Now()
		records, lookupErr := lookupRecords(cfg.ctx, resolver, cfg.Domain, cfg.Type)
		attempt.DurationMs = durationMs(time.Since(startedAt))
		if lookupErr != nil {
			attempt.Success = false
			attempt.Error = normalizeLookupError(lookupErr)
			result.FailureCount++
		} else {
			records = uniqueSortedRecords(records)
			if len(records) == 0 {
				attempt.Success = false
				attempt.Error = "no_records"
				result.FailureCount++
			} else {
				attempt.Success = true
				attempt.RecordCnt = len(records)
				result.SuccessCount++
				durations = append(durations, attempt.DurationMs)
			}
		}
		result.Attempts = append(result.Attempts, attempt)
	}

	if len(durations) > 0 {
		sort.Float64s(durations)
		result.MinMs = durations[0]
		result.MaxMs = durations[len(durations)-1]
		total := 0.0
		for _, value := range durations {
			total += value
		}
		result.AvgMs = total / float64(len(durations))
	}

	return result, nil
}

type preparedResolve struct {
	ctx       context.Context
	Domain    string
	Type      ResolveType
	DNSServer string
	Timeout   time.Duration
}

type preparedBench struct {
	ctx       context.Context
	Domain    string
	Type      ResolveType
	DNSServer string
	Count     int
	Timeout   time.Duration
}

func normalizeResolve(ctx context.Context, opts ResolveOptions) (*preparedResolve, *net.Resolver, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	domain := strings.TrimSpace(opts.Domain)
	if domain == "" {
		return nil, nil, errs.InvalidArgument("domain 不能为空")
	}
	typ := opts.Type
	if typ == "" {
		typ = ResolveTypeA
	}
	dnsServer, err := normalizeDNSServer(opts.DNSServer)
	if err != nil {
		return nil, nil, err
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultLookupTimeout
	}
	resolver := buildResolver(dnsServer, timeout)
	return &preparedResolve{ctx: ctx, Domain: domain, Type: typ, DNSServer: dnsServer, Timeout: timeout}, resolver, nil
}

func normalizeBench(ctx context.Context, opts BenchOptions) (*preparedBench, *net.Resolver, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	domain := strings.TrimSpace(opts.Domain)
	if domain == "" {
		return nil, nil, errs.InvalidArgument("domain 不能为空")
	}
	count := opts.Count
	if count <= 0 {
		count = defaultBenchCount
	}
	typ := opts.Type
	if typ == "" {
		typ = ResolveTypeA
	}
	dnsServer, err := normalizeDNSServer(opts.DNSServer)
	if err != nil {
		return nil, nil, err
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultLookupTimeout
	}
	resolver := buildResolver(dnsServer, timeout)
	return &preparedBench{ctx: ctx, Domain: domain, Type: typ, DNSServer: dnsServer, Count: count, Timeout: timeout}, resolver, nil
}

func buildResolver(dnsServer string, timeout time.Duration) *net.Resolver {
	if strings.TrimSpace(dnsServer) == "" {
		return net.DefaultResolver
	}
	dialer := &net.Dialer{Timeout: timeout}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "udp", dnsServer)
		},
	}
}

func lookupRecords(ctx context.Context, resolver *net.Resolver, domain string, typ ResolveType) ([]ResolveRecord, error) {
	resolverRef := resolver
	if resolverRef == nil {
		resolverRef = net.DefaultResolver
	}

	switch typ {
	case ResolveTypeA:
		ips, err := resolverRef.LookupIP(ctx, "ip4", domain)
		if err != nil {
			return nil, err
		}
		records := make([]ResolveRecord, 0, len(ips))
		for _, ip := range ips {
			records = append(records, ResolveRecord{Type: string(ResolveTypeA), Value: ip.String()})
		}
		return records, nil
	case ResolveTypeAAAA:
		ips, err := resolverRef.LookupIP(ctx, "ip6", domain)
		if err != nil {
			return nil, err
		}
		records := make([]ResolveRecord, 0, len(ips))
		for _, ip := range ips {
			records = append(records, ResolveRecord{Type: string(ResolveTypeAAAA), Value: ip.String()})
		}
		return records, nil
	case ResolveTypeCNAME:
		cname, err := resolverRef.LookupCNAME(ctx, domain)
		if err != nil {
			return nil, err
		}
		return []ResolveRecord{{Type: string(ResolveTypeCNAME), Value: strings.TrimSpace(cname)}}, nil
	case ResolveTypeMX:
		mxList, err := resolverRef.LookupMX(ctx, domain)
		if err != nil {
			return nil, err
		}
		records := make([]ResolveRecord, 0, len(mxList))
		for _, item := range mxList {
			records = append(records, ResolveRecord{Type: string(ResolveTypeMX), Value: fmt.Sprintf("%d %s", item.Pref, strings.TrimSpace(item.Host))})
		}
		return records, nil
	case ResolveTypeNS:
		nsList, err := resolverRef.LookupNS(ctx, domain)
		if err != nil {
			return nil, err
		}
		records := make([]ResolveRecord, 0, len(nsList))
		for _, item := range nsList {
			records = append(records, ResolveRecord{Type: string(ResolveTypeNS), Value: strings.TrimSpace(item.Host)})
		}
		return records, nil
	case ResolveTypeTXT:
		txtList, err := resolverRef.LookupTXT(ctx, domain)
		if err != nil {
			return nil, err
		}
		records := make([]ResolveRecord, 0, len(txtList))
		for _, item := range txtList {
			records = append(records, ResolveRecord{Type: string(ResolveTypeTXT), Value: strings.TrimSpace(item)})
		}
		return records, nil
	default:
		return nil, errs.InvalidArgument(fmt.Sprintf("不支持的记录类型: %s", typ))
	}
}

func mapLookupError(domain string, err error) error {
	if err == nil {
		return nil
	}
	if timeoutErr := mapTimeoutError(err); timeoutErr != nil {
		return timeoutErr
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, fmt.Sprintf("域名 %s 不存在或无记录", domain))
		}
		if dnsErr.IsTimeout {
			return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "DNS 查询超时", "请调大 --timeout 或更换 --dns 后重试")
		}
	}
	return errs.ExecutionFailed("DNS 查询失败", err)
}

func mapTimeoutError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "DNS 查询超时", "请调大 --timeout 或更换 --dns 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
	}
	return nil
}

func normalizeLookupError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return "not_found"
		}
		if dnsErr.IsTimeout {
			return "timeout"
		}
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return "unknown_error"
	}
	return message
}

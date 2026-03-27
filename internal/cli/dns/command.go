// Package dns 负责 DNS 诊断命令。
package dns

import (
	"fmt"
	"strings"
	"time"

	"syskit/internal/cliutil"
	dnscollector "syskit/internal/collectors/dns"
	"syskit/internal/errs"
	"syskit/internal/output"

	"github.com/spf13/cobra"
)

type resolveOptions struct {
	typ string
	dns string
}

type benchOptions struct {
	typ   string
	dns   string
	count int
}

// NewCommand 创建 `dns` 顶层命令。
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "DNS 诊断命令",
		Long:  "dns 命令组用于 DNS 解析与响应性能测试，支持指定记录类型和 DNS 服务器。",
		Example: "  syskit dns resolve example.com\n" +
			"  syskit dns resolve example.com --type MX --dns 8.8.8.8\n" +
			"  syskit dns bench example.com --count 5 --dns 1.1.1.1",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newResolveCommand(), newBenchCommand())
	return cmd
}

func newResolveCommand() *cobra.Command {
	opts := &resolveOptions{}
	cmd := &cobra.Command{
		Use:   "resolve <domain>",
		Short: "解析指定域名",
		Long: "dns resolve 会按指定记录类型解析域名，默认解析 A 记录。" +
			"\n\n可通过 --dns 指定上游 DNS 服务器，未指定时使用系统默认解析器。",
		Example: "  syskit dns resolve localhost\n" +
			"  syskit dns resolve example.com --type AAAA\n" +
			"  syskit dns resolve example.com --type MX --dns 8.8.8.8 --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResolve(cmd, args[0], opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.typ, "type", "A", "记录类型: A/AAAA/CNAME/MX/NS/TXT")
	flags.StringVar(&opts.dns, "dns", "", "指定 DNS 服务器（host 或 host:port）")
	return cmd
}

func newBenchCommand() *cobra.Command {
	opts := &benchOptions{count: 5}
	cmd := &cobra.Command{
		Use:   "bench <domain>",
		Short: "测试 DNS 响应性能",
		Long: "dns bench 会对指定域名执行多次 DNS 查询，输出成功率与时延统计。" +
			"\n\n默认查询 A 记录并执行 5 次，可通过 --type、--count、--dns 调整。",
		Example: "  syskit dns bench localhost\n" +
			"  syskit dns bench example.com --count 10 --type AAAA\n" +
			"  syskit dns bench example.com --dns 1.1.1.1 --timeout 2s --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBench(cmd, args[0], opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.typ, "type", "A", "记录类型: A/AAAA/CNAME/MX/NS/TXT")
	flags.StringVar(&opts.dns, "dns", "", "指定 DNS 服务器（host 或 host:port）")
	flags.IntVar(&opts.count, "count", 5, "探测次数")
	return cmd
}

func runResolve(cmd *cobra.Command, domain string, opts *resolveOptions) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return errs.InvalidArgument("domain 不能为空")
	}
	typ, err := dnscollector.ParseResolveType(opts.typ)
	if err != nil {
		return err
	}
	timeout, err := resolveLookupTimeout(cmd)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := dnscollector.ResolveDomain(ctx, dnscollector.ResolveOptions{
		Domain:    domain,
		Type:      typ,
		DNSServer: opts.dns,
		Timeout:   timeout,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("DNS 解析完成，共 %d 条记录", resultData.Count)
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newResolvePresenter(resultData))
}

func runBench(cmd *cobra.Command, domain string, opts *benchOptions) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return errs.InvalidArgument("domain 不能为空")
	}
	if opts.count <= 0 {
		return errs.InvalidArgument("--count 必须大于 0")
	}
	typ, err := dnscollector.ParseResolveType(opts.typ)
	if err != nil {
		return err
	}
	timeout, err := resolveLookupTimeout(cmd)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := dnscollector.BenchDomain(ctx, dnscollector.BenchOptions{
		Domain:    domain,
		Type:      typ,
		DNSServer: opts.dns,
		Count:     opts.count,
		Timeout:   timeout,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("DNS bench 完成，成功 %d/%d", resultData.SuccessCount, resultData.Count)
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newBenchPresenter(resultData))
}

func resolveLookupTimeout(cmd *cobra.Command) (time.Duration, error) {
	raw := strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "timeout"))
	if raw == "" || raw == "0" || raw == "0s" {
		return time.Second, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --timeout: %s", raw))
	}
	if parsed <= 0 {
		return time.Second, nil
	}
	return parsed, nil
}

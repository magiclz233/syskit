// Package port 负责端口查询和端口释放命令。
package port

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syskit/internal/audit"
	"syskit/internal/cliutil"
	portcollector "syskit/internal/collectors/port"
	"syskit/internal/config"
	"syskit/internal/errs"
	"syskit/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type queryOptions struct {
	detail bool
}

type listOptions struct {
	by       string
	protocol string
	listen   string
	detail   bool
}

type pingOptions struct {
	count    int
	interval time.Duration
}

type scanOptions struct {
	portRange string
	mode      string
}

type killOptions struct {
	force    bool
	killTree bool
}

const fullScanDefaultUpperPort = 1024

var quickScanDefaultPorts = []int{
	21, 22, 25, 53, 80, 110, 143, 443, 445, 587, 993, 995, 1433, 1521, 3306, 3389, 5432, 6379, 8080, 8443,
}

// killOutputData 是 `port kill` 的统一输出结构。
type killOutputData struct {
	Mode   string                    `json:"mode"`
	Apply  bool                      `json:"apply"`
	Plan   *portcollector.KillPlan   `json:"plan"`
	Result *portcollector.KillResult `json:"result,omitempty"`
}

// NewCommand 创建 `port` 顶层命令。
func NewCommand() *cobra.Command {
	queryOpts := &queryOptions{}

	cmd := &cobra.Command{
		Use:   "port <port[,port]|range>",
		Short: "端口查询与释放",
		Long: "port 提供端口占用查询、监听列表、TCP 可达性测试、端口扫描和端口释放能力。" +
			"\n\n`port kill` 属于写操作，正式执行前必须同时传入 `--apply --yes`。",
		Example: "  syskit port 8080\n" +
			"  syskit port 80,443,8080 --detail\n" +
			"  syskit port list --protocol tcp --by pid\n" +
			"  syskit port ping 127.0.0.1 8080 --count 3\n" +
			"  syskit port scan 127.0.0.1 --port 22,80,443",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runQuery(cmd, args[0], queryOpts)
		},
	}

	cmd.Flags().BoolVar(&queryOpts.detail, "detail", false, "显示用户、命令行等详细字段")
	cmd.AddCommand(
		newListCommand(),
		newPingCommand(),
		newScanCommand(),
		newKillCommand(),
	)
	return cmd
}

func newListCommand() *cobra.Command {
	opts := &listOptions{
		by: "port",
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "查看监听端口列表",
		Long:  "port list 用于查看当前主机的监听端口，并支持按 PID、协议和监听地址过滤。",
		Example: "  syskit port list\n" +
			"  syskit port list --protocol tcp --listen 127.0.0.1\n" +
			"  syskit port list --by pid --detail",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.by, "by", "port", "排序维度: port/pid")
	flags.StringVar(&opts.protocol, "protocol", "", "协议过滤: tcp/udp")
	flags.StringVar(&opts.listen, "listen", "", "监听地址过滤（模糊匹配）")
	flags.BoolVar(&opts.detail, "detail", false, "显示用户、命令行等详细字段")
	return cmd
}

func newPingCommand() *cobra.Command {
	opts := &pingOptions{
		count:    4,
		interval: 200 * time.Millisecond,
	}

	cmd := &cobra.Command{
		Use:   "ping <target> <port>",
		Short: "执行 TCP 端口可达性测试",
		Long: "port ping 会对目标 TCP 端口执行多次建连探测，输出成功率与时延统计。" +
			"\n\n`--timeout` 复用全局超时参数，同时作为单次探测超时；未设置时单次探测默认 1s。",
		Example: "  syskit port ping 127.0.0.1 8080\n" +
			"  syskit port ping db.internal 5432 --count 6 --interval 300ms\n" +
			"  syskit port ping 10.0.0.12 22 --timeout 2s --format json",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPing(cmd, args[0], args[1], opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.count, "count", 4, "探测次数")
	flags.DurationVar(&opts.interval, "interval", 200*time.Millisecond, "探测间隔")
	return cmd
}

func newScanCommand() *cobra.Command {
	opts := &scanOptions{
		mode: "quick",
	}
	cmd := &cobra.Command{
		Use:   "scan <target>",
		Short: "扫描目标开放端口",
		Long: "port scan 对目标主机做 TCP 端口探测，默认 quick 模式扫描常见端口集合。" +
			"\n\n`--mode full` 且未指定 `--port` 时，默认扫描 `1-1024`；`--timeout` 复用全局超时并作为单次探测超时。",
		Example: "  syskit port scan 127.0.0.1\n" +
			"  syskit port scan 10.0.0.12 --port 22,80,443,8080\n" +
			"  syskit port scan db.internal --mode full --timeout 400ms --format json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(cmd, args[0], opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.portRange, "port", "", "扫描端口表达式，例如 22,80,443,8080-8090")
	flags.StringVar(&opts.mode, "mode", "quick", "扫描模式: quick/full")
	return cmd
}

func newKillCommand() *cobra.Command {
	opts := &killOptions{}
	cmd := &cobra.Command{
		Use:   "kill <port>",
		Short: "释放指定端口",
		Long: "port kill 会先发现端口占用进程并生成释放计划，默认只做 dry-run。" +
			"\n\n若要真实执行，必须显式传入 `--apply --yes`；执行结果会写入审计日志。",
		Example: "  syskit port kill 8080\n" +
			"  syskit port kill 8080 --force\n" +
			"  syskit port kill 8080 --apply --yes",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKill(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.force, "force", false, "强制终止占用进程")
	flags.BoolVar(&opts.killTree, "kill-tree", false, "连同子进程一起终止")
	return cmd
}

func runQuery(cmd *cobra.Command, expression string, opts *queryOptions) error {
	startedAt := time.Now()
	ports, err := portcollector.ParsePortExpression(expression)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := portcollector.QueryPorts(ctx, ports, opts.detail)
	if err != nil {
		return err
	}

	msg := "端口查询完成"
	if len(resultData.Entries) == 0 {
		msg = "未发现端口占用"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newQueryPresenter(resultData, opts.detail))
}

func runList(cmd *cobra.Command, opts *listOptions) error {
	startedAt := time.Now()
	by, err := portcollector.ParseSortBy(opts.by)
	if err != nil {
		return err
	}
	protocol, err := portcollector.ParseProtocol(opts.protocol)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := portcollector.ListPorts(ctx, portcollector.ListOptions{
		By:       by,
		Protocol: protocol,
		Listen:   opts.listen,
	}, opts.detail)
	if err != nil {
		return err
	}

	result := output.NewSuccessResult("监听端口列表采集完成", resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newListPresenter(resultData, opts.detail))
}

func runPing(cmd *cobra.Command, target string, portRaw string, opts *pingOptions) error {
	if opts.count <= 0 {
		return errs.InvalidArgument("--count 必须大于 0")
	}
	if opts.interval < 0 {
		return errs.InvalidArgument("--interval 不能小于 0")
	}

	port, err := parseSinglePort(portRaw)
	if err != nil {
		return err
	}
	probeTimeout, err := resolveProbeTimeout(cmd, time.Second)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := portcollector.PingPort(ctx, portcollector.PingOptions{
		Target:   target,
		Port:     port,
		Count:    opts.count,
		Timeout:  probeTimeout,
		Interval: opts.interval,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("TCP 可达性测试完成（成功 %d/%d）", resultData.SuccessCount, resultData.Count)
	if resultData.SuccessCount == 0 {
		msg = "TCP 可达性测试完成，未命中可用连接"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newPingPresenter(resultData))
}

func runScan(cmd *cobra.Command, target string, opts *scanOptions) error {
	mode, err := portcollector.ParseScanMode(opts.mode)
	if err != nil {
		return err
	}
	ports, warnings, err := resolveScanPorts(mode, opts.portRange)
	if err != nil {
		return err
	}
	probeTimeout, err := resolveProbeTimeout(cmd, 500*time.Millisecond)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	resultData, err := portcollector.ScanPorts(ctx, portcollector.ScanOptions{
		Target:  target,
		Mode:    mode,
		Ports:   ports,
		Timeout: probeTimeout,
	})
	if err != nil {
		return err
	}
	resultData.Warnings = append(resultData.Warnings, warnings...)

	msg := fmt.Sprintf("端口扫描完成，发现 %d 个开放端口", resultData.OpenCount)
	if resultData.OpenCount == 0 {
		msg = "端口扫描完成，未发现开放端口"
	}
	result := output.NewSuccessResult(msg, resultData, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newScanPresenter(resultData))
}

func runKill(cmd *cobra.Command, portRaw string, opts *killOptions) error {
	startedAt := time.Now()
	port, err := parseSinglePort(portRaw)
	if err != nil {
		return err
	}

	ctx, cancel, err := cliutil.CommandContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	plan, err := portcollector.BuildKillPlan(ctx, portcollector.KillOptions{
		Port:     port,
		Force:    opts.force,
		KillTree: opts.killTree,
	})
	if err != nil {
		return err
	}

	apply := cliutil.ResolveBoolFlag(cmd, "apply")
	yes := cliutil.ResolveBoolFlag(cmd, "yes")
	if apply && !yes {
		return errs.InvalidArgument("真实执行 port kill 需要同时传入 --yes")
	}

	if !apply {
		data := &killOutputData{
			Mode:  "dry-run",
			Apply: false,
			Plan:  plan,
		}
		result := output.NewSuccessResult("已生成端口释放计划（dry-run）", data, startedAt)
		return cliutil.RenderCommandResult(cmd, result, newKillPresenter(data))
	}

	execResult, err := portcollector.ExecuteKillPlan(ctx, plan)
	if err != nil {
		auditErr := writeAuditEvent(cmd, ctx, audit.Event{
			Command:    cmd.CommandPath(),
			Action:     "port.kill",
			Target:     fmt.Sprintf("port:%d", port),
			Before:     plan,
			Result:     "failed",
			ErrorMsg:   errs.Message(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Metadata: map[string]any{
				"apply":     true,
				"force":     opts.force,
				"kill_tree": opts.killTree,
			},
		})
		if auditErr != nil {
			return errs.ExecutionFailed(
				fmt.Sprintf("port kill 执行失败且审计写入失败: %s", errs.Message(err)),
				auditErr,
			)
		}
		return err
	}
	data := &killOutputData{
		Mode:   "apply",
		Apply:  true,
		Plan:   plan,
		Result: execResult,
	}

	msg := fmt.Sprintf("端口 %d 释放执行完成", port)
	if !execResult.Released {
		msg = fmt.Sprintf("端口 %d 释放执行完成，但仍存在占用", port)
	}
	if err := writeAuditEvent(cmd, ctx, audit.Event{
		Command:    cmd.CommandPath(),
		Action:     "port.kill",
		Target:     fmt.Sprintf("port:%d", port),
		Before:     plan,
		After:      execResult,
		Result:     "success",
		DurationMs: time.Since(startedAt).Milliseconds(),
		Metadata: map[string]any{
			"apply":     true,
			"force":     opts.force,
			"kill_tree": opts.killTree,
			"released":  execResult.Released,
		},
	}); err != nil {
		return err
	}

	result := output.NewSuccessResult(msg, data, startedAt)
	return cliutil.RenderCommandResult(cmd, result, newKillPresenter(data))
}

func parseSinglePort(raw string) (int, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, errs.InvalidArgument("端口不能为空")
	}
	value, err := strconv.Atoi(text)
	if err != nil || value <= 0 || value > 65535 {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效端口: %s", raw))
	}
	return value, nil
}

func resolveProbeTimeout(cmd *cobra.Command, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "timeout"))
	if raw == "" || raw == "0" || raw == "0s" {
		return fallback, nil
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return 0, errs.InvalidArgument(fmt.Sprintf("无效的 --timeout: %s", raw))
	}
	if timeout <= 0 {
		return fallback, nil
	}
	return timeout, nil
}

func resolveScanPorts(mode portcollector.ScanMode, rawPortRange string) ([]int, []string, error) {
	rawPortRange = strings.TrimSpace(rawPortRange)
	if rawPortRange != "" {
		ports, err := portcollector.ParsePortExpression(rawPortRange)
		if err != nil {
			return nil, nil, err
		}
		return ports, nil, nil
	}

	switch mode {
	case portcollector.ScanModeFull:
		ports := make([]int, 0, fullScanDefaultUpperPort)
		for port := 1; port <= fullScanDefaultUpperPort; port++ {
			ports = append(ports, port)
		}
		return ports, []string{
			fmt.Sprintf("未指定 --port，full 模式默认扫描 1-%d", fullScanDefaultUpperPort),
		}, nil
	default:
		ports := append([]int(nil), quickScanDefaultPorts...)
		return ports, []string{
			"未指定 --port，quick 模式使用内置常见端口集合",
		}, nil
	}
}

func loadRuntimeConfig(cmd *cobra.Command) (*config.Config, error) {
	loadResult, err := config.Load(config.LoadOptions{
		ExplicitPath: strings.TrimSpace(cliutil.ResolveStringFlag(cmd, "config")),
	})
	if err != nil {
		return nil, err
	}
	return loadResult.Config, nil
}

func writeAuditEvent(cmd *cobra.Command, ctx context.Context, event audit.Event) error {
	cfg, err := loadRuntimeConfig(cmd)
	if err != nil {
		return err
	}
	logger, err := audit.NewLogger(cfg.Storage.DataDir)
	if err != nil {
		return err
	}
	return logger.Log(ctx, event)
}

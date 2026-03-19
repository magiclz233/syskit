package cli

import (
	"syskit/internal/cliutil"

	"github.com/spf13/cobra"
)

// registerPendingCommands 把 CLI 规范里已预留但尚未实现的命令注册到帮助树中，
// 这样帮助输出、契约测试和文档可以共享同一套正式命令面。
func registerPendingCommands(rootCmd *cobra.Command) {
	if rootCmd == nil {
		return
	}

	doctorCmd := findCommand(rootCmd, "doctor")
	if doctorCmd != nil {
		doctorCmd.AddCommand(
			cliutil.NewPendingCommand("network", "执行网络链路专项诊断"),
			cliutil.NewPendingCommand("disk-full", "执行磁盘爆满场景诊断"),
			cliutil.NewPendingCommand("slowness", "执行系统卡顿场景诊断"),
		)
	}

	portCmd := findCommand(rootCmd, "port")
	if portCmd != nil {
		portCmd.AddCommand(
			cliutil.NewPendingCommand("ping <target> <port>", "执行 TCP 端口可达性测试"),
			cliutil.NewPendingCommand("scan <target>", "扫描目标开放端口"),
		)
	}

	cpuCmd := findCommand(rootCmd, "cpu")
	if cpuCmd != nil {
		cpuCmd.AddCommand(
			cliutil.NewPendingCommand("burst", "捕捉突发高 CPU 进程"),
			cliutil.NewPendingCommand("watch", "持续监控 CPU 使用情况"),
		)
	}

	memCmd := findCommand(rootCmd, "mem")
	if memCmd != nil {
		memCmd.AddCommand(
			cliutil.NewPendingCommand("leak <pid>", "监控进程内存泄漏趋势"),
			cliutil.NewPendingCommand("watch", "持续监控内存使用情况"),
		)
	}

	fixCmd := findCommand(rootCmd, "fix")
	if fixCmd != nil {
		fixCmd.AddCommand(
			cliutil.NewPendingCommand("run <script>", "执行内置或自定义修复剧本"),
		)
	}

	rootCmd.AddCommand(
		newPendingGroupCommand(
			"file",
			"文件治理命令",
			"file 命令组已在 CLI 规范中保留，用于后续重复文件、归档和清理治理能力。",
			cliutil.NewPendingCommand("dup <path>", "检测重复文件"),
			cliutil.NewPendingCommand("dedup <path>", "清理重复文件"),
			cliutil.NewPendingCommand("archive <path>", "归档旧日志或历史文件"),
			cliutil.NewPendingCommand("empty <path>", "清理空目录"),
		),
		newPendingGroupCommand(
			"net",
			"网络诊断命令",
			"net 命令组已在 CLI 规范中保留，用于连接审计、监听检查和带宽测速。",
			cliutil.NewPendingCommand("conn", "审计当前网络连接"),
			cliutil.NewPendingCommand("listen", "查看监听端口和地址"),
			cliutil.NewPendingCommand("speed", "执行带宽测速"),
		),
		newPendingGroupCommand(
			"dns",
			"DNS 诊断命令",
			"dns 命令组已在 CLI 规范中保留，用于 DNS 解析和性能测试。",
			cliutil.NewPendingCommand("resolve <domain>", "解析指定域名"),
			cliutil.NewPendingCommand("bench <domain>", "测试 DNS 响应性能"),
		),
		cliutil.NewPendingCommand("ping <target>", "执行 ICMP Ping 测试"),
		cliutil.NewPendingCommand("traceroute <target>", "执行路由跟踪"),
		newPendingGroupCommand(
			"service",
			"服务管理命令",
			"service 命令组已在 CLI 规范中保留，用于跨平台服务查看与运维。",
			cliutil.NewPendingCommand("list", "列出系统服务"),
			cliutil.NewPendingCommand("check <name>", "检查服务健康状态"),
			cliutil.NewPendingCommand("start <name>", "启动指定服务"),
			cliutil.NewPendingCommand("stop <name>", "停止指定服务"),
			cliutil.NewPendingCommand("restart <name>", "重启指定服务"),
			cliutil.NewPendingCommand("enable <name>", "启用服务开机自启"),
			cliutil.NewPendingCommand("disable <name>", "禁用服务开机自启"),
		),
		newPendingGroupCommand(
			"startup",
			"启动项管理命令",
			"startup 命令组已在 CLI 规范中保留，用于查看和管理系统启动项。",
			cliutil.NewPendingCommand("list", "列出启动项"),
			cliutil.NewPendingCommand("enable <id>", "启用指定启动项"),
			cliutil.NewPendingCommand("disable <id>", "禁用指定启动项"),
		),
		newPendingLogCommand(),
		newPendingGroupCommand(
			"monitor",
			"持续监控命令",
			"monitor 命令组已在 CLI 规范中保留，用于后续持续监控与告警能力。",
			cliutil.NewPendingCommand("all", "持续监控全系统"),
		),
	)
}

func newPendingGroupCommand(use string, short string, long string, children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(children...)
	return cmd
}

func newPendingLogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "日志体检与检索命令",
		Long:  "log 命令组已在 CLI 规范中保留，后续将用于日志总览、搜索和增长监控。",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliutil.PendingError(cmd.CommandPath())
		},
	}
	cmd.AddCommand(
		cliutil.NewPendingCommand("search <keyword>", "搜索日志关键字"),
		cliutil.NewPendingCommand("watch", "持续观察日志增长"),
	)
	return cmd
}

func findCommand(rootCmd *cobra.Command, name string) *cobra.Command {
	if rootCmd == nil {
		return nil
	}
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

# syskit CLI 命令规范（对齐版）

## 1. 文档定位

- 文档版本：v3.1
- 更新日期：2026-03-29
- 主文档来源：`docs/PRD.md`
- 对齐文档：
  - `docs/SYSKIT_PRODUCT_PRD.md`
  - `docs/SYSKIT_ARCHITECTURE.md`
  - `docs/SYSKIT_RULES_CATALOG.md`

本文件负责：

1. 唯一正式命令树。
2. 参数、输出、退出码协议。
3. 配置文件、策略文件、环境变量协议。

## 2. 设计原则

1. 以 `PRD.md` 中的扁平化命令作为唯一正式命令。
2. 一种行为只保留一个正式入口，不保留历史兼容命令。
3. `--format` 只负责输出格式，`--output` 只负责导出路径，二者不混用。
4. 所有结构化输出采用统一 JSON 包装。
5. 退出码既支持脚本判断，也支持 CI 阻断。

## 3. 全局参数

| 参数 | 短名 | 类型 | 默认值 | 说明 |
|---|---|---|---|---|
| `--format` | `-f` | string | `table` | 输出格式：`table/json/markdown/csv` |
| `--json` | - | bool | `false` | 等价于 `--format json` |
| `--output` | `-o` | string | 空 | 导出文件路径；为空时输出到 stdout |
| `--config` | - | string | 自动查找 | 指定配置文件路径 |
| `--policy` | - | string | 自动查找 | 指定策略文件路径 |
| `--quiet` | `-q` | bool | `false` | 仅输出核心结果或错误 |
| `--verbose` | `-v` | bool | `false` | 输出调试信息 |
| `--no-color` | - | bool | `false` | 禁用颜色 |
| `--timeout` | - | duration | 命令默认值 | 覆盖命令超时时间 |
| `--dry-run` | - | bool | `true` | 写操作默认开启，仅 fix/service/startup/file 等命令生效 |
| `--apply` | - | bool | `false` | 真实执行写操作 |
| `--yes` | `-y` | bool | `false` | 跳过危险操作确认 |
| `--fail-on` | - | string | `high` | CI 阻断阈值：`critical/high/medium/low/never` |
| `--help` | `-h` | bool | - | 显示帮助 |
| `--version` | - | bool | - | 输出版本信息 |

参数约束：

1. `--json` 优先级高于 `--format`。
2. `--output` 仅表示文件导出路径，不代表输出格式。
3. 写操作类命令在未显式传入 `--apply` 时必须保持 dry-run。
4. dangerous 级写操作必须同时具备 `--apply --yes`。

## 4. 命令总览

### 4.1 主命令树

```text
syskit
├── doctor
│   ├── all
│   ├── port
│   ├── cpu
│   ├── mem
│   ├── disk
│   ├── network
│   ├── disk-full
│   └── slowness
├── port
│   ├── <port>
│   ├── list
│   ├── kill <port>
│   ├── ping <target> <port>
│   └── scan <target>
├── proc
│   ├── top
│   ├── tree [pid]
│   ├── info <pid>
│   └── kill <pid>
├── cpu
│   ├── (overview)
│   ├── burst
│   └── watch
├── mem
│   ├── (overview)
│   ├── top
│   ├── leak <pid>
│   └── watch
├── disk
│   ├── (overview)
│   └── scan <path>
├── file
│   ├── dup <path>
│   ├── dedup <path>
│   ├── archive <path>
│   └── empty <path>
├── net
│   ├── conn
│   ├── listen
│   └── speed
├── dns
│   ├── resolve <domain>
│   └── bench <domain>
├── ping <target>
├── traceroute <target>
├── service
│   ├── list
│   ├── check <name>
│   ├── start <name>
│   ├── stop <name>
│   ├── restart <name>
│   ├── enable <name>
│   └── disable <name>
├── startup
│   ├── list
│   ├── enable <id>
│   └── disable <id>
├── log
│   ├── (overview)
│   ├── search <keyword>
│   └── watch
├── fix
│   ├── cleanup
│   └── run <script>
├── monitor
│   └── all
├── snapshot
│   ├── create
│   ├── list
│   ├── show <id>
│   ├── diff <idA> [idB]
│   └── delete <id>
├── report
│   └── generate
└── policy
    ├── show
    ├── init
    └── validate <path>
```

## 5. 版本分层

| 版本 | 命令范围 |
|---|---|
| P0 | `doctor all/port/cpu/mem/disk`、`port/*` 核心子集、`proc/*`、`cpu`、`mem`、`disk`、`disk scan`、`fix cleanup`、`snapshot *`、`report generate`、`policy *` |
| P1 | 网络全量、服务、启动项、日志、监控、`mem leak`、`cpu burst/watch`、`file *` 扩展、`fix run` |
| P2 | 插件、容器/K8s 诊断、多主机场景 |

说明：

1. 文档列出的是全量命令面，不代表全部都进入 P0。
2. 截至 2026-03-29，P0/P1 命令已在主分支完成交付，P2 仍按清单规划推进。
3. P0/P1/P2 分层定义以本节为准，产品优先级与 PRD 一致。

## 6. 命令规格

### 6.1 doctor

| 命令 | 版本 | 用途 | 关键参数 |
|---|---|---|---|
| `doctor all` | P0 | 一键体检，输出健康分、问题清单、覆盖率 | `--mode <quick/deep>` `--exclude <modules>` `--fail-on <severity>` |
| `doctor port` | P0 | 端口冲突专项诊断 | `--port <port>` `--common-ports` |
| `doctor cpu` | P0 | CPU 专项诊断 | `--threshold <percent>` `--duration <sec>` |
| `doctor mem` | P0 | 内存专项诊断 | `--threshold <percent>` |
| `doctor disk` | P0 | 磁盘专项诊断 | `--threshold <percent>` `--analyze-growth` |
| `doctor network` | P1 | 网络链路专项诊断 | `--target <address>` |
| `doctor disk-full` | P1 | 磁盘爆满场景诊断 | `--path <path>` `--top <n>` |
| `doctor slowness` | P1 | 系统卡顿场景诊断 | `--mode <quick/deep>` |

### 6.2 port

| 命令 | 版本 | 用途 | 关键参数 | 安全等级 |
|---|---|---|---|---|
| `port <port[,port]|range>` | P0 | 查询端口占用和进程链路 | `--detail` | read-only |
| `port list` | P0 | 查看监听端口列表 | `--by <pid/port>` `--protocol <tcp/udp>` `--listen <addr>` | read-only |
| `port kill <port>` | P0 | 释放端口 | `--force` `--kill-tree` `--apply` `--yes` | cautious/dangerous |
| `port ping <target> <port>` | P1 | TCP 端口可达性测试 | `--count` `--timeout` `--interval` | read-only |
| `port scan <target>` | P1 | 扫描开放端口 | `--port <range>` `--mode <quick/full>` `--timeout` | read-only |

### 6.3 proc

| 命令 | 版本 | 用途 | 关键参数 | 安全等级 |
|---|---|---|---|---|
| `proc top` | P0 | 进程资源排行 | `--by <cpu/mem/io/fd>` `--top <n>` `--user` `--name` `--watch` | read-only |
| `proc tree [pid]` | P0 | 查看进程树 | `--detail` `--full` | read-only |
| `proc info <pid>` | P0 | 查看单进程详情 | `--env` | read-only |
| `proc kill <pid>` | P0 | 结束指定进程 | `--force` `--tree` `--apply` `--yes` | cautious/dangerous |

### 6.4 cpu

| 命令 | 版本 | 用途 | 关键参数 |
|---|---|---|---|
| `cpu` | P0 | CPU 总览和高 CPU 进程概览 | `--detail` |
| `cpu burst` | P1 | 捕捉突发高 CPU 进程 | `--interval` `--duration` `--threshold` |
| `cpu watch` | P1 | 持续监控 CPU | `--top` `--interval` `--threshold-cpu` `--threshold-load` `--alert` |

### 6.5 mem

| 命令 | 版本 | 用途 | 关键参数 |
|---|---|---|---|
| `mem` | P0 | 内存总览和高内存进程概览 | `--detail` |
| `mem top` | P0 | 进程内存排行 | `--top` `--by <rss/vms/swap>` `--user` `--name` |
| `mem leak <pid>` | P1 | 进程内存泄漏趋势监控 | `--duration` `--interval` `--output` |
| `mem watch` | P1 | 持续监控内存 | `--top` `--interval` `--threshold-mem` `--threshold-swap` `--alert` |

### 6.6 disk / file

| 命令 | 版本 | 用途 | 关键参数 | 安全等级 |
|---|---|---|---|---|
| `disk` | P0 | 分区、空间、增长趋势总览 | `--detail` | read-only |
| `disk scan <path>` | P0 | 大文件/大目录扫描 | `--min-size` `--limit` `--depth` `--exclude` `--format` `--export-csv` | read-only |
| `file dup <path>` | P1 | 重复文件检测 | `--min-size` `--exclude` `--hash <md5/sha256>` | read-only |
| `fix cleanup` | P0 | 清理 temp/logs/cache | `--target <temp,logs,cache>` `--older-than <age>` `--apply` | cautious |
| `file archive <path>` | P1 | 旧日志归档 | `--older-than` `--archive-path` `--compress <gzip/zip>` `--retention` `--apply` | cautious |
| `file empty <path>` | P1 | 空目录清理 | `--apply` `--yes` | cautious |
| `file dedup <path>` | P1 | 重复文件清理 | `--apply` `--yes` | dangerous |

### 6.7 net / dns / ping / traceroute

| 命令 | 版本 | 用途 | 关键参数 |
|---|---|---|---|
| `net conn` | P1 | 网络连接审计 | `--pid` `--state` `--proto` `--remote` |
| `net listen` | P1 | 监听端口列表 | `--proto` `--addr` |
| `dns resolve <domain>` | P1 | DNS 解析工具 | `--type` `--dns` `--timeout` |
| `dns bench <domain>` | P1 | DNS 性能测试 | `--dns` `--count` |
| `ping <target>` | P1 | ICMP Ping 测试 | `--count` `--interval` `--timeout` `--size` |
| `traceroute <target>` | P1 | 路由跟踪 | `--max-hops` `--timeout` `--proto <icmp/tcp>` |
| `net speed` | P1 | 带宽测速 | `--server` `--mode <full/download/upload>` |

### 6.8 service / startup / log

| 命令 | 版本 | 用途 | 关键参数 | 安全等级 |
|---|---|---|---|---|
| `service list` | P1 | 服务列表 | `--state` `--startup` `--name` | read-only |
| `service check <name>` | P1 | 服务健康检查 | `--all` `--detail` | read-only |
| `service start|stop|restart <name>` | P1 | 服务操作 | `--apply` `--yes` | cautious |
| `service enable|disable <name>` | P1 | 服务开机自启管理 | `--apply` `--yes` | dangerous |
| `startup list` | P1 | 启动项扫描 | `--only-risk` `--user` | read-only |
| `startup enable|disable <id>` | P1 | 启动项管理 | `--apply` `--yes` | dangerous |
| `log` | P1 | 日志快速体检 | `--since` `--level` `--top` `--detail` | read-only |
| `log search <keyword>` | P1 | 日志搜索 | `--since` `--file` `--ignore-case` `--context` | read-only |
| `log watch` | P1 | 日志增长监控 | `--file` `--threshold-size` `--threshold-error` `--interval` | read-only |

### 6.9 fix / monitor / snapshot / report / policy

| 命令 | 版本 | 用途 | 关键参数 |
|---|---|---|---|
| `fix run <script>` | P1 | 执行内置或自定义修复剧本 | `--apply` `--yes` `--dry-run` `--on-fail <stop/continue>` |
| `monitor all` | P1 | 持续监控全系统 | `--interval` `--max-samples` `--alert` `--inspection-interval` `--inspection-mode` `--inspection-fail-on` `--policy` |
| `snapshot create` | P0 | 创建快照 | `--name` `--description` `--module` |
| `snapshot list` | P0 | 列出快照 | `--limit` |
| `snapshot show <id>` | P0 | 查看快照详情 | `--module` |
| `snapshot diff <idA> [idB]` | P0 | 快照对比 | `--only-change` `--module` `--output` |
| `snapshot delete <id>` | P0 | 删除快照 | `--apply` `--yes` |
| `report generate` | P0 | 生成 health/inspection/monitor 报告 | `--type <health/inspection/monitor>` `--format <markdown/json/csv>` `--output` `--time-range` |
| `policy show` | P0 | 查看生效配置和策略 | `--type <config/policy/all>` `--default` |
| `policy init` | P0 | 生成配置或策略模板 | `--type <config/policy/all>` `--output` |
| `policy validate <path>` | P0 | 校验配置或策略文件 | `--type <config/policy>` |

## 7. 输出协议

### 7.1 统一 JSON 包装

```json
{
  "code": 0,
  "msg": "ok",
  "data": {},
  "error": null,
  "metadata": {
    "schema_version": "1.0",
    "timestamp": "2026-03-12T10:30:00Z",
    "host": "dev-machine",
    "command": "syskit doctor all --format json",
    "execution_ms": 1200,
    "platform": "linux",
    "trace_id": "abc123"
  }
}
```

### 7.2 `doctor all` 数据结构

```json
{
  "health_score": 82,
  "health_level": "degraded",
  "coverage": 91.7,
  "issues": [
    {
      "rule_id": "PORT-001",
      "severity": "critical",
      "summary": "端口 8080 被非预期进程占用",
      "evidence": {
        "port": 8080,
        "pid": 1234,
        "process_name": "java"
      },
      "impact": "可能导致目标服务无法启动",
      "suggestion": "先确认进程用途，再执行端口释放",
      "fix_command": "syskit port kill 8080 --apply",
      "auto_fixable": true,
      "confidence": 100,
      "scope": "local"
    }
  ],
  "skipped": [
    {
      "module": "service",
      "reason": "permission_denied",
      "required_permission": "admin",
      "impact": "未覆盖系统服务状态",
      "suggestion": "以管理员权限重试"
    }
  ]
}
```

字段约束：

1. JSON tag 统一 snake_case。
2. 现有字段只增不减。
3. `Issue` 字段以规则目录为准，CLI 规范和架构文档必须同步。

### 7.3 错误输出格式

```json
{
  "code": 4,
  "msg": "权限不足",
  "data": null,
  "error": {
    "error_code": "ERR_PERMISSION_DENIED",
    "error_message": "需要管理员权限执行此操作",
    "suggestion": "请提升权限后重试"
  },
  "metadata": {
    "schema_version": "1.0",
    "timestamp": "2026-03-12T10:30:00Z",
    "host": "dev-machine",
    "command": "syskit port kill 8080 --apply",
    "execution_ms": 15,
    "platform": "windows",
    "trace_id": "abc456"
  }
}
```

## 8. 退出码

| 退出码 | 含义 | 场景 |
|---|---|---|
| `0` | 成功且未命中阻断阈值 | 无风险或只有可接受问题 |
| `1` | 成功但存在非阻断警告 | 存在 `medium/low` 等非阻断问题 |
| `2` | 成功但命中 `--fail-on` 阈值 | 适用于 CI 阻断 |
| `3` | 参数或配置非法 | 参数错误、配置格式错误、策略格式错误 |
| `4` | 权限不足 | 全局或关键模块需要提升权限 |
| `5` | 执行失败 | 平台不支持、外部命令失败、超时等 |
| `6` | 部分执行成功 | 批量操作部分成功、部分失败 |

错误码基线：

- `ERR_INVALID_ARGUMENT`
- `ERR_PERMISSION_DENIED`
- `ERR_PLATFORM_UNSUPPORTED`
- `ERR_EXECUTION_FAILED`
- `ERR_CONFIG_INVALID`
- `ERR_POLICY_INVALID`
- `ERR_TIMEOUT`
- `ERR_NOT_FOUND`
- `ERR_ALREADY_EXISTS`
- `ERR_STORAGE_FULL`
- `ERR_DEPENDENCY_MISSING`

## 9. 配置文件协议

### 9.1 配置优先级

`命令行参数 > 环境变量覆盖 > 用户配置 > 系统配置 > 默认值`

### 9.2 配置文件路径

| 平台 | 系统级 | 用户级 |
|---|---|---|
| Linux | `/etc/syskit/config.yaml` | `~/.syskit/config.yaml` |
| Windows | `%ProgramData%/syskit/config.yaml` | `%USERPROFILE%/.syskit/config.yaml` |
| macOS | `/Library/Application Support/syskit/config.yaml` | `~/.syskit/config.yaml` |

### 9.3 配置文件示例

```yaml
output:
  format: table
  no_color: false
  quiet: false

logging:
  level: info
  file: ~/.local/share/syskit/logs/syskit.log
  max_size_mb: 100
  max_backups: 3

storage:
  data_dir: ~/.local/share/syskit/data
  retention_days: 14
  max_storage_mb: 500

thresholds:
  cpu_percent: 80.0
  mem_percent: 90.0
  disk_percent: 85.0
  connection_count: 1000
  process_count: 500
  file_size_gb: 10.0

risk:
  require_confirm_for:
    - destructive
  dry_run_default: true

privacy:
  redact: true
  allow_no_redact: false
  redact_fields: [user, cmdline, path]

excludes:
  paths: [.git, node_modules, vendor, build, /proc, /sys]
  processes: [systemd, init]
  ports: [22, 443]

monitor:
  interval_sec: 5
  alert_threshold: 3
  max_samples: 1000

fix:
  backup_before_fix: true
  max_retry: 3
  verify_after_fix: true

report:
  default_format: markdown
  include_evidence: true
  include_suggestions: true
```

## 10. 策略文件协议

### 10.1 设计目的

策略文件用于团队标准化校验，不取代配置文件。

### 10.2 策略文件示例

```yaml
name: team-dev-standard
version: "1.0"

required_rules:
  - rule_id: PORT-001
    max_severity: high
  - rule_id: DISK-001
    max_severity: critical

threshold_overrides:
  cpu_percent: 85.0
  disk_percent: 90.0

forbidden_processes:
  - name: bitcoin-miner
    severity: critical

required_services:
  - name: docker
    platform: [linux, darwin]
  - name: Docker Desktop
    platform: [windows]

required_startup_items: []

allow_public_listen:
  - nginx
  - caddy
```

### 10.3 `policy` 命令约定

1. `policy show --type config` 输出配置。
2. `policy show --type policy` 输出策略。
3. `policy init --type config|policy|all` 生成模板。
4. `policy validate <path> --type config|policy` 校验指定文件。
5. 默认 `--type all` 仅对 `policy show` 生效。

## 11. 环境变量

| 变量名 | 说明 |
|---|---|
| `SYSKIT_CONFIG` | 默认配置文件路径 |
| `SYSKIT_POLICY` | 默认策略文件路径 |
| `SYSKIT_OUTPUT` | 默认输出格式 |
| `SYSKIT_NO_COLOR` | 是否禁用颜色输出 |
| `SYSKIT_DATA_DIR` | 数据目录 |
| `SYSKIT_LOG_LEVEL` | 日志级别覆盖 |

## 12. 协议约束

1. 新增命令时必须同步更新本文件的命令树和参数表。
2. 新增输出字段时必须同步更新架构文档的模型定义。
3. 新增或修改规则字段时必须同步更新规则目录。
4. 不允许为同一行为同时维护多个正式命令入口。

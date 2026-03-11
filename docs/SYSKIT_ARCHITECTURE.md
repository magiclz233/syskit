# syskit 架构设计文档

## 文档信息
- 文档版本：v1.0
- 对应 PRD：v1.3
- 文档日期：2026-03-11
- 目标读者：研发、测试、架构评审

## 1. 目标与范围

本文档定义 `syskit` 的实现级架构，用于支撑 PRD 的 P0/P1 落地，重点覆盖：
1. 模块目录设计
2. 核心接口定义
3. 跨平台适配设计（Windows/Linux/macOS）

P0 聚焦：
1. `doctor all`（quick）
2. `doctor port/cpu/mem/io/disk/network`
3. `inspect` 核心能力与高频别名
4. `fix port` 与 `fix cleanup`
5. `snapshot` 与 `report` 基础能力

## 2. 架构原则

1. `KISS`：优先保障高频排障链路，默认参数可直接使用。
2. `YAGNI`：不预埋插件系统，不引入复杂分布式组件。
3. `DRY`：采集、规则、输出、错误码采用统一模型。
4. `SOLID`：采集器、分析器、执行器分层；平台实现隔离到适配层。
5. 默认只读：写操作全部通过 `fix`，并要求显式确认。

## 3. 总体架构

```text
CLI(Command) -> Application(Service) -> Domain(Rule/Model) -> Infrastructure(Adapter/Store)
                                       \-> Output(Renderer: table/json/markdown)
```

执行主链路（`doctor all`）：
1. `cmd` 层解析命令与参数，加载配置与策略。
2. `application` 层编排检查模块并发执行。
3. `infrastructure` 通过平台适配器采集原始指标。
4. `domain` 层规则引擎生成问题清单、健康分、建议动作。
5. `output` 层统一渲染终端/JSON/Markdown。
6. 根据结果返回退出码（`0/1/2`）。

## 4. 模块目录设计

建议目录如下：

```text
syskit/
├── cmd/
│   ├── root.go
│   ├── inspect/
│   ├── doctor/
│   ├── fix/
│   ├── monitor/
│   ├── snapshot/
│   ├── report/
│   └── policy/
├── internal/
│   ├── app/
│   │   ├── doctor_service.go
│   │   ├── inspect_service.go
│   │   ├── fix_service.go
│   │   ├── monitor_service.go
│   │   ├── snapshot_service.go
│   │   └── report_service.go
│   ├── domain/
│   │   ├── model/
│   │   ├── rules/
│   │   └── scoring/
│   ├── collectors/
│   │   ├── port/
│   │   ├── process/
│   │   ├── cpu/
│   │   ├── mem/
│   │   ├── disk/
│   │   ├── network/
│   │   └── file/
│   ├── executors/
│   │   ├── fix_port.go
│   │   └── fix_cleanup.go
│   ├── policy/
│   ├── config/
│   ├── output/
│   │   ├── table/
│   │   ├── json/
│   │   └── markdown/
│   └── storage/
│       ├── snapshot_store.go
│       ├── monitor_store.go
│       └── retention.go
├── platform/
│   ├── common/
│   ├── windows/
│   ├── linux/
│   └── darwin/
└── pkg/
    └── types/
```

## 5. 核心接口定义

以下接口用于约束模块边界，便于替换实现与测试。

### 5.1 平台适配接口

```go
type PlatformAdapter interface {
    Name() string
    CollectPorts(ctx context.Context, filter PortFilter) ([]PortInfo, error)
    CollectProcesses(ctx context.Context, filter ProcessFilter) ([]ProcessInfo, error)
    CollectCPU(ctx context.Context, durationSec int) (CPUInfo, error)
    CollectMemory(ctx context.Context) (MemoryInfo, error)
    CollectDisk(ctx context.Context, path string) ([]DiskInfo, error)
    CollectNetwork(ctx context.Context, filter NetworkFilter) ([]ConnInfo, error)
    KillProcess(ctx context.Context, pid int, force bool) error
    Cleanup(ctx context.Context, plan CleanupPlan) (CleanupResult, error)
    CheckPermission(ctx context.Context, op Operation) PermissionLevel
}
```

### 5.2 采集与诊断接口

```go
type Collector[T any, Q any] interface {
    Collect(ctx context.Context, query Q) (T, error)
}

type Rule interface {
    ID() string
    Check(ctx context.Context, in DiagnoseInput) (*Issue, error)
}

type RuleEngine interface {
    Evaluate(ctx context.Context, in DiagnoseInput) ([]Issue, error)
}
```

### 5.3 修复执行接口

```go
type FixExecutor interface {
    Name() string
    Plan(ctx context.Context, req FixRequest) (FixPlan, error)
    Apply(ctx context.Context, plan FixPlan) (FixResult, error)
    Scope() ScopeLevel // local/system/destructive
}
```

### 5.4 输出接口

```go
type Renderer interface {
    Format() string // table/json/markdown
    Render(result CommandResult) ([]byte, error)
}
```

## 6. 关键领域模型

```go
type Issue struct {
    RuleID      string
    Severity    string // critical/high/medium/low
    Summary     string
    Evidence    any
    Impact      string
    Suggestion  string
    FixCommand  string
    AutoFixable bool
    Confidence  int // 0-100
    Scope       string // local/system/destructive
}

type DoctorReport struct {
    Timestamp      time.Time
    Host           string
    Mode           string // quick/deep
    HealthScore    int
    HealthLevel    string
    Coverage       float64
    Issues         []Issue
    SkippedModules []SkippedModule
    ExecutionMs    int64
}
```

健康分公式：
`health_score = max(0, 100 - Σ(rule_penalty * confidence_weight * scope_weight))`

## 7. 跨平台适配设计

### 7.1 适配层职责

1. 封装平台采集差异（进程、端口、网络、磁盘、日志）。
2. 统一权限判定接口与错误语义。
3. 标准化原生命令输出为统一结构体。

### 7.2 适配策略

1. 优先纯 Go（`gopsutil`）；必要时桥接系统命令。
2. 命令桥接统一由 `platform/common/exec.go` 调度，禁止业务层直接调用系统命令。
3. 所有平台实现输出同一模型，避免上层分支判断。

### 7.3 平台差异示例

1. 进程终止：
- Windows：`taskkill`
- Linux/macOS：`kill`
2. 权限提升：
- Windows：管理员/UAC
- Linux/macOS：`sudo`/root
3. 数据路径：
- 系统级：`%ProgramData%/syskit/` 或 `/var/lib/syskit/`
- 用户级：`%LOCALAPPDATA%/syskit/` 或 `~/.local/share/syskit/`

## 8. 命令与模块映射

| 命令 | Application Service | 主要依赖模块 |
|---|---|---|
| `inspect port/ports` | `InspectService` | `collectors/port`, `platform/*` |
| `inspect proc top/tree` | `InspectService` | `collectors/process` |
| `inspect mem/disk/network` | `InspectService` | `collectors/mem,disk,network` |
| `inspect file *` | `InspectService` | `collectors/file` |
| `doctor all` | `DoctorService` | `collectors/*`, `domain/rules`, `domain/scoring` |
| `doctor port/cpu/mem/io/disk/network` | `DoctorService` | 对应规则与采集器 |
| `fix port/cleanup` | `FixService` | `executors/*`, `platform/*` |
| `monitor start/stop/status` | `MonitorService` | `storage/monitor_store` |
| `snapshot create/list/diff/delete` | `SnapshotService` | `storage/snapshot_store` |
| `report generate` | `ReportService` | `output/*`, `storage/*` |
| `policy validate/show` | `PolicyService` | `policy`, `domain/rules` |

## 9. 配置、存储与保留

### 9.1 配置优先级

`命令行参数 > 用户配置 > 系统配置 > 内置默认值`

### 9.2 存储模型

1. P0 使用文件存储（JSON/JSONL），不引入 SQLite。
2. 快照文件：按创建时间与标签命名。
3. 监控文件：按日期滚动。

### 9.3 保留策略

1. 默认 `retention_days=14`。
2. 默认 `max_storage_mb=500`。
3. 超限时按时间删除最旧数据。

## 10. 权限与降级

1. `inspect/doctor` 默认普通用户可运行，受限模块标记 `skipped(permission_denied)`。
2. `fix` 系统级操作需要管理员/root。
3. 健康分仅基于已检查模块计算，并输出 `coverage`：
`coverage = 已检查模块数 / 总模块数 * 100%`
4. 报告必须包含“未检查项”列表，禁止静默忽略。

## 11. 退出码与错误语义

1. `0`：执行成功且无高风险问题。
2. `1`：执行失败（参数错误、权限错误、运行时错误）。
3. `2`：执行成功但发现高风险问题（CI 阻断）。

部分模块失败不单独定义退出码，通过 `skipped_modules` 和错误详情表达。

## 12. 可观测性与审计

1. 日志使用 `slog` 结构化输出，字段至少包含：`command`、`module`、`duration_ms`、`result`、`error_code`。
2. `fix --apply` 必须写审计日志（操作者、目标、变更前后摘要、时间戳）。
3. `--verbose` 输出模块级耗时与跳过原因。

## 13. 测试策略

1. 单元测试：规则引擎、健康分计算、参数解析、退出码映射。
2. 适配层测试：平台命令桥接与解析器（使用 fixture）。
3. 集成测试：`doctor all`、`fix` dry-run/confirm、snapshot diff。
4. 回归测试：JSON 输出字段“只增不减”契约测试。

## 14. 实施顺序（建议）

1. 先完成 `platform` 适配接口与 `collectors` 基础采集器。
2. 实现 `doctor all` 主流程与 P0 10 条规则。
3. 实现 `fix port` 与 `fix cleanup` 的 dry-run + apply 双阶段。
4. 补齐 `snapshot/report/policy` 和 CI 示例。
5. 最后收敛文档、验收用例与跨平台差异清单。

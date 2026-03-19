# syskit 架构设计文档（对齐版）

## 1. 文档定位

- 文档版本：v3.0
- 更新日期：2026-03-12
- 主文档来源：`docs/PRD.md`
- 对齐文档：
  - `docs/SYSKIT_PRODUCT_PRD.md`
  - `docs/SYSKIT_CLI_SPEC.md`
  - `docs/SYSKIT_RULES_CATALOG.md`

本文件负责：

1. 分层架构和目录结构。
2. 领域模型、请求模型、接口基线。
3. 核心执行流程、权限与存储约束。
4. 测试、发布和稳定性基线。

## 2. 架构目标

1. 支撑 P0-P1 的完整命令面，但保证 P0 先闭环。
2. 以统一模型承接 CLI、规则、报告、审计和 CI 输出。
3. 平台差异收敛到适配层，上层不感知系统命令差异。
4. 写操作默认安全、可审计、可确认。
5. 保证 JSON 协议、规则字段、错误码的一致性。

## 3. 总体架构

```text
CLI(Command) -> Application(Service) -> Domain(Rule/Score/Policy) -> Infrastructure(Adapter/Store)
                                       -> Output(Renderer: table/json/markdown/csv)
```

分层职责：

| 分层 | 责任 | 不负责 |
|---|---|---|
| `cmd/<binary>` | 二进制入口、版本注入、退出码桥接 | 参数解析、规则判断 |
| `internal/cli` | 参数解析、命令路由、帮助输出 | 规则判断、平台命令细节 |
| `app` | 用例编排、超时控制、事务边界、权限校验 | 采集实现、渲染实现 |
| `domain` | 模型、规则、评分、策略校验、风险分级 | OS 命令调用 |
| `collectors/executors` | 采集系统数据、执行修复动作 | CLI 参数解析 |
| `platform` | Windows/Linux/macOS 差异封装 | 业务判断 |
| `storage/audit` | 快照、监控、报告、审计落盘 | 业务编排 |
| `output` | table/json/markdown/csv 渲染 | 数据采集、规则判断 |

## 4. 推荐目录结构

```text
syskit/
├── cmd/
│   └── syskit/
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── root.go
│   │   ├── global.go
│   │   ├── doctor/
│   │   ├── port/
│   │   ├── proc/
│   │   ├── cpu/
│   │   ├── mem/
│   │   ├── disk/
│   │   ├── file/
│   │   ├── net/
│   │   ├── service/
│   │   ├── startup/
│   │   ├── log/
│   │   ├── fix/
│   │   ├── monitor/
│   │   ├── snapshot/
│   │   ├── report/
│   │   └── policy/
│   ├── app/
│   ├── domain/
│   │   ├── model/
│   │   ├── rules/
│   │   ├── scoring/
│   │   └── policy/
│   ├── collectors/
│   ├── executors/
│   ├── output/
│   ├── config/
│   ├── storage/
│   ├── audit/
│   ├── errs/
│   └── version/
├── platform/
│   ├── common/
│   ├── windows/
│   ├── linux/
│   └── darwin/
└── pkg/
```

目录规则：

1. `cmd/<binary>` 只保留入口装配，不承载命令定义。
2. `internal/cli` 只做参数和路由，不落业务逻辑。
3. `domain` 不依赖 `internal/cli` 和 `platform`。
4. `platform` 只负责差异适配，不得生成规则结论。
5. `output` 只能读取标准结果对象。
6. `storage` 不得反向依赖 `internal/cli`。

## 5. 核心领域模型

### 5.1 输出模型

```go
type CommandResult struct {
    Code     int         `json:"code"`
    Msg      string      `json:"msg"`
    Data     any         `json:"data"`
    Error    *ErrorInfo  `json:"error,omitempty"`
    Metadata Metadata    `json:"metadata"`
}

type Metadata struct {
    SchemaVersion string    `json:"schema_version"`
    Timestamp     time.Time `json:"timestamp"`
    Host          string    `json:"host"`
    Command       string    `json:"command"`
    ExecutionMs   int64     `json:"execution_ms"`
    Platform      string    `json:"platform"`
    TraceID       string    `json:"trace_id"`
}

type ErrorInfo struct {
    ErrorCode    string `json:"error_code"`
    ErrorMessage string `json:"error_message"`
    Suggestion   string `json:"suggestion"`
}
```

### 5.2 诊断模型

```go
type DoctorReport struct {
    HealthScore int             `json:"health_score"`
    HealthLevel string          `json:"health_level"`
    Coverage    float64         `json:"coverage"`
    Issues      []Issue         `json:"issues"`
    Skipped     []SkippedModule `json:"skipped"`
}

type Issue struct {
    RuleID      string `json:"rule_id"`
    Severity    string `json:"severity"`
    Summary     string `json:"summary"`
    Evidence    any    `json:"evidence"`
    Impact      string `json:"impact"`
    Suggestion  string `json:"suggestion"`
    FixCommand  string `json:"fix_command"`
    AutoFixable bool   `json:"auto_fixable"`
    Confidence  int    `json:"confidence"`
    Scope       string `json:"scope"`
}

type SkippedModule struct {
    Module             string `json:"module"`
    Reason             string `json:"reason"`
    RequiredPermission string `json:"required_permission"`
    Impact             string `json:"impact"`
    Suggestion         string `json:"suggestion"`
}
```

说明：

1. `Issue` 是规则、CLI、报告的共享模型。
2. 如需扩展 `category`、`fix_safety_level`，必须先同步 CLI 规范和规则目录。
3. 现阶段不在基础协议中强制 `category`，避免重复来源。

### 5.3 请求模型

```go
type DoctorRequest struct {
    Mode      string
    Severity  []string
    Exclude   []string
    FailOn    string
    Timeout   time.Duration
}

type PortQuery struct {
    Targets  []string
    Detail   bool
    Protocol string
}

type ProcTopQuery struct {
    By       string
    Top      int
    User     string
    Name     string
    Watch    bool
    Interval time.Duration
}

type FileQuery struct {
    Path      string
    MinSize   string
    Limit     int
    Depth     int
    Excludes  []string
    Algorithm string
}

type FixRequest struct {
    Action string
    Target string
    Params map[string]string
    Apply  bool
    Yes    bool
}

type ReportRequest struct {
    Type      string
    Format    string
    Output    string
    TimeRange string
}
```

### 5.4 配置和策略模型

```go
type Config struct {
    Output     OutputConfig     `yaml:"output"`
    Logging    LoggingConfig    `yaml:"logging"`
    Storage    StorageConfig    `yaml:"storage"`
    Thresholds ThresholdsConfig `yaml:"thresholds"`
    Risk       RiskConfig       `yaml:"risk"`
    Privacy    PrivacyConfig    `yaml:"privacy"`
    Excludes   ExcludesConfig   `yaml:"excludes"`
    Monitor    MonitorConfig    `yaml:"monitor"`
    Fix        FixConfig        `yaml:"fix"`
    Report     ReportConfig     `yaml:"report"`
}

type Policy struct {
    Name                string                      `yaml:"name"`
    Version             string                      `yaml:"version"`
    RequiredRules       []RequiredRule              `yaml:"required_rules"`
    ThresholdOverrides  map[string]float64          `yaml:"threshold_overrides"`
    ForbiddenProcesses  []ForbiddenProcess          `yaml:"forbidden_processes"`
    RequiredServices    []RequiredService           `yaml:"required_services"`
    RequiredStartup     []RequiredStartupItem       `yaml:"required_startup_items"`
    AllowPublicListen   []string                    `yaml:"allow_public_listen"`
}
```

## 6. 核心接口基线

### 6.1 Service 接口

```go
type DoctorService interface {
    All(ctx context.Context, req DoctorRequest) (DoctorReport, error)
    Port(ctx context.Context, port int) (CommandResult, error)
    CPU(ctx context.Context, req DoctorRequest) (CommandResult, error)
    Mem(ctx context.Context, req DoctorRequest) (CommandResult, error)
    Disk(ctx context.Context, req DoctorRequest) (CommandResult, error)
    Network(ctx context.Context, req DoctorRequest) (CommandResult, error)
}

type InspectService interface {
    Port(ctx context.Context, q PortQuery) (CommandResult, error)
    PortList(ctx context.Context, q PortQuery) (CommandResult, error)
    ProcTop(ctx context.Context, q ProcTopQuery) (CommandResult, error)
    ProcTree(ctx context.Context, pid *int) (CommandResult, error)
    ProcInfo(ctx context.Context, pid int) (CommandResult, error)
    CPU(ctx context.Context) (CommandResult, error)
    Mem(ctx context.Context) (CommandResult, error)
    Disk(ctx context.Context, q FileQuery) (CommandResult, error)
}

type FixService interface {
    Execute(ctx context.Context, req FixRequest) (CommandResult, error)
    RunScript(ctx context.Context, path string, req FixRequest) (CommandResult, error)
}

type SnapshotService interface {
    Create(ctx context.Context, modules []string) (CommandResult, error)
    Diff(ctx context.Context, baseID, targetID string, modules []string) (CommandResult, error)
}

type PolicyService interface {
    Show(ctx context.Context, kind string) (CommandResult, error)
    Init(ctx context.Context, kind string, output string) (CommandResult, error)
    Validate(ctx context.Context, kind string, path string) (CommandResult, error)
}
```

### 6.2 Domain 接口

```go
type Rule interface {
    ID() string
    Phase() string
    Check(ctx context.Context, in DiagnoseInput) (*Issue, error)
}

type RuleEngine interface {
    Evaluate(ctx context.Context, in DiagnoseInput, enabled []string) ([]Issue, error)
}

type Scorer interface {
    Score(issues []Issue, coverage float64) (score int, level string)
}
```

### 6.3 Infrastructure 接口

```go
type PlatformAdapter interface {
    Name() string
    CollectPorts(ctx context.Context, filter PortFilter) ([]PortInfo, error)
    CollectProcesses(ctx context.Context, filter ProcessFilter) ([]ProcessInfo, error)
    CollectCPU(ctx context.Context, durationSec int) (CPUInfo, error)
    CollectMemory(ctx context.Context) (MemoryInfo, error)
    CollectDisk(ctx context.Context, path string) ([]DiskInfo, error)
    CollectNetwork(ctx context.Context, filter NetworkFilter) ([]ConnInfo, error)
    CollectServices(ctx context.Context) ([]ServiceInfo, error)
    CollectStartupItems(ctx context.Context) ([]StartupItem, error)
    CollectLogs(ctx context.Context, q LogQuery) ([]LogEntry, error)
    KillProcess(ctx context.Context, pid int, force bool) error
    Cleanup(ctx context.Context, plan CleanupPlan) (CleanupResult, error)
    CheckPermission(ctx context.Context, op string) (bool, string)
}

type SnapshotStore interface {
    Save(ctx context.Context, snapshot Snapshot) error
    Load(ctx context.Context, id string) (Snapshot, error)
    List(ctx context.Context) ([]SnapshotMeta, error)
    Delete(ctx context.Context, id string) error
}

type MonitorStore interface {
    Append(ctx context.Context, item MonitorSample) error
    Query(ctx context.Context, q MonitorQuery) ([]MonitorSample, error)
}

type AuditLogger interface {
    Log(ctx context.Context, event AuditEvent) error
}
```

## 7. 核心执行流程

### 7.1 `doctor all`

1. 解析 CLI 参数并加载配置、策略。
2. 根据 `mode/exclude` 选择模块和规则集合。
3. 并发执行 collectors，收集 `DiagnoseInput`。
4. 权限不足、超时、平台不支持写入 `SkippedModule`。
5. 规则引擎遍历命中规则，生成 `Issue[]`。
6. 评分器计算 `health_score` 和 `health_level`。
7. 输出渲染器生成 `table/json/markdown/csv`。
8. 根据 `--fail-on` 决定退出码。

并发基线：

1. 默认并发度 `min(模块数, 6)`。
2. 单模块默认超时 8 秒，总体默认超时 60 秒。
3. 模块失败不应导致整体 panic。

### 7.2 `port kill` / `proc kill` / `fix cleanup`

1. `discover`：定位目标资源。
2. `plan`：生成 dry-run 计划。
3. `confirm`：检查 `--apply` 和 `--yes`。
4. `apply`：执行系统动作。
5. `verify`：校验执行后状态。
6. `audit`：记录 before/after/result。

### 7.3 `disk scan` / `file dup`

1. 校验路径、权限、排除规则。
2. 递归遍历目录，跳过符号链接和排除目录。
3. `disk scan` 聚合大文件/大目录。
4. `file dup` 先按大小分桶，再计算哈希。
5. 单个文件失败写入 `skipped_paths`，不终止整体扫描。

### 7.4 `snapshot diff`

1. 加载基线快照和目标快照。
2. 按模块和字段级别比对。
3. 识别新增、删除、变化项。
4. 对高风险变化追加规则标记。

### 7.5 `policy validate`

1. 根据 `--type` 判断是 config 还是 policy。
2. 进行 YAML 解析。
3. 进行 schema 校验、字段取值校验、引用规则校验。
4. 输出通过/失败和具体问题位置。

## 8. 评分与降级策略

### 8.1 健康分

推荐公式：

`health_score = max(0, 100 - Σ(base_penalty * confidence_weight * scope_weight))`

基线扣分：

| severity | base_penalty |
|---|---|
| critical | 20 |
| high | 10 |
| medium | 5 |
| low | 2 |

说明：

1. `confidence` 用于降低低置信度问题的扣分权重。
2. `scope` 用于区分 local/system/destructive 风险影响面。
3. `coverage` 单独输出，不直接折算为扣分。

### 8.2 权限降级

1. 所有只读命令和 `doctor` 默认普通用户可执行。
2. 系统级 fix、service、startup 操作要求管理员/root。
3. 权限不足时必须写入 `skipped`，不能静默跳过。
4. 退出码优先反映命令执行状态，覆盖率反映检查完整度。

## 9. 配置、存储与审计

### 9.1 配置加载

优先级：`命令行 > 环境变量覆盖 > 用户配置 > 系统配置 > 默认值`

实现要求：

1. 支持 `SYSKIT_CONFIG`、`SYSKIT_POLICY` 环境变量覆盖。
2. CLI 参数覆盖配置时写入 `metadata.overrides` 供调试使用。
3. 配置错误返回 `ERR_CONFIG_INVALID`，策略错误返回 `ERR_POLICY_INVALID`。

### 9.2 存储目录

| 平台 | 系统级数据目录 | 用户级数据目录 |
|---|---|---|
| Linux/macOS | `/var/lib/syskit/` | `~/.local/share/syskit/` |
| Windows | `%ProgramData%/syskit/` | `%LOCALAPPDATA%/syskit/` |

建议布局：

```text
data/
├── snapshots/
├── monitor/
├── reports/
└── audit/
```

保留策略：

1. 默认 `retention_days=14`。
2. 默认 `max_storage_mb=500`。
3. 超限后按时间删除最旧数据。
4. 清理任务执行时持有独占锁。

### 9.3 审计日志

仅对真实写操作记录审计：

- `timestamp`
- `trace_id`
- `operator`
- `command`
- `action`
- `target`
- `before`
- `after`
- `result`
- `error_msg`
- `duration_ms`
- `metadata`

## 10. 平台适配原则

1. 优先使用 `gopsutil` 统一采集接口。
2. 平台特有能力封装在 `platform/windows|linux|darwin`。
3. 对于 `service`、`startup`、`log` 等能力，需要显式处理平台支持矩阵。
4. 无法支持的命令返回 `ERR_PLATFORM_UNSUPPORTED`，并在 CLI 输出中说明。

## 11. 技术选型

| 能力 | 方案 |
|---|---|
| CLI | `cobra` + `pflag` |
| 系统采集 | `gopsutil` |
| 配置解析 | `gopkg.in/yaml.v3` |
| 日志 | `log/slog` |
| 并发控制 | `context` + `errgroup` |
| 存储 | JSON / JSONL + 文件锁 |
| 报告模板 | `text/template` |
| 测试 | `testing`，必要时补充 `testify` |

约束：

1. P0 不引入 SQLite。
2. P0-P1 不引入插件框架。
3. 远程配置中心、消息队列不在当前架构范围内。

## 12. 测试基线

### 12.1 单元测试

1. 规则阈值边界和误报排除。
2. 评分逻辑和退出码逻辑。
3. 参数解析与配置合并。
4. 输出渲染和 JSON 字段完整性。

### 12.2 集成测试

1. `doctor all` 全链路。
2. `port kill` / `proc kill` / `fix cleanup` 的 dry-run 与 apply。
3. `snapshot create/diff`。
4. `policy validate`。

### 12.3 契约测试

1. JSON 协议字段只增不减。
2. 规则输出字段和规则目录保持一致。
3. CLI 帮助与命令树一致。

### 12.4 性能与平台测试

1. `doctor all --mode quick` 30 秒内完成。
2. 查询类命令常规场景 2 秒内完成。
3. 重复文件扫描支持大目录且内存可控。
4. Windows/Linux/macOS 三平台完成核心命令验证。

## 13. 分阶段实施

| 阶段 | 重点 |
|---|---|
| P0 | CLI 骨架、统一输出、核心 collectors、规则引擎、核心 fix、snapshot/report/policy |
| P1 | network/service/startup/log、持续监控、修复剧本、Webhook、模板报告 |
| P2 | 插件、容器/K8s、多主机、Web 能力 |

## 14. 架构协同约束

1. 命令协议改动先改 CLI 规范，再改代码。
2. 共享模型改动先改本文件，再改 CLI 和规则目录。
3. 规则证据结构改动先改规则目录，再改模型和渲染器。
4. 不允许在多个文档分别维护同一字段的不同版本。

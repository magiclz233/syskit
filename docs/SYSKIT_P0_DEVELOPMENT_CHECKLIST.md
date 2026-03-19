# syskit P0 开发需求清单

## 1. 文档目的

这份文档是 `syskit` P0 阶段的唯一开发控制清单，用于管理以下事项：

1. P0 范围内需要开发的需求项。
2. 需求项的开发顺序、依赖关系和当前状态。
3. 每次开发完成后需要回写的完成标记。
4. 后续开发时默认继续处理的“下一个未完成需求”。

本清单对齐以下文档：

- `docs/SYSKIT_PRODUCT_PRD.md`
- `docs/SYSKIT_CLI_SPEC.md`
- `docs/SYSKIT_ARCHITECTURE.md`
- `docs/SYSKIT_RULES_CATALOG.md`

## 2. 使用规则

1. 本文档确认前，不进入正式 P0 开发。
2. 文档确认后，默认严格按本文档自上而下顺序开发。
3. 若某项未完成，默认不跳过到后续项；确需跳过时，需要先明确调整优先级。
4. 每完成一项开发并完成基本验证后，将状态更新为 `已开发`。
5. 下一次继续开发时，从第一个状态不是 `已开发` 的需求项继续。
6. 若某项拆分为多次提交，状态可暂记为 `开发中`，完成后再改为 `已开发`。

## 3. 状态约定

| 状态 | 含义 |
|---|---|
| `待确认` | 已整理但尚未得到用户确认，不开始开发 |
| `待开发` | 已确认但尚未开始 |
| `开发中` | 正在实现中 |
| `已开发` | 已完成实现并做过基本验证 |
| `暂缓` | 暂不继续，等待重新排期 |

当前整份清单状态：`已开发`

## 4. P0 范围边界

本清单只覆盖 P0，包含：

1. CLI 骨架、统一输出协议、错误码、配置与策略基线。
2. `doctor all/port/cpu/mem/disk`。
3. `port`、`proc`、`cpu`、`mem`、`disk`、`disk scan`。
4. `fix cleanup`。
5. `snapshot *`、`report generate`、`policy *`。
6. P0 规则、评分、退出码、审计、测试和发布基线。

覆盖说明：

1. 本清单以 `SYSKIT_PRODUCT_PRD.md`、`SYSKIT_CLI_SPEC.md`、`SYSKIT_ARCHITECTURE.md`、`SYSKIT_RULES_CATALOG.md` 中所有 P0 范围要求为基准。
2. 不仅覆盖 P0 命令和规则，也覆盖 P0 的协议、降级、审计、测试、发布和完成定义要求。
3. 若后续发现四份文档中的某条 P0 要求没有映射到本清单，应先补充到本清单，再继续开发。

本清单不包含 P1/P2 内容，例如：

1. `net`、`dns`、`ping`、`traceroute`。
2. `service`、`startup`、`log`、`monitor`。
3. `file dup`、`file dedup`、`file archive`、`file empty`。
4. 插件、容器/K8s、多主机能力。

## 5. 前置确认项

| 顺序 | ID | 状态 | 事项 | 说明 |
|---|---|---|---|---|
| 0 | `P0-PRE-001` | `已开发` | 项目名称与模块路径收敛 | 项目名称确定为 `syskit`，并立即从 `find-large-files` 向 `syskit` 迁移 |

## 6. P0 开发清单

### 阶段 A：基础骨架与协议

| 顺序 | ID | 状态 | 需求项 | 关键交付 |
|---|---|---|---|---|
| 1 | `P0-001` | `已开发` | 建立新 CLI 工程骨架 | `cmd/syskit/main.go` 仅做启动；引入 `internal/cli` 根命令；子命令注册结构落地 |
| 2 | `P0-002` | `已开发` | 实现全局参数与统一退出码基线 | 落地 `--format`、`--json`、`--output`、`--config`、`--policy`、`--quiet`、`--verbose`、`--dry-run`、`--apply`、`--yes`、`--fail-on`、`--timeout`、`--no-color` |
| 3 | `P0-003` | `已开发` | 实现统一结果模型与输出渲染 | 落地 `CommandResult`、`Metadata`、`ErrorInfo`，支持 `table/json/markdown/csv` 基础渲染 |
| 4 | `P0-004` | `已开发` | 实现错误码与元数据协议 | 落地 `ERR_INVALID_ARGUMENT`、`ERR_PERMISSION_DENIED`、`ERR_EXECUTION_FAILED` 等 P0 所需错误码及统一封装 |
| 5 | `P0-005` | `已开发` | 实现配置加载基线 | 支持默认值、系统配置、用户配置、环境变量覆盖、命令行覆盖 |
| 6 | `P0-006` | `已开发` | 实现策略模型与 `policy` 命令 | 支持 `policy show`、`policy init`、`policy validate`，并完成 config/policy 模型与校验 |

### 阶段 B：核心采集与高频命令

| 顺序 | ID | 状态 | 需求项 | 关键交付 |
|---|---|---|---|---|
| 7 | `P0-007` | `已开发` | 迁移现有扫描器为 `disk scan` | 复用当前大文件/大目录扫描能力，对齐 `syskit disk scan <path>` 协议与参数 |
| 8 | `P0-008` | `已开发` | 实现 `disk` 总览命令 | 输出分区、空间、使用率等 P0 基础信息 |
| 9 | `P0-009` | `已开发` | 实现 `proc` 命令集 | 完成 `proc top`、`proc tree`、`proc info`、`proc kill` |
| 10 | `P0-010` | `已开发` | 实现 `port` 命令集 | 完成 `port <port>`、`port list`、`port kill <port>` |
| 11 | `P0-011` | `已开发` | 实现 `cpu` 命令 | 完成 `cpu` 总览和高 CPU 进程概览 |
| 12 | `P0-012` | `已开发` | 实现 `mem` 命令集 | 完成 `mem` 总览和 `mem top` |
| 13 | `P0-013` | `已开发` | 实现 `fix cleanup` | 支持 dry-run、`--apply`、基础清理计划、执行和校验流程 |

### 阶段 C：诊断与规则闭环

| 顺序 | ID | 状态 | 需求项 | 关键交付 |
|---|---|---|---|---|
| 14 | `P0-014` | `已开发` | 建立规则模型与规则引擎 | 落地 `Issue`、`SkippedModule`、规则执行接口、模块级降级处理 |
| 15 | `P0-015` | `已开发` | 实现 P0 评分与 `--fail-on` 逻辑 | 完成健康分、健康等级、覆盖率、CI 阻断阈值计算 |
| 16 | `P0-016` | `已开发` | 实现 P0 规则集 | 至少落地 `PORT-001`、`PORT-002`、`PROC-001`、`PROC-002`、`CPU-001`、`MEM-001`、`DISK-001`、`DISK-002`、`FILE-001`、`ENV-001` |
| 17 | `P0-017` | `已开发` | 实现专项诊断命令 | 完成 `doctor port`、`doctor cpu`、`doctor mem`、`doctor disk` |
| 18 | `P0-018` | `已开发` | 实现 `doctor all` | 完成模块编排、并发采集、跳过项、问题清单、统一输出与退出码 |

### 阶段 D：存储、报告与审计

| 顺序 | ID | 状态 | 需求项 | 关键交付 |
|---|---|---|---|---|
| 19 | `P0-019` | `已开发` | 实现存储目录与保留策略基线 | 建立快照、报告、审计数据目录与基础清理策略 |
| 20 | `P0-020` | `已开发` | 实现 `snapshot` 命令集 | 完成 `create`、`list`、`show`、`diff`、`delete` |
| 21 | `P0-021` | `已开发` | 实现 `report generate` | 支持 `health/inspection/monitor` 中 P0 实际可生成的报告类型与导出 |
| 22 | `P0-022` | `已开发` | 实现真实写操作审计日志 | `port kill`、`proc kill`、`fix cleanup`、`snapshot delete` 等写操作记录审计 |
| 23 | `P0-023` | `已开发` | 实现权限不足/超时/平台不支持的统一降级 | 所有 P0 命令对 `permission_denied`、`timeout`、`unsupported` 具备可解释的降级输出，`doctor` 输出 `skipped` 与 `coverage` |

### 阶段 E：质量、契约与发布基线

| 顺序 | ID | 状态 | 需求项 | 关键交付 |
|---|---|---|---|---|
| 24 | `P0-024` | `已开发` | 补齐单元测试 | 覆盖配置合并、规则判断、评分逻辑、输出渲染、参数约束 |
| 25 | `P0-025` | `已开发` | 补齐集成测试 | 覆盖 `doctor all`、`disk scan`、`port kill/proc kill/fix cleanup` dry-run 与 apply、`snapshot`、`policy validate` |
| 26 | `P0-026` | `已开发` | 补齐契约测试与帮助校验 | 校验 JSON 字段、规则字段、CLI 帮助和命令树一致性 |
| 27 | `P0-027` | `已开发` | 补齐命令帮助、示例与错误提示 | 满足 P0 DoD 中“命令帮助、示例、错误提示完整”的要求 |
| 28 | `P0-028` | `已开发` | 完成三平台编译与 P0 核心命令验证 | 满足 Windows/Linux/macOS 三平台编译通过和 P0 验证要求 |
| 29 | `P0-029` | `已开发` | 完成 P0 文档与发布基线收口 | 同步 README、快速开始、开发说明、构建与发布脚本、版本流程 |

## 7. 当前推荐开发顺序

确认本清单后，推荐按以下大顺序推进：

1. 先做阶段 A，确保 CLI、协议、配置、策略骨架成立。
2. 再做阶段 B，先把 P0 高频命令全部跑通。
3. 再做阶段 C，形成 `doctor` 诊断闭环。
4. 再做阶段 D，补快照、报告、审计和持久化能力。
5. 最后做阶段 E，完成测试、契约、文档和发布收口。

## 8. 完成标记模板

每个需求项完成后，在对应行状态改为 `已开发`，并在本节追加记录：

```text
- 2026-03-14: P0-001 已开发，完成 CLI 根命令骨架与子命令注册。
```

开发记录：

- 2026-03-14: `P0-PRE-001` 已开发，项目名称确定为 `syskit`，并启动从 `find-large-files` 到 `syskit` 的命名迁移。
- 2026-03-14: `P0-001` 已开发，完成 `cmd/syskit/main.go` + `internal/cli/` 目录骨架、P0 子命令注册，并保留现有根命令目录扫描能力。
- 2026-03-14: `P0-002` 已开发，完成全局 persistent flags 接入，并建立统一退出码基线实现。
- 2026-03-14: `P0-003` 已开发，完成统一 `CommandResult/Metadata/ErrorInfo` 模型、基础 renderer，以及根命令扫描的统一输出接入。
- 2026-03-14: `P0-004` 已开发，完成统一 `CLIError` 封装、P0 错误码映射、JSON/Markdown 错误渲染和退出码协议，并修正配置/策略校验类错误的封装行为。
- 2026-03-14: `P0-005` 已开发，完成默认值、系统配置、用户配置、环境变量和命令行覆盖的配置加载链路，并补齐基础字段校验与优先级测试。
- 2026-03-14: `P0-006` 已开发，完成策略文件模型、校验器、自动查找和 `policy show/init/validate` 命令闭环。
- 2026-03-14: `P0-007` 已开发，完成共享扫描执行器、`disk scan` 命令接入，并补齐 `--limit`、`--min-size`、`--depth`、`--exclude`、`--export-csv` 参数支持。
- 2026-03-16: `P0-008` 已开发，完成 `disk` 总览命令，支持分区容量/使用率输出、`--detail` 详细字段以及 table/json/markdown/csv 渲染。
- 2026-03-16: `P0-009` 已开发，完成 `proc top/tree/info/kill` 命令、dry-run 与 `--apply --yes` 执行门禁、跨平台进程采集封装与基础测试。
- 2026-03-16: `P0-010` 已开发，完成 `port` 查询表达式解析、`port list` 监听列表、`port kill` dry-run/apply 流程与 `--force --kill-tree` 参数接入，并补齐基础测试。
- 2026-03-16: `P0-011` 已开发，完成 `cpu` 总览与高 CPU 进程概览、`--detail` 每核心输出、以及在平台不支持/权限受限场景下的可解释降级提示。
- 2026-03-16: `P0-012` 已开发，完成 `mem` 总览与 `mem top`，支持 `--detail`、`--top`、`--by <rss/vms/swap>`、`--user`、`--name`，并补充 swap 指标不可用时的降级提示。
- 2026-03-16: `P0-013` 已开发，完成 `fix cleanup` 的 dry-run/apply 流程、`--target/--older-than` 参数、discover-plan-apply-verify 清理链路以及 table/json/markdown/csv 输出。
- 2026-03-16: `P0-014` 已开发，完成 `Issue`/`SkippedModule` 领域模型、规则执行接口与默认规则引擎、模块级降级归类（permission_denied/timeout/unsupported）及对应单元测试。
- 2026-03-16: `P0-015` 已开发，完成默认评分器（severity/confidence/scope 权重扣分）、健康等级映射、`--fail-on` 阈值匹配与 doctor 场景退出码计算，并补齐单元测试。
- 2026-03-16: `P0-016` 已开发，完成 `PORT-001/002`、`PROC-001/002`、`CPU-001`、`MEM-001`、`DISK-001/002`、`FILE-001`、`ENV-001` 十条 P0 规则实现，补齐规则输入模型与规则集单元测试。
- 2026-03-16: `P0-017` 已开发，完成 `doctor port/cpu/mem/disk` 四个专项诊断命令，打通采集→规则评估→统一输出→退出码链路，并支持专项参数覆盖与模块降级为 skipped。
- 2026-03-16: `P0-018` 已开发，完成 `doctor all` 模块编排与并发采集，接入覆盖率、问题清单、跳过项汇总和 `--fail-on` 阈值退出行为。
- 2026-03-17: `P0-019` 已开发，新增 `internal/storage` 基线能力：自动创建 `snapshots/monitor/reports/audit` 数据目录，接入 `retention_days + max_storage_mb` 保留策略与独占锁清理，并在 CLI 初始化阶段统一执行。
- 2026-03-18: `P0-020` 已开发，完成 `snapshot create/list/show/diff/delete`，新增快照存储层（落盘、查询、删除）、模块采集快照、差异对比与 dry-run/apply 删除门禁，并补齐单元测试。
- 2026-03-18: `P0-021` 已开发，完成 `report generate`（`--type health/inspection/monitor`、`--time-range`），支持 table/json/markdown/csv 导出，接入快照窗口统计、health 回退策略与 monitor 目录降级提示，并补齐基础单元测试。
- 2026-03-18: `P0-022` 已开发，新增 `internal/audit` 审计模块并落地 JSONL 审计日志，接入 `port kill`、`proc kill`、`fix cleanup`、`snapshot delete` 的 apply 执行路径，记录 command/action/target/before/after/result/error/duration/metadata。
- 2026-03-18: `P0-023` 已开发，统一错误降级输出协议：终端错误输出补充 `错误码`，并对 `permission_denied`、`timeout`、`unsupported` 输出标准化降级说明；同时补齐错误渲染与审计模块单元测试。
- 2026-03-18: `P0-024` 已开发，补齐单元测试闭环：新增 `global` 参数优先级测试（命令行/环境变量/配置）、补充评分与规则边界/负例、扩展输出渲染路由与错误协议测试，并新增 `cpu/disk/doctor/fix/policy` 命令参数约束测试，验证命令：`go test ./...`。
- 2026-03-19: `P0-025` 已开发，新增 `internal/cli` 黑盒集成测试 harness，补齐 `doctor all`、`disk scan`、`fix cleanup`、`snapshot`、`policy validate`、`proc kill`、`port kill` 的 dry-run/apply 集成测试与审计验证。
- 2026-03-19: `P0-026` 已开发，新增 JSON envelope、规则字段、帮助文案和命令树路径的契约测试，确保字段只增不减且 CLI 规范与实现一致。
- 2026-03-19: `P0-027` 已开发，根命令改为帮助优先，移除 legacy scan 入口与根级扫描 flags，补齐 P0 命令 `Long/Example`、危险操作说明，并注册 P1/P2 占位命令保持帮助树稳定。
- 2026-03-19: `P0-028` 已开发，构建矩阵收窄为 6 个正式支持目标，新增 `scripts/verify-p0.(sh|bat)`，用于统一执行 `go test ./...`、六目标编译和核心 help/smoke 验证；当前 Windows 开发机已完成一次 `scripts\verify-p0.bat` 验证。
- 2026-03-19: `P0-029` 已开发，收口 `README`、快速开始、开发/发布/设计说明、脚本说明与 P0 平台验证清单，并删除旧的 `scripts/find-largest-local.ps1`。
- 2026-03-14: 目录结构已迁移为 `cmd/<binary> + internal/cli`，并同步更新构建脚本、发布脚本和架构文档。

## 9. 已确认事项

1. 本清单作为 P0 唯一开发顺序文档使用。
2. 默认按本文档顺序逐项开发，不随意跳项。
3. 项目名称确定为 `syskit`。

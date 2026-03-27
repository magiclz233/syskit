# 设计说明

## 设计目标

P0 的设计目标不是做“功能很多”的 CLI，而是先建立一套稳定、可解释、可脚本化的运维命令基线：

- 命令树稳定
- 输出协议统一
- 写操作可审计
- 平台差异可降级
- 文档、帮助和测试一起收口

## 核心取舍

### 1. 根命令帮助优先

`syskit` 无参数直接显示帮助，而不是执行任何隐式扫描。

这样做的原因：

- 避免历史兼容入口继续扩散
- 让脚本和人工使用都围绕正式命令树展开
- 把“扫描”明确收敛到 `disk scan`

### 2. 单一正式扫描入口

目录扫描只保留 `syskit disk scan <path>`，不再维护根命令兼容行为。

这样可以保证：

- 参数定义只维护一处
- 帮助文案和脚本示例不再分叉
- 集成测试和契约测试可以稳定锁定扫描入口

### 3. 写操作默认 dry-run

`port kill`、`proc kill`、`fix cleanup`、`snapshot delete` 这类写操作默认只输出计划。

真实执行必须显式传入：

```text
--apply --yes
```

这样做的原因：

- 减少误操作
- 让帮助文案和 CLI 行为一致
- 便于审计日志只记录真实变更

### 4. P1/P2 默认占位，按清单渐进落地

命令树中保留 `service`、`net`、`dns`、`file`、`monitor` 等占位命令，并按开发清单逐项替换为正式实现（例如已落地 `port ping/scan`）。

这是刻意遵循 YAGNI：

- 先把 P0 命令面、帮助和测试收稳，再按优先级推进 P1/P2
- 避免未来需求未定时做半成品实现
- 让 CLI 规范、帮助树和后续扩展路径提前一致

## 架构分层

### CLI 层

`internal/cli/` 负责：

- Cobra 命令树注册
- 参数绑定与校验
- `Long/Example` 帮助文案
- presenter 选择与输出
- 黑盒集成测试和契约测试

### 采集层

`internal/collectors/` 负责端口、进程、CPU、内存、磁盘等实际采集。

这一层只返回结构化数据和可解释 warning，不负责 CLI 展示。

### 领域层

`internal/domain/` 负责：

- `Issue` / `SkippedModule`
- 评分与健康等级
- 规则引擎
- doctor/snapshot/report 共享模型

### 配置与策略层

`internal/config/` 与 `internal/policy/` 分别负责：

- 配置加载优先级
- 环境变量覆盖
- 配置/策略校验

### 输出层

`internal/output/` 负责统一 envelope 与多格式渲染：

- `table`
- `json`
- `markdown`
- `csv`

### 存储与审计层

`internal/storage/` 和 `internal/audit/` 负责落盘能力：

- `snapshots/`
- `monitor/`
- `reports/`
- `audit/`

真实写操作会在 `audit/` 下写入 JSONL 审计日志。

## 关键流程

### doctor

`doctor all` 会并发采集 `port/cpu/mem/disk`，然后统一执行规则评估、评分和结构化输出。

### 写操作

真实写操作流程统一为：

1. 参数校验
2. dry-run 计划或显式 apply
3. 执行结果封装
4. 审计落盘

### snapshot/report

快照负责“记录某一时刻的状态”，报告负责“在时间窗口内总结状态”。

两者共用 storage 与领域模型，但职责分离：

- `snapshot` 关注采集与对比
- `report` 关注汇总与导出

## 测试与契约

P0 收口后，设计上把以下内容视为协议而不是实现细节：

- JSON 顶层 envelope
- `Issue`/`SkippedModule` 字段集合
- 命令树路径集合
- 帮助文案中的 `Long/Example`
- 危险操作帮助中对 `--apply --yes` 与审计的说明

这些内容已经由 `internal/cli` 下的集成测试与契约测试直接锁定。

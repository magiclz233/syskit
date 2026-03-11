# syskit 产品方案与需求文档（PRD v1.3）

## 1. 文档信息
- 产品名称：`syskit`
- 文档版本：`v1.3`
- 文档日期：`2026-03-11`
- 当前阶段：`MVP 立项与需求冻结`
- 目标读者：产品、研发、测试、运维、DevOps

## 2. 产品定位
`syskit` 是一个面向开发者和中小团队运维场景的本地系统诊断与修复 CLI 工具集，提供“检查（Inspect）-诊断（Doctor）-修复（Fix）-巡检（Monitor）-快照（Snapshot）-报告（Report）”闭环能力。

核心价值：
1. 用户即使不知道系统哪里有问题，也能通过一条命令得到优先级明确的问题清单。
2. 比原生命令组合（`netstat`、`tasklist`、`top`、`lsof`）更快完成根因定位。
3. 默认只读、可审计、可脚本化，适合个人排障与团队标准化巡检。

### 2.1 快速参考
| 命令 | 用途 | 典型场景 |
|---|---|---|
| `syskit doctor all` | 一键体检并输出问题清单 | 不知道哪里有问题 |
| `syskit port 8080` | 查询端口占用链路 | 服务启动失败 |
| `syskit top` | 查看 CPU Top 进程 | 机器变慢 |
| `syskit disk` | 查看磁盘风险与增长 | 空间告急 |
| `syskit fix port 8080 --apply` | 释放端口占用 | 快速恢复服务 |

## 3. 产品边界
### 3.1 目标
1. 一键体检：`doctor all` 输出可执行的问题列表。
2. 高频排障闭环：端口冲突、资源异常、磁盘告急可在 3 分钟内完成定位到修复建议。
3. 支持 JSON 输出与退出码，便于 CI/CD 接入。
4. 提供轻量历史数据和趋势能力，支持基础预警。

### 3.2 非目标（YAGNI）
1. 不做远程主机集中管控平台。
2. 不替代重型监控平台（Prometheus/Grafana）。
3. 不做内核级全量性能分析平台。
4. 本版本不引入插件系统。

## 4. 竞品与差异化
### 4.1 对比对象
1. `htop/btop`：实时资源监控。
2. `glances`：跨平台系统监控。
3. `netstat/ss/lsof`：网络与进程原生命令。

### 4.2 差异化
1. 场景化诊断：不是只展示指标，而是输出“问题 + 证据 + 建议 + 修复命令”。
2. 一键体检：可直接生成优先级排序问题清单和健康分。
3. 可自动化：统一 JSON 输出和退出码，天然适配流水线。
4. 安全修复：`dry-run` 默认开启，危险动作有分级确认。

## 5. 用户与场景
### 5.1 目标用户
1. 开发者：本机服务启动失败、端口冲突、性能异常排障。
2. 测试工程师：环境稳定性验证与故障复现对比。
3. 小团队运维/DevOps：定时巡检、快速诊断、统一报告。

### 5.2 核心场景（按优先级）
1. 端口冲突（最高频）：服务启动失败，用户不知道谁占用了端口。
2. 机器变慢：CPU/内存/IO 任一维度异常，用户无法快速识别主因进程。
3. 磁盘告急：空间快速增长，不知道是日志、缓存还是大文件导致。
4. 发布前后对比：需要用快照快速识别系统状态差异。

### 5.3 用户故事（User Story）
1. 作为开发者，我想执行一条命令看到端口冲突根因，以便立即恢复本地调试。
2. 作为测试工程师，我想导出结构化体检报告，以便附在缺陷单中复现环境问题。
3. 作为运维，我想在 CI 中阻断高风险构建，以便避免将异常环境发布到后续环节。
4. 作为团队负责人，我想统一巡检策略配置，以便新成员也能按同一标准排障。

## 6. 产品原则（KISS / YAGNI / DRY / SOLID）
1. KISS：高频路径优先，命令简洁，默认参数可直接使用。
2. YAGNI：先完成 P0 闭环，不预埋低频复杂能力。
3. DRY：采集、规则计算、输出格式统一复用。
4. SOLID：采集器、分析器、执行器分层解耦，平台实现隔离。

## 7. 命令架构与信息架构
### 7.1 主命令分组
1. `inspect`：只读检查。
2. `doctor`：场景化诊断。
3. `fix`：自动修复（默认 dry-run）。
4. `monitor`：持续监控与趋势采样。
5. `snapshot`：快照创建与对比。
6. `report`：报告导出。
7. `policy`：策略与阈值规则执行。

### 7.2 高频短命令别名（优化易用性）
1. `syskit port 8080` = `syskit inspect port 8080`
2. `syskit ports` = `syskit inspect ports`
3. `syskit top` = `syskit inspect proc top --by cpu`
4. `syskit mem` = `syskit inspect mem`
5. `syskit disk` = `syskit inspect disk`

说明：保留分组命令保证结构清晰；高频场景提供短命令提升效率。

## 8. 核心入口：一键体检 `doctor all`
### 8.1 功能定义
用户执行 `syskit doctor all` 后，系统自动完成多维检查，输出问题列表、健康分、优先级建议。

### 8.2 运行模式
1. `--mode quick`：30-60 秒，覆盖核心高频风险。
2. `--mode deep`：深度扫描，包含日志与文件趋势分析。

### 8.3 输出要求
每条问题必须包含：
1. `rule_id`（如 `PORT-001`）
2. `severity`（`critical/high/medium/low`）
3. `summary`
4. `evidence`
5. `impact`
6. `suggestion`
7. `fix_command`
8. `auto_fixable`
9. `confidence`（0-100）

### 8.4 端口冲突场景（重点）
诊断链路必须覆盖：
1. 端口 -> PID -> 进程名 -> 启动命令 -> 父进程 -> 启动时间。
2. 识别是否属于常见开发工具残留（如旧调试实例）。
3. 内置常见开发进程名识别列表（如 `node`、`java`、`python`、`dotnet`、`go`），并在结果中标注进程类型（开发工具/系统服务/未知）。
4. 输出两种路径：
- 保守路径：改服务端口。
- 直接路径：安全终止占用进程（可选）。

## 9. Doctor 场景定义
### 9.1 P0 场景
1. `syskit doctor port --port <port>`
2. `syskit doctor cpu`
3. `syskit doctor mem`
4. `syskit doctor io`
5. `syskit doctor disk`
6. `syskit doctor network`
7. `syskit doctor all`

### 9.2 说明
1. 原 `doctor slowness` 拆分为 `cpu/mem/io`，减少语义宽泛问题。
2. `mem-leak` 调整到 `monitor` 模块（需时序数据，不属于即时诊断）。

## 10. 功能需求（MVP 聚焦版）
### 10.1 Inspect
1. 端口占用与监听列表。
2. 进程资源 TopN 与进程树。
3. CPU/内存/磁盘基础状态。
4. 网络连接与按进程聚合。
5. 文件能力：大文件、大目录、重复文件、类型统计。

### 10.2 Fix
1. `fix port <port>`：释放端口。
2. `fix cleanup --target temp,logs`：清理临时与日志。
3. 默认 `--dry-run`，执行需 `--apply`。
4. 影响范围分级：
- `local`：仅当前用户可见影响，需 `--apply`。
- `system`：系统级变更，需 `--apply` 且具备管理员/root 权限。
- `destructive`：不可逆操作，需 `--apply --yes` 二次确认。

### 10.3 Monitor / Snapshot / Report
1. `monitor`：采样 CPU/内存/磁盘/网络。
2. `snapshot`：生成一次系统状态快照并支持 diff。
3. `report`：导出 table/json/markdown。

## 11. 规则库规划
### 11.1 P0 核心 10 条
1. `PORT-001` 关键端口冲突（critical）
2. `PORT-002` 非预期进程监听公网地址（high）
3. `PROC-001` CPU 高占用持续超阈值（high）
4. `MEM-001` 可用内存低于阈值（high）
5. `DISK-001` 分区使用率超阈值（critical）
6. `DISK-002` 磁盘增长速度异常（high）
7. `FILE-001` 日志文件异常膨胀（high）
8. `NET-001` 连接数异常突增（high）
9. `SVC-001` 关键服务未运行（critical）
10. `ENV-001` PATH 冲突或重复（medium）

### 11.2 扩展目标
后续版本扩展至 20 条规则，先验证核心规则准确率与误报率。

## 12. 健康分计算规则
### 12.1 基础公式
`health_score = max(0, 100 - Σ(rule_penalty * confidence_weight * scope_weight))`

### 12.2 扣分建议
1. `critical`：20 分/条
2. `high`：10 分/条
3. `medium`：5 分/条
4. `low`：2 分/条

### 12.3 加权规则
1. `confidence_weight`：`confidence / 100`。
2. `scope_weight`：
- `local`（当前用户范围）= 1.0
- `system`（系统级影响）= 1.2
- `destructive`（不可逆风险）= 1.5

### 12.4 分区解释
1. 90-100：健康
2. 70-89：可用但有风险
3. 40-69：高风险
4. 0-39：严重异常

## 13. 输出规范与字段映射
### 13.1 终端输出（人类友好）
展示：级别、摘要、关键证据、建议动作、可执行命令。

### 13.2 JSON 输出（机器友好）
统一字段：
- `timestamp`
- `host`
- `module`
- `rule_id`
- `severity`
- `summary`
- `evidence`
- `impact`
- `suggestion`
- `fix_command`
- `auto_fixable`
- `confidence`
- `scope`

### 13.3 映射关系
1. 终端 `[HIGH] PORT-001 端口冲突` -> JSON: `severity=high, rule_id=PORT-001, summary=端口冲突`
2. 终端 `Evidence:` -> JSON: `evidence`
3. 终端 `Fix:` -> JSON: `fix_command`

### 13.4 退出码
1. `0`：执行成功且无高风险问题
2. `1`：执行失败
3. `2`：执行成功但发现高风险问题（用于 CI 阻断）

## 14. 数据存储与保留策略
### 14.1 存储方案
1. P0 采用“文件优先”策略，不引入 SQLite。
2. 快照与监控数据使用 JSONL/JSON 文件存储。

### 14.2 存储路径
1. 系统级：
- Linux/macOS: `/var/lib/syskit/`
- Windows: `%ProgramData%/syskit/`
2. 用户级：
- Linux/macOS: `~/.local/share/syskit/`
- Windows: `%LOCALAPPDATA%/syskit/`

### 14.3 数据保留
1. 默认保留 14 天。
2. 默认最多 500MB，超出按时间滚动删除最旧数据。
3. 支持配置覆盖：`retention_days`、`max_storage_mb`。

## 15. 配置体系与优先级
### 15.1 配置文件
1. 系统级：`/etc/syskit/config.yaml` 或 `%ProgramData%/syskit/config.yaml`
2. 用户级：`~/.config/syskit/config.yaml` 或 `%APPDATA%/syskit/config.yaml`
3. 策略文件：`policy.yaml`（默认同用户配置目录，可通过 `--policy` 指定）

### 15.2 优先级
`命令行参数 > 用户配置 > 系统配置 > 内置默认值`

### 15.3 最小配置示例
```yaml
output: table
retention_days: 14
max_storage_mb: 500
risk:
  require_confirm_for:
    - destructive
```

## 16. 权限模型与降级策略
### 16.1 权限等级
1. 普通用户：可执行绝大多数 inspect/doctor/read-only monitor。
2. 管理员/root：执行系统级采集与所有 fix 动作。

### 16.2 需要提升权限的典型命令
1. `fix` 全部动作。
2. `inspect startup`（部分平台）
3. `inspect service`（部分平台）
4. 读取受限系统日志（部分平台）

### 16.3 降级策略
1. 无权限模块标记为 `skipped(permission_denied)`。
2. 报告中保留“未检查项”清单，不静默忽略。
3. 健康分计算只基于已检查项，并给出 `coverage`（覆盖率）。
4. `coverage = 已检查模块数 / 总模块数 * 100%`。

## 17. 技术选型
### 17.1 语言与版本
1. 开发语言：Go
2. 最低版本：`Go 1.25.0`（与当前项目一致）

### 17.2 核心依赖建议
1. CLI 框架：`cobra`
2. 指标采集：`gopsutil`
3. 配置管理：`gopkg.in/yaml.v3`（P0 默认）；如后续需要复杂配置源合并与环境变量自动绑定，再引入 `viper`
4. 结构化日志：`slog`（标准库）

### 17.3 跨平台策略
1. 默认不使用 cgo，优先纯 Go 与系统命令桥接。
2. 平台差异通过 `platform/windows`、`platform/linux`、`platform/darwin` 适配层隔离。
3. 若平台特性必须依赖原生命令，统一封装并标准化输出。

## 18. 日志分析范围边界
### 18.1 P0 支持范围
1. 系统日志：
- Linux: journald/syslog（按可用性选择）
- Windows: Event Log（系统、应用）
2. 应用日志：仅支持“文本行日志 + 时间戳”通用格式目录扫描。

### 18.2 非范围
1. 不承诺支持所有私有日志格式解析。
2. 不做复杂日志语义解析平台。

## 19. CI/CD 集成示例
### 19.1 GitHub Actions 示例
```yaml
name: syskit-health-check
on: [push, pull_request]

jobs:
  health-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run syskit doctor
        run: |
          syskit doctor all --mode quick --json > health.json
          code=$?
          if [ $code -eq 2 ]; then
            echo "High risk issues detected"
            cat health.json
            exit 1
          fi
          exit $code
```

### 19.2 集成约定
1. 退出码 `2` 视为质量门禁失败。
2. 将 `health.json` 作为构建产物归档。

## 20. 非功能需求
1. 性能：`doctor all --mode quick` 在常见开发机 60 秒内完成。
2. 稳定性：单模块失败不影响整体报告输出；失败模块记录为 `error(reason)` 并在最终结果汇总失败清单。
3. 可维护性：采集、分析、执行模块解耦，具备单元测试接口。
4. 安全性：默认只读，所有写操作有确认机制和审计记录。
5. 可移植性：Windows/Linux/macOS 三平台支持并明确差异提示。

## 21. 版本规划
### 21.1 P0（MVP）
1. 一键体检 `doctor all`（quick）。
2. 核心 doctor 子场景：`port/cpu/mem/io/disk/network`。
3. inspect 核心模块与高频别名命令。
4. fix 两项动作：端口释放、日志/临时清理。
5. snapshot/report 基础能力。

### 21.2 P1
1. `doctor all --mode deep`。
2. monitor 与 policy 告警联动。
3. 规则库从 10 条扩展到 20 条。

### 21.3 P2（待评审）
1. 团队规则包分发能力。
2. 容器诊断增强。

## 22. 验收标准
1. 用户未知问题前提下，执行一次 `doctor all` 可得到按严重级别排序的问题清单。
2. 每条问题包含证据、影响、建议、修复命令。
3. 端口冲突场景可输出完整链路并提供两种可执行处理路径。
4. 具备 JSON 输出与 CI 阻断能力。
5. 权限不足时明确标注未检查项和覆盖率，不误导用户。

## 23. 风险与应对
1. 跨平台口径差异：通过适配层统一字段和行为定义。
2. 自动修复误操作：默认 dry-run + 分级确认 + 审计日志。
3. 深度扫描耗时：quick/deep 分层 + 进度反馈。

## 24. 外部评审建议取舍（最终结论）
### 24.1 已采纳
1. 增补技术选型、数据存储、权限模型、配置优先级、健康分公式。
2. 增加高频短命令别名，保留分组命令结构。
3. `doctor slowness` 拆分为 `doctor cpu/mem/io`。
4. `mem-leak` 从 doctor 移到 monitor。
5. 增加终端输出与 JSON 字段映射。
6. 增加 CI/CD 集成示例与退出码约定。
7. 增加竞品对比、用户故事、日志分析边界。
8. 插件系统移出当前版本范围。
9. 规则库调整为“P0 10 条，后续扩展到 20 条”。

### 24.2 部分采纳
1. “PRD 功能清单过长”建议：PRD 仍保留完整功能清单，用于立项评审与工作量评估；详细命令参数规范与架构设计在后续专项文档展开。

### 24.3 未采纳
1. 完全取消 `inspect` 分组，仅保留平铺命令：未采纳。原因是可维护性和可发现性会下降，最终采用“分组主命令 + 高频别名”的折中方案。

---

如果本 PRD 确认，下一步输出 2 份配套文档：
1. `docs/SYSKIT_CLI_SPEC.md`（命令参数与返回码完整规范）
2. `docs/SYSKIT_ARCHITECTURE.md`（模块目录、接口定义、平台适配设计）

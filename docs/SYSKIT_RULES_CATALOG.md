# syskit 规则目录（对齐版）

## 1. 文档定位

- 文档版本：v3.0
- 更新日期：2026-03-12
- 主文档来源：`docs/PRD.md`
- 对齐文档：
  - `docs/SYSKIT_PRODUCT_PRD.md`
  - `docs/SYSKIT_CLI_SPEC.md`
  - `docs/SYSKIT_ARCHITECTURE.md`

本文件是规则真相源，负责：

1. 规则 ID、阶段、严重级别。
2. 触发条件、证据结构、建议动作。
3. 排除条件、误报控制、最小测试用例。

## 2. 通用约定

### 2.1 统一输出字段

每条规则命中后必须输出：

1. `rule_id`
2. `severity`
3. `summary`
4. `evidence`
5. `impact`
6. `suggestion`
7. `fix_command`
8. `auto_fixable`
9. `confidence`
10. `scope`

### 2.2 通用约束

1. `severity` 取值：`critical/high/medium/low`。
2. `scope` 取值：`local/system/destructive`。
3. 规则阈值优先读取 config，再叠加 policy override。
4. 每条规则至少定义 1 个排除条件。
5. 每条规则至少定义 1 个最小测试用例。
6. 证据必须可追溯，禁止只给结论不给证据。

### 2.3 阶段定义

| 阶段 | 说明 |
|---|---|
| P0 | MVP 必须落地，服务于核心诊断闭环 |
| P1 | 扩展能力，依赖 network/service/startup/log/monitor |
| P2 | 高级场景，依赖后续插件或容器诊断 |

## 3. 规则索引

| 规则 ID | 名称 | 默认严重级别 | 阶段 |
|---|---|---|---|
| `PORT-001` | 关键端口冲突 | critical | P0 |
| `PORT-002` | 非预期进程监听公网地址 | high | P0 |
| `PROC-001` | 进程 CPU 高占用持续超阈值 | high | P0 |
| `PROC-002` | 进程内存占用持续异常 | high | P0 |
| `CPU-001` | 系统 CPU 负载异常 | high | P0 |
| `MEM-001` | 可用内存低于阈值 | high | P0 |
| `DISK-001` | 分区使用率超阈值 | critical | P0 |
| `DISK-002` | 磁盘增长速度异常 | high | P0 |
| `FILE-001` | 大文件或日志膨胀异常 | high | P0 |
| `ENV-001` | PATH 冲突或重复 | medium | P0 |
| `NET-001` | 连接数异常突增 | high | P1 |
| `SVC-001` | 关键服务未运行 | critical | P1 |
| `STARTUP-001` | 可疑开机启动项 | medium | P1 |
| `LOG-001` | 错误日志异常增长 | high | P1 |

## 4. P0 规则定义

### 4.1 PORT-001 关键端口冲突

- 严重级别：`critical`
- 触发条件：
  1. 被关注端口已被占用。
  2. 占用进程不在允许清单内，或同一端口存在异常监听冲突。
- evidence：
  - `port`
  - `pid`
  - `process_name`
  - `command`
  - `parent_process`
  - `start_time`
  - `process_type`
- 影响：目标服务无法启动、冲突端口不可用。
- 建议动作：
  1. 修改目标服务端口。
  2. 确认后释放占用进程。
- fix_command：`syskit port kill <port> --apply`
- auto_fixable：`true`
- scope：`local`
- 排除条件：
  1. 端口由策略允许的长期服务占用。
  2. 系统关键进程占用，仅告警不建议自动结束。
- 最小测试用例：
  1. 旧 `node` 进程占用 `8080`，新服务启动失败，应命中。
  2. `80` 端口被系统 Web 服务占用，应告警但禁用强杀建议。

### 4.2 PORT-002 非预期进程监听公网地址

- 严重级别：`high`
- 触发条件：
  1. 进程监听 `0.0.0.0` 或 `::`。
  2. 进程不在策略允许的公网监听清单中。
- evidence：`pid` `process_name` `local_address` `port` `command`
- 影响：本机服务暴露到公网，存在安全风险。
- 建议动作：绑定到 `127.0.0.1` 或调整防火墙策略。
- fix_command：`syskit service restart <name> --apply` 或人工调整配置。
- auto_fixable：`false`
- scope：`system`
- 排除条件：
  1. 网关、反向代理、数据库代理等显式允许服务。
  2. 受控环境且策略文件明确放行。
- 最小测试用例：
  1. 本地调试服务监听 `0.0.0.0:3000`，应命中。
  2. 白名单中的 `nginx` 监听 `0.0.0.0:80`，不命中。

### 4.3 PROC-001 进程 CPU 高占用持续超阈值

- 严重级别：`high`
- 触发条件：
  1. 进程 CPU 使用率超过阈值。
  2. 持续时间超过配置窗口。
- evidence：`pid` `process_name` `cpu_percent` `threshold` `duration_sec` `command`
- 影响：机器卡顿、系统响应下降。
- 建议动作：降低并发、停止异常任务或结束进程。
- fix_command：`syskit proc kill <pid> --apply`
- auto_fixable：`true`
- scope：`local`
- 排除条件：
  1. 编译、压测、批处理等计划内高负载任务。
  2. 持续时间未达到阈值窗口。
- 最小测试用例：
  1. 非白名单进程 CPU 95% 持续 30 秒，应命中。
  2. `go test` 短时冲高但 10 秒内恢复，不命中。

### 4.4 PROC-002 进程内存占用持续异常

- 严重级别：`high`
- 触发条件：
  1. 进程 RSS 或 VMS 超过阈值。
  2. 且在监控窗口内持续增长或 Swap 占用异常。
- evidence：`pid` `process_name` `rss_mb` `vms_mb` `swap_mb` `growth_rate_mb_per_min`
- 影响：系统可用内存下降，可能诱发 OOM。
- 建议动作：分析进程用途、重启异常进程、检查泄漏。
- fix_command：`syskit proc kill <pid> --apply`
- auto_fixable：`true`
- scope：`local`
- 排除条件：
  1. 明确的大内存数据处理任务。
  2. 人工确认中的压力测试环境。
- 最小测试用例：
  1. 进程 RSS 持续上涨且 30 分钟增长超阈值，应命中。
  2. 大对象加载后稳定不再增长，不命中。

### 4.5 CPU-001 系统 CPU 负载异常

- 严重级别：`high`
- 触发条件：
  1. 1 分钟平均负载大于 `CPU 核心数 * 2`。
  2. 或总 CPU 使用率在窗口期内持续超阈值。
- evidence：`load1` `load5` `load15` `cpu_cores` `usage_percent` `top_processes`
- 影响：整体系统吞吐下降，交互变慢。
- 建议动作：优先定位高负载进程，必要时调整服务并发。
- fix_command：空，由建议驱动。
- auto_fixable：`false`
- scope：`system`
- 排除条件：
  1. 已批准的性能压测窗口。
  2. 构建服务器处于预期峰值阶段。
- 最小测试用例：
  1. 8 核机器 `load1=18` 持续 1 分钟，应命中。
  2. 短时波动后恢复，不命中。

### 4.6 MEM-001 可用内存低于阈值

- 严重级别：`high`
- 触发条件：
  1. `available_mb` 低于阈值。
  2. 或 `usage_percent` 和 `swap_usage_percent` 同时超过阈值。
- evidence：`total_mb` `available_mb` `usage_percent` `swap_usage_percent`
- 影响：系统性能下降，容易触发 OOM 或频繁交换。
- 建议动作：关闭高占用进程、清理缓存、评估扩容。
- fix_command：空，由人工处理或配合 `proc kill`。
- auto_fixable：`false`
- scope：`system`
- 排除条件：
  1. 缓存可快速回收的短时波动。
  2. 人工确认的压测环境。
- 最小测试用例：
  1. 可用内存低于 10% 且 swap 超 50%，应命中。
  2. 瞬时抖动并自动恢复，不命中。

### 4.7 DISK-001 分区使用率超阈值

- 严重级别：`critical`
- 触发条件：关键分区使用率超过阈值，或剩余空间低于安全线。
- evidence：`mount_point` `usage_percent` `free_gb` `inode_usage_percent`
- 影响：写入失败、日志中断、服务不可用。
- 建议动作：定位大文件、清理临时目录、归档旧日志。
- fix_command：`syskit fix cleanup --apply`
- auto_fixable：`true`
- scope：`system`
- 排除条件：
  1. 非业务关键挂载点且被策略忽略。
  2. 只读镜像分区。
- 最小测试用例：
  1. `/var` 使用率 92%，应命中。
  2. 忽略列表中的挂载点 88%，不命中。

### 4.8 DISK-002 磁盘增长速度异常

- 严重级别：`high`
- 触发条件：
  1. 分区日增长率超过阈值。
  2. 或显著高于历史基线。
- evidence：`mount_point` `growth_rate_gb_per_day` `baseline_gb_per_day` `window_days`
- 影响：短期内磁盘可能写满。
- 建议动作：定位新增大文件、检查日志轮转和缓存策略。
- fix_command：`syskit disk scan <path>` 或 `syskit file archive <path> --apply`
- auto_fixable：`false`
- scope：`system`
- 排除条件：
  1. 数据导入、镜像拉取等计划内增长。
  2. 样本窗口过短。
- 最小测试用例：
  1. 日增长 15GB，历史基线 2GB，应命中。
  2. 单日突增后恢复正常，不命中。

### 4.9 FILE-001 大文件或日志膨胀异常

- 严重级别：`high`
- 触发条件：
  1. 单文件体积超过阈值。
  2. 或日志文件增长速度超过阈值。
- evidence：`file_path` `size_mb` `growth_mb_per_hour` `last_modified`
- 影响：占满磁盘，影响服务写入。
- 建议动作：归档旧日志、启用轮转、排查异常输出。
- fix_command：`syskit file archive <path> --apply` 或 `syskit fix cleanup --apply`
- auto_fixable：`true`
- scope：`local`
- 排除条件：
  1. 明确的备份文件、镜像文件且位于允许目录。
  2. 轮转后的压缩文件。
- 最小测试用例：
  1. `app.log` 2 小时增长 8GB，应命中。
  2. 体积大但位于明确允许的备份目录，不命中。

### 4.10 ENV-001 PATH 冲突或重复

- 严重级别：`medium`
- 触发条件：
  1. PATH 存在重复项。
  2. 同类工具链路径顺序冲突导致解析歧义。
- evidence：`duplicates` `conflicts` `path_length`
- 影响：命令解析不稳定，工具链版本混乱。
- 建议动作：删除重复项、调整版本优先顺序。
- fix_command：空，由人工调整环境变量。
- auto_fixable：`false`
- scope：`local`
- 排除条件：
  1. 用户通过策略显式允许多版本并存。
  2. 临时会话 PATH 修改不写回系统。
- 最小测试用例：
  1. PATH 中同一路径重复三次，应命中。
  2. 白名单里的多版本工具链顺序，不命中。

## 5. P1 规则定义

### 5.1 NET-001 连接数异常突增

- 严重级别：`high`
- 触发条件：连接总数超过阈值，或较基线突增达到倍率阈值。
- evidence：`connection_count` `baseline_count` `top_remote_hosts` `top_processes`
- 影响：可能存在流量异常、连接泄漏或攻击面暴露。
- 建议动作：检查远端流量、限流、定位异常进程。
- fix_command：空，由人工决策。
- auto_fixable：`false`
- scope：`system`
- 排除条件：发布窗口、批量同步、压测时段。
- 最小测试用例：连接数从 200 突增到 2500，应命中。

### 5.2 SVC-001 关键服务未运行

- 严重级别：`critical`
- 触发条件：策略定义的关键服务未运行或状态异常。
- evidence：`service_name` `expected_state` `actual_state` `check_time`
- 影响：核心能力不可用。
- 建议动作：查看日志、确认依赖、重启服务。
- fix_command：`syskit service restart <name> --apply`
- auto_fixable：`true`
- scope：`system`
- 排除条件：
  1. 当前平台不支持该服务名。
  2. 服务被标记为可选。
- 最小测试用例：关键服务 `docker` 未启动，应命中。

### 5.3 STARTUP-001 可疑开机启动项

- 严重级别：`medium`
- 触发条件：启动项缺少签名、路径可疑、发布者未知或位于临时目录。
- evidence：`startup_id` `name` `path` `publisher` `signed` `user`
- 影响：开机性能下降，存在潜在风险程序。
- 建议动作：人工确认后禁用或清理。
- fix_command：`syskit startup disable <id> --apply --yes`
- auto_fixable：`true`
- scope：`system`
- 排除条件：
  1. 团队已知的内部脚本启动项。
  2. 临时测试条目已在策略允许清单中。
- 最小测试用例：临时目录下的未知发布者启动项，应命中。

### 5.4 LOG-001 错误日志异常增长

- 严重级别：`high`
- 触发条件：错误日志增长速度或错误率超过阈值。
- evidence：`file_path` `error_rate` `growth_mb_per_hour` `sample_errors`
- 影响：磁盘占用升高，系统稳定性下降，问题持续扩大。
- 建议动作：聚类错误、修复根因、启用归档和轮转。
- fix_command：`syskit log search error` 或 `syskit file archive <path> --apply`
- auto_fixable：`false`
- scope：`system`
- 排除条件：
  1. 一次性故障复盘期间的临时 debug 日志。
  2. 已知回放任务造成的短期错误峰值。
- 最小测试用例：24 小时内错误率翻倍且日志增量超阈值，应命中。

## 6. 规则维护要求

1. 规则 ID 一旦发布不得随意重命名。
2. 规则字段和 CLI JSON 协议保持兼容，只允许新增可选字段。
3. 阈值或 evidence 结构调整时，必须同步更新测试用例。
4. 规则阶段调整必须同步更新产品 PRD 的版本规划。
5. 新增规则前先判断是否已有规则可覆盖，避免重复定义。
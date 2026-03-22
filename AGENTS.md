# 项目级开发约束

## 注释要求

1. 后续新增或明显改动的 Go 代码，默认补充简体中文注释。
2. 新增 package、struct、interface、重要常量块、导出函数，优先补充注释。
3. 非直观的分支、优先级合并、协议封装、路径解析、降级处理等逻辑，必须解释"为什么这样做"，不要只重复代码表面行为。
4. 注释应和代码保持同步；如果实现变了，注释也必须一起更新。
5. 对极其直白的一行赋值或 getter，不要求强行加注释；重点放在边界、约束、协议和设计意图。

## Go 代码规范

1. 保持职责清晰：
   - `cmd/syskit/` 只负责二进制入口和退出码返回，不承载命令定义。
   - `internal/cli/` 负责 Cobra 命令树注册、参数绑定、帮助文案和黑盒集成/契约测试。
   - `internal/cliutil/` 存放 CLI 共享工具（pending 命令、扫描入口适配等）。
   - `internal/collectors/` 实现端口、进程、CPU、内存、磁盘等采集，只返回结构化数据，不负责 CLI 展示。
   - `internal/domain/` 负责规则、评分、doctor/snapshot/report 领域模型，不依赖 CLI 和平台层。
   - `internal/config/` 负责配置加载、环境变量覆盖和基础校验。
   - `internal/policy/` 负责策略文件模型与校验。
   - `internal/output/` 负责统一 envelope 与 table/json/markdown/csv 多格式渲染。
   - `internal/storage/` 负责 `data_dir` 布局、保留策略和快照/报告目录管理。
   - `internal/audit/` 负责真实写操作（`port kill`、`proc kill`、`fix cleanup`、`snapshot delete`）的 JSONL 审计日志。
   - `internal/errs/` 提供统一错误封装，所有结构化错误优先走此包。
2. 优先复用已有 helper，避免在多个命令里复制 flag 解析、输出写入、扫描执行等相同逻辑。
3. 错误优先走 `internal/errs` 统一封装；结构化输出优先走 `internal/output`。
4. 新增配置、策略、协议字段时，同时补齐 `yaml/json` tag、基础校验和必要注释。
5. 提交前至少运行 `gofmt -w cmd internal pkg`；涉及功能逻辑改动时，运行 `go test ./...` 做基本验证。
6. 除非文件已有其他风格，新增注释和说明文字统一使用简体中文。

## 写操作行为约束

1. `port kill`、`proc kill`、`fix cleanup`、`snapshot delete` 等写操作默认 dry-run，不得静默执行。
2. 真实执行必须显式传入 `--apply --yes`；危险操作在帮助文案中必须说明审计日志行为。
3. 权限不足时必须写入 `skipped`，不能静默跳过。
4. 真实写操作执行后必须写入 `internal/audit/` 下的 JSONL 审计日志。

## 新增或修改命令的工作流

1. **先改规范**：在 `docs/SYSKIT_CLI_SPEC.md` 中更新命令树或参数协议。
2. **再落代码**：在 `internal/cli/<module>/` 中实现命令与 presenter。
3. **补齐写操作**：为真实写操作补齐 dry-run、`--apply --yes` 和审计日志。
4. **补齐测试**：为结构化输出补齐 JSON 契约或帮助契约测试。
5. **同步文档**：同步更新 `README.md`、`docs/QUICKSTART.md`、`scripts/README.md`。

## 测试分层要求

| 层级 | 范围 | 约束 |
|---|---|---|
| 单元测试 | 配置合并、规则判断、评分逻辑、输出渲染、参数约束 | 必须覆盖边界条件 |
| 黑盒集成测试 | `doctor all`、`disk scan`、`fix cleanup` dry-run/apply、`snapshot *`、`policy validate`、`proc kill`/`port kill` dry-run/apply | 真实写操作必须先验证 dry-run |
| 契约测试 | JSON envelope 与关键字段集合、命令树路径集合、`Long/Example` 与危险操作帮助文本 | 字段只增不减 |

相关测试文件：`internal/cli/contract_test.go`、`internal/cli/integration_test.go`。

## 文档同步约束

以下文档必须一致维护，避免命令面漂移：

- `docs/SYSKIT_CLI_SPEC.md`（命令协议唯一权威来源）
- `docs/SYSKIT_ARCHITECTURE.md`（模型与接口权威来源）
- `README.md` / `docs/QUICKSTART.md` / `docs/RELEASE_GUIDE.md`
- `scripts/README.md`
- `syskit --help` 实际输出

如果某个命令只是占位（P1/P2），请保持帮助树存在，但执行时返回明确的"尚未开发"提示，不要做半实现。

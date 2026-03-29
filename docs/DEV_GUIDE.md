# 开发说明

## 目标

`syskit` P0 的目标是提供一套可脚本化、可审计、跨平台的本地系统运维 CLI 基线能力，而不是继续扩展旧的单一扫描器入口。

当前正式约束：

- 根命令默认显示帮助，不再直接扫描目录。
- 正式扫描只保留 `syskit disk scan <path>`。
- 写操作默认 dry-run，危险操作要求 `--apply --yes`。
- 命令树中的 P1/P2 命令默认保留帮助与占位；已落地项按开发清单逐步转为正式实现（当前已包含 `port ping/scan`、`net conn/listen/speed`、`dns resolve/bench`、`ping`、`traceroute`、`cpu burst/watch`、`mem leak/watch`、`monitor all`、`service list/check`、`doctor network`、`doctor disk-full`、`doctor slowness`）。

## 目录职责

- `cmd/syskit/`
  - 二进制入口，只负责启动 CLI 和退出码返回。
- `internal/cli/`
  - Cobra 命令树、参数编排、帮助文案、黑盒集成/契约测试。
- `internal/cliutil/`
  - CLI 共享工具，如 pending 命令、扫描入口适配等。
- `internal/collectors/`
  - 端口、进程、CPU、内存、磁盘等采集逻辑。
- `internal/domain/`
  - 规则、评分、doctor 结果、snapshot/report 领域模型。
- `internal/config/`
  - 配置加载、环境变量覆盖、基础校验。
- `internal/policy/`
  - 策略文件模型与校验。
- `internal/output/`
  - table/json/markdown/csv 统一输出。
- `internal/storage/`
  - `data_dir` 布局、保留策略、快照/报告目录管理。
- `internal/audit/`
  - `port kill`、`proc kill`、`fix cleanup`、`snapshot delete` 等真实写操作审计。

## 常用命令

```bash
gofmt -w cmd internal pkg
go test ./...
./scripts/build.sh all
./scripts/verify-p0.sh
```

Windows:

```powershell
gofmt -w cmd internal pkg
go test ./...
scripts\build.bat all
scripts\verify-p0.bat
```

## 测试基线

P0 当前测试分为三层：

1. 单元测试
   - 配置合并、规则判断、评分逻辑、输出渲染、参数约束。
2. 黑盒集成测试
   - `doctor all`
   - `disk scan`
   - `fix cleanup` dry-run/apply
   - `snapshot create/list/show/diff/delete`
   - `policy validate`
   - `proc kill` / `port kill` dry-run/apply
3. 契约测试
   - JSON envelope 与关键字段集合
   - 规则字段集合
   - 命令树路径集合
   - `Long/Example` 与危险操作帮助文本

相关测试文件位于 `internal/cli/contract_test.go`、`internal/cli/integration_test.go`、`internal/cli/test_helpers_test.go`。

## 新增或修改命令时的要求

1. 先改 `docs/SYSKIT_CLI_SPEC.md` 中的命令树或协议。
2. 在 `internal/cli/<module>/` 中落地命令与 presenter。
3. 为真实写操作补齐 dry-run、`--apply --yes`、审计日志。
4. 为结构化输出补齐 JSON 契约或帮助契约测试。
5. 同步更新 `README.md`、`docs/QUICKSTART.md`、`scripts/README.md`。

## 配置与数据目录

常用环境变量：

- `SYSKIT_CONFIG`
- `SYSKIT_POLICY`
- `SYSKIT_OUTPUT`
- `SYSKIT_NO_COLOR`
- `SYSKIT_DATA_DIR`
- `SYSKIT_LOG_LEVEL`

`SYSKIT_DATA_DIR` 下会自动维护：

- `snapshots/`
- `monitor/`
- `reports/`
- `audit/`

## 文档同步要求

以下内容必须一起维护，避免命令面漂移：

- `docs/SYSKIT_CLI_SPEC.md`
- `README.md`
- `docs/QUICKSTART.md`
- `docs/RELEASE_GUIDE.md`
- `scripts/README.md`
- `syskit --help` 实际输出

如果某个命令尚未在阶段清单中开发，请保持帮助树占位并返回明确“尚未开发”提示，不要做半实现。


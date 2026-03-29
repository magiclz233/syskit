# syskit

`syskit` 是一个跨平台本地系统运维 CLI，当前已完成 P0 阶段的诊断、扫描、清理、快照、报告和策略基线能力，并开始交付 P1 网络扩展能力。

当前正式可用的 P0 命令包括：

- `doctor all/port/cpu/mem/disk`
- `disk`、`disk scan <path>`
- `port <expr>`、`port list`、`port kill <port>`
- `proc top/tree/info/kill`
- `cpu`、`mem`、`mem top`
- `fix cleanup`
- `snapshot create/list/show/diff/delete`
- `report generate`
- `policy show/init/validate`

当前已落地的 P1 命令包括：

- `port ping <target> <port>`
- `port scan <target>`
- `net conn`
- `net listen`
- `net speed`
- `dns resolve <domain>`
- `dns bench <domain>`
- `ping <target>`
- `traceroute <target>`
- `cpu burst`
- `doctor network`
- `doctor disk-full`
- `doctor slowness`
CLI 帮助树中其余 P1/P2 命令仍以占位形式保留，用于保持命令树、帮助文案和后续扩展路径稳定；未实现命令会明确返回“尚未开发”。

## 关键行为

- `syskit` 无参数默认显示帮助，不再兼容旧的根命令扫描入口。
- 正式扫描入口只有 `syskit disk scan <path>`。
- 写操作默认 `dry-run`，危险操作需要显式传入 `--apply --yes`。
- 结构化输出统一使用 `--format json`，JSON 顶层 envelope 与关键字段已纳入契约测试。
- 审计日志、快照和报告统一落在 `storage.data_dir` 下，可通过 `SYSKIT_DATA_DIR` 覆盖。

## 快速开始

直接查看帮助：

```bash
go run ./cmd/syskit --help
```

执行一次体检：

```bash
go run ./cmd/syskit doctor all --fail-on never
```

扫描当前目录：

```bash
go run ./cmd/syskit disk scan . --limit 20 --format json
```

查看快照与策略：

```bash
go run ./cmd/syskit snapshot list --limit 10
go run ./cmd/syskit policy init --type config --output .syskit/config.yaml
go run ./cmd/syskit policy validate .syskit/config.yaml --type config
```

真实执行写操作前，先看 dry-run：

```bash
go run ./cmd/syskit fix cleanup --target temp --older-than 72h
go run ./cmd/syskit port kill 8080 --apply --yes
```

更多示例见 [快速开始](docs/QUICKSTART.md)。

## 构建与验证

正式支持的构建目标只有 6 个：

- `windows-amd64`
- `windows-arm64`
- `linux-amd64`
- `linux-arm64`
- `darwin-amd64`
- `darwin-arm64`

常用命令：

```bash
gofmt -w cmd internal pkg
go test ./...
./scripts/build.sh all
./scripts/verify-p0.sh
```

Windows：

```powershell
gofmt -w cmd internal pkg
go test ./...
scripts\build.bat all
scripts\verify-p0.bat
```

`verify-p0` 会统一执行全量测试、六目标交叉编译，以及核心 help/smoke 命令。

## 发布

本地发布脚本：

```bash
./scripts/release.sh 0.4.0
```

Windows：

```powershell
scripts\release.bat 0.4.0
```

GitHub Actions 的 [release.yml](.github/workflows/release.yml) 会在推送 `v*` tag 后调用 `scripts/build.sh all`，发布这 6 个正式目标的产物。

发布细节见 [发布说明](docs/RELEASE_GUIDE.md)。

## 项目结构

```text
syskit/
├── cmd/syskit/               # 二进制入口
├── internal/
│   ├── cli/                  # Cobra 命令树、参数编排、帮助文案、集成/契约测试
│   ├── cliutil/              # CLI 共享工具（pending 命令、扫描入口适配等）
│   ├── collectors/           # 端口、进程、CPU、内存、磁盘采集
│   ├── domain/               # 规则、评分、snapshot/report 领域模型
│   ├── config/               # 配置加载与校验
│   ├── policy/               # 策略文件模型与校验
│   ├── output/               # table/json/markdown/csv 统一输出渲染
│   ├── storage/              # data_dir 布局、快照/报告/保留策略
│   ├── audit/                # 写操作 JSONL 审计日志
│   └── errs/                 # 统一错误封装
├── pkg/                      # 可对外暴露的公共包
├── scripts/                  # 构建、验证、发布脚本
└── docs/                     # CLI 规范、架构、开发/发布文档
```

## 退出码

| 退出码 | 含义 | 场景 |
|---|---|---|
| `0` | 成功且未命中阻断阈值 | 无风险或只有可接受问题 |
| `1` | 成功但存在非阻断警告 | 存在 `medium/low` 等非阻断问题 |
| `2` | 成功但命中 `--fail-on` 阈值 | 适用于 CI 阻断 |
| `3` | 参数或配置非法 | 参数错误、配置格式错误、策略格式错误 |
| `4` | 权限不足 | 全局或关键模块需要提升权限 |
| `5` | 执行失败 | 平台不支持、外部命令失败、超时等 |
| `6` | 部分执行成功 | 批量操作部分成功、部分失败 |

## 环境变量

| 变量名 | 说明 |
|---|---|
| `SYSKIT_CONFIG` | 默认配置文件路径 |
| `SYSKIT_POLICY` | 默认策略文件路径 |
| `SYSKIT_OUTPUT` | 默认输出格式 |
| `SYSKIT_NO_COLOR` | 是否禁用颜色输出 |
| `SYSKIT_DATA_DIR` | 数据目录（快照、报告、审计日志） |
| `SYSKIT_LOG_LEVEL` | 日志级别覆盖 |

## 文档

- [快速开始](docs/QUICKSTART.md)
- [开发说明](docs/DEV_GUIDE.md)
- [设计说明](docs/DESIGN.md)
- [架构文档](docs/SYSKIT_ARCHITECTURE.md)
- [CLI 命令规范](docs/SYSKIT_CLI_SPEC.md)
- [规则目录](docs/SYSKIT_RULES_CATALOG.md)
- [发布说明](docs/RELEASE_GUIDE.md)
- [脚本说明](scripts/README.md)
- [P0 平台验证清单](docs/P0_PLATFORM_VERIFICATION.md)



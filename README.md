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
go test ./...
./scripts/build.sh all
./scripts/verify-p0.sh
```

Windows：

```powershell
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
├── internal/cli/             # Cobra 命令树与命令编排
├── internal/collectors/      # 端口、进程、CPU、内存、磁盘采集
├── internal/domain/          # 规则、评分、快照/报告领域模型
├── internal/config/          # 配置加载与校验
├── internal/output/          # 统一输出渲染
├── internal/storage/         # data_dir 布局、快照/报告/保留策略
├── internal/audit/           # 写操作 JSONL 审计
├── scripts/                  # 构建、验证、发布脚本
└── docs/                     # CLI 规范、架构、开发/发布文档
```

## 文档

- [快速开始](docs/QUICKSTART.md)
- [开发说明](docs/DEV_GUIDE.md)
- [设计说明](docs/DESIGN.md)
- [发布说明](docs/RELEASE_GUIDE.md)
- [脚本说明](scripts/README.md)
- [P0 平台验证清单](docs/P0_PLATFORM_VERIFICATION.md)

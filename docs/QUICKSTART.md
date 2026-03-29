# 快速开始

## 1. 前置条件

- Go 版本以 `go.mod` 为准
- 当前工作目录位于仓库根目录
- 如需真实写操作，请提前确认权限和目标对象

## 2. 先看帮助

`syskit` 根命令默认显示帮助，不再兼容旧扫描行为：

```bash
go run ./cmd/syskit --help
go run ./cmd/syskit disk scan --help
```

## 3. 常用 P0 命令

### 3.1 一键体检

```bash
go run ./cmd/syskit doctor all --fail-on never
go run ./cmd/syskit doctor all --fail-on never --format json
```

### 3.2 磁盘总览与目录扫描

```bash
go run ./cmd/syskit disk
go run ./cmd/syskit disk --format json
go run ./cmd/syskit disk scan . --limit 20
go run ./cmd/syskit disk scan . --limit 50 --min-size 100MB --format json
```

说明：

- 正式扫描入口只有 `disk scan`。
- `limit/min-size/depth/exclude/export-csv` 都在 `disk scan` 下维护。

### 3.3 端口与进程

```bash
go run ./cmd/syskit port list
go run ./cmd/syskit port 8080
go run ./cmd/syskit port ping 127.0.0.1 8080 --count 3
go run ./cmd/syskit port scan 127.0.0.1 --port 22,80,443
go run ./cmd/syskit net conn --proto tcp --state established
go run ./cmd/syskit net listen --proto tcp --addr 127.0.0.1
go run ./cmd/syskit net speed --mode full --verbose
go run ./cmd/syskit net speed --server https://speed.cloudflare.com --mode download
go run ./cmd/syskit dns resolve localhost --type A
go run ./cmd/syskit dns bench localhost --count 3 --type A
go run ./cmd/syskit ping localhost --count 2
go run ./cmd/syskit traceroute localhost --max-hops 5
go run ./cmd/syskit cpu burst --interval 200ms --duration 5s --threshold 70
go run ./cmd/syskit cpu watch --interval 1s --top 10 --threshold-cpu 85 --timeout 30s
go run ./cmd/syskit mem leak 1234 --duration 2m --interval 2s
go run ./cmd/syskit mem watch --interval 5s --top 10 --threshold-mem 90 --threshold-swap 50 --timeout 30s
go run ./cmd/syskit monitor all --interval 2s --max-samples 10 --format json
go run ./cmd/syskit monitor all --inspection-interval 1m --inspection-mode deep --inspection-fail-on high --timeout 2m
go run ./cmd/syskit doctor network --target localhost --fail-on never
go run ./cmd/syskit doctor disk-full --path . --top 10 --fail-on never
go run ./cmd/syskit doctor slowness --mode quick --fail-on never
go run ./cmd/syskit proc top --top 10
go run ./cmd/syskit proc info 1234
```

危险操作默认 dry-run，真实执行必须显式确认：

```bash
go run ./cmd/syskit port kill 8080
go run ./cmd/syskit port kill 8080 --apply --yes
go run ./cmd/syskit proc kill 1234 --apply --yes
```

### 3.4 清理

```bash
go run ./cmd/syskit fix cleanup
go run ./cmd/syskit fix cleanup --target temp --older-than 72h
go run ./cmd/syskit fix cleanup --apply --yes --older-than 7d
```

### 3.5 快照与报告

```bash
go run ./cmd/syskit snapshot create --module port,cpu
go run ./cmd/syskit snapshot list --limit 10
go run ./cmd/syskit snapshot show <snapshot-id>
go run ./cmd/syskit report generate --type health --format markdown
```

### 3.6 配置与策略

```bash
go run ./cmd/syskit policy show --type all
go run ./cmd/syskit policy init --type config --output .syskit/config.yaml
go run ./cmd/syskit policy validate .syskit/config.yaml --type config
```

## 4. 结构化输出

所有 P0 命令共用统一输出协议：

```bash
go run ./cmd/syskit doctor all --fail-on never --format json
go run ./cmd/syskit disk scan . --format json
go run ./cmd/syskit snapshot list --format json
```

常用全局参数：

- `--format table/json/markdown/csv`
- `--output <path>`
- `--config <path>`
- `--policy <path>`
- `--fail-on <critical/high/medium/low/never>`
- `--dry-run` / `--apply` / `--yes`

## 5. 本地验证

推荐在开始或收尾时执行统一验证脚本：

Linux/macOS:

```bash
./scripts/verify-p0.sh
```

Windows:

```powershell
scripts\verify-p0.bat
```

脚本会执行：

1. `go test ./...`
2. 六个正式支持目标的交叉编译
3. `--help`、`doctor all`、`disk`、`disk scan`、`snapshot list`、`policy validate` 等 smoke 命令

## 6. 下一步

- 详细开发约定见 [开发说明](DEV_GUIDE.md)
- 架构与设计取舍见 [设计说明](DESIGN.md)
- 发布流程见 [发布说明](RELEASE_GUIDE.md)



# 脚本说明

## 构建脚本

文件：

- `build.sh`
- `build.bat`

正式支持目标：

- `windows-amd64`
- `windows-arm64`
- `linux-amd64`
- `linux-arm64`
- `darwin-amd64`
- `darwin-arm64`

聚合目标：

- `all`
- `windows`
- `linux`
- `darwin`

默认行为：

- `build.sh` 无参数时编译当前宿主机版本，仅支持 `amd64/arm64`
- `build.bat` 无参数时编译当前 Windows 宿主机版本，仅支持 `amd64/arm64`

已下线目标：

- `windows-386`
- `linux-386`
- `linux-arm`

对这些目标执行构建时，脚本会直接返回明确错误。

常用命令：

```bash
./scripts/build.sh
./scripts/build.sh all
./scripts/build.sh linux-arm64
```

Windows：

```powershell
scripts\build.bat
scripts\build.bat all
scripts\build.bat windows-arm64
```

## P0 验证脚本

文件：

- `verify-p0.sh`
- `verify-p0.bat`

用途：

1. 执行 `go test ./...`
2. 编译 6 个正式支持目标
3. 执行核心 help/smoke 命令：
   - `syskit --help`
   - `doctor all --fail-on never --format json`
   - `disk --format json`
   - `disk scan . --limit 3 --format json`
   - `snapshot list --limit 1 --format json`
   - `policy validate <temp-config> --type config --format json`

使用方式：

```bash
./scripts/verify-p0.sh
```

Windows：

```powershell
scripts\verify-p0.bat
```

## 发布脚本

文件：

- `release.sh`
- `release.bat`

作用：

1. 检查工作区是否干净
2. 检查 tag 是否已存在
3. 更新 `internal/version/version.go`
4. 提交版本变更
5. 构建 6 个正式支持目标
6. 创建本地 tag

示例：

```bash
./scripts/release.sh 0.4.0
```

Windows：

```powershell
scripts\release.bat 0.4.0
```

## P1 监控示例

`monitor all` 已转为正式命令，可用于持续监控和定时巡检基线验证：

```bash
go run ./cmd/syskit monitor all --interval 2s --max-samples 5 --format json
go run ./cmd/syskit monitor all --inspection-interval 1m --inspection-mode deep --inspection-fail-on high --timeout 2m
```

## 已移除脚本

旧的 `find-largest-local.ps1` 已删除。

当前所有扫描示例都应改用：

```bash
syskit disk scan <path>
```

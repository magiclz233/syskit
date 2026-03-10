# 脚本说明

## 构建脚本

### 文件

- `build.sh`: Linux/macOS 下使用的构建脚本
- `build.bat`: Windows 下使用的构建脚本

### 支持的平台目标

两个构建脚本支持同一组目标：

- `windows-amd64`
- `windows-386`
- `windows-arm64`
- `linux-amd64`
- `linux-386`
- `linux-arm64`
- `linux-arm`
- `darwin-amd64`
- `darwin-arm64`

聚合目标：

- `all`: 构建全部平台
- `windows`: 构建全部 Windows 目标
- `linux`: 构建全部 Linux 目标
- `darwin`: 构建全部 macOS 目标

默认行为：

- `build.sh` 不带参数时，会自动检测当前宿主机平台并构建当前平台
- `build.bat` 不带参数时，会检测当前 Windows 架构并构建当前 Windows 目标

### 产物命名规则

构建结果统一输出到 `build/` 目录，文件名格式如下：

```text
find-large-files-<os>-<arch>[.exe]
```

示例：

- `build/find-large-files-windows-amd64.exe`
- `build/find-large-files-linux-arm64`
- `build/find-large-files-darwin-arm64`

### 常用命令

Linux/macOS:

```bash
./scripts/build.sh
./scripts/build.sh all
./scripts/build.sh windows
./scripts/build.sh linux-arm64
./scripts/build.sh darwin-arm64
```

Windows:

```powershell
scripts\build.bat
scripts\build.bat all
scripts\build.bat windows
scripts\build.bat linux-arm64
scripts\build.bat darwin-arm64
```

### 什么时候用哪种方式

- 只给自己当前机器用：直接运行默认构建
- 要准备 GitHub Release 资产：运行 `all`
- 只补某个平台：运行对应精确目标，例如 `linux-arm64`

## 发布脚本

### 文件

- `release.sh`
- `release.bat`

### 它们做什么

发布脚本会按顺序执行：

1. 检查工作区是否干净
2. 检查目标 tag 是否已存在
3. 更新 [main.go](/e:/code/golang/find-large-files/main.go) 中的版本号
4. 提交版本更新
5. 构建全部平台产物
6. 创建 Git tag
7. 提示后续 push 和 Release 操作

### 使用方式

Linux/macOS:

```bash
./scripts/release.sh 0.4.0
```

Windows:

```powershell
scripts\release.bat 0.4.0
```

执行后会生成：

- 一个版本提交
- 一个本地 tag，例如 `v0.4.0`
- `build/` 目录下的全部构建产物

### 推送和 GitHub Release

发布脚本不会替你直接推送远端仓库；你需要自行执行：

```bash
git push origin <当前分支>
git push origin v0.4.0
```

或者：

```bash
git push origin <当前分支> --follow-tags
```

推送 tag 后，GitHub Actions 工作流 [.github/workflows/release.yml](/e:/code/golang/find-large-files/.github/workflows/release.yml) 会自动触发，并执行：

1. 检出代码
2. 安装 Go 1.25
3. 运行 `./scripts/build.sh all`
4. 创建 GitHub Release
5. 上传全部构建产物到 Release

### 手动使用 GitHub CLI 发版

如果你不想依赖 Actions，也可以在本地构建后手动创建 Release：

```bash
gh release create v0.4.0 build/* --title "v0.4.0" --notes "Release 0.4.0"
```

前提：

- 已经 `git push` 对应 tag
- 本机已安装并登录 `gh`

## 本地扫描脚本

`find-largest-local.ps1` 是一个 PowerShell 包装器，用于在 Windows 本地快速启动当前项目的准确扫描。

示例：

```powershell
scripts\find-largest-local.ps1
scripts\find-largest-local.ps1 -Path D:\ -Top 30
scripts\find-largest-local.ps1 -Path D:\ -Exclude "node_modules,.git" -ExportCsvPath D:\scan
```

行为：

- 优先调用已编译好的程序
- 如果没找到可执行文件，则回退到 `go run .`
- 与主程序保持相同语义，只做全量准确扫描

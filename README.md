# syskit

一个跨平台本地系统运维 CLI 项目。当前仓库仍以内置的磁盘扫描能力为主，用来准确找出目标目录树中：

- 最大的子目录
- 最大的文件

当前代码实现仍以全量准确扫描为主，后续将按 `docs/SYSKIT_*` 文档逐步扩展为完整的 `syskit` P0 能力。

## 特性

- 全量扫描整个目录树，结果完整
- 同时输出 Top N 子目录和 Top N 文件
- 支持表格、JSON、CSV 输出
- 支持排除指定目录
- 自动跳过无权限路径和符号链接
- Windows、Linux、macOS 可用

## 构建

### 直接编译

```bash
go build -o syskit
```

Windows:

```powershell
go build -o syskit.exe
```

### 使用构建脚本

构建脚本支持的目标如下：

- `windows-amd64`
- `windows-386`
- `windows-arm64`
- `linux-amd64`
- `linux-386`
- `linux-arm64`
- `linux-arm`
- `darwin-amd64`
- `darwin-arm64`
- 聚合目标：`all`、`windows`、`linux`、`darwin`

Linux/macOS:

```bash
./scripts/build.sh
./scripts/build.sh all
```

Windows:

```powershell
scripts\build.bat
scripts\build.bat all
scripts\build.bat linux-arm64
scripts\build.bat darwin-arm64
```

产物统一输出到 `build/`，文件名格式为：

```text
syskit-<platform>[.exe]
```

示例：

- `syskit-windows-x64.exe`
- `syskit-linux-arm64`
- `syskit-macos-arm64`

## 发布

### 推荐方式：发布脚本 + GitHub Actions

Windows:

```powershell
scripts\release.bat 0.4.0
git push origin master --follow-tags
```

Linux/macOS:

```bash
./scripts/release.sh 0.4.0
git push origin master --follow-tags
```

推送 `v0.4.0` 这类 tag 后，GitHub Actions 工作流会自动构建全部平台并创建 GitHub Release。

### 手动方式：GitHub CLI

```bash
gh release create v0.4.0 build/* --title "v0.4.0" --notes "Release 0.4.0"
```

更详细的发布步骤见 [发布说明](docs/RELEASE_GUIDE.md) 和 [脚本说明](scripts/README.md)。

## 使用

### 基本命令

```bash
syskit D:\
syskit .
syskit --top 50 D:\
syskit --exclude node_modules,.git,vendor D:\
```

### 输出控制

```bash
# 只看文件
syskit --include-dirs=false D:\

# 只看子目录
syskit --include-files=false D:\

# JSON 输出
syskit --format json D:\

# CSV 导出
syskit --export-csv report D:\
```

## 输出语义

- 子目录结果不包含扫描根目录本身
- 子目录大小是累计大小，包含其所有后代文件和目录
- 文件结果按单文件大小排序

## 主要参数

- `--top`, `-t`: 返回前 N 条目录和文件结果
- `--exclude`: 排除目录名，多个值用逗号分隔
- `--include-files`: 是否输出文件结果
- `--include-dirs`: 是否输出子目录结果
- `--format`: `table`、`json`、`csv`
- `--export-csv`: 导出 CSV 文件前缀
- `--help`: 查看帮助
- `--version`: 查看版本

## PowerShell 脚本

仓库提供了一个本地脚本包装器：

```powershell
scripts\find-largest-local.ps1
scripts\find-largest-local.ps1 -Path D:\ -Top 30
scripts\find-largest-local.ps1 -Path D:\ -Exclude "node_modules,.git" -ExportCsvPath D:\scan
```

它会优先调用已编译的程序；如果找不到，就回退到 `go run .`。

## 项目结构

```text
syskit/
├── main.go
├── README.md
├── docs/
│   ├── QUICKSTART.md
│   ├── DESIGN.md
│   ├── DEV_GUIDE.md
│   └── RELEASE_GUIDE.md
├── internal/scanner/
│   ├── scanner.go
│   └── types.go
├── pkg/utils/
│   └── size.go
└── scripts/
    ├── build.bat
    ├── build.sh
    ├── find-largest-local.ps1
    ├── release.bat
    ├── release.sh
    └── README.md
```

## 开发

```bash
go test ./...
gofmt -w .
```

更多信息见：

- [快速开始](docs/QUICKSTART.md)
- [设计说明](docs/DESIGN.md)
- [开发说明](docs/DEV_GUIDE.md)
- [发布说明](docs/RELEASE_GUIDE.md)

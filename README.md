# Find Large Files

一个高性能的跨平台文件系统分析工具，快速找出占用空间最大的文件和文件夹。

## ✨ 特性

- 🚀 **三种扫描模式**：混合模式（推荐）、快速模式、完整模式
- 📊 **多种输出格式**：表格、JSON、CSV
- 🎯 **智能过滤**：支持目录排除、大小阈值、深度限制
- 💻 **跨平台支持**：Windows、Linux、macOS
- ⚡ **高性能**：并发扫描，15-30秒完成大盘扫描
- 🎨 **友好界面**：彩色表格输出，进度提示

## 📦 快速开始

### 方式 1：下载预编译版本（推荐）

从 [Releases](../../releases) 页面下载对应平台的可执行文件：

- **Windows**: `find-large-files-windows-amd64.exe`
- **Linux**: `find-large-files-linux-amd64`
- **macOS (Intel)**: `find-large-files-darwin-amd64`
- **macOS (Apple Silicon)**: `find-large-files-darwin-arm64`

下载后直接运行：

```bash
# Windows
find-large-files-windows-amd64.exe D:\

# Linux/macOS
chmod +x find-large-files-linux-amd64
./find-large-files-linux-amd64 /home/user
```

### 方式 2：从源码编译

#### 前置要求

- Go 1.25.0 或更高版本
- Git（可选，用于克隆仓库）

#### 编译步骤

**1. 克隆或下载项目**

```bash
# 使用 Git 克隆
git clone <repository-url>
cd find-large-files

# 或直接下载源码包并解压
```

**2. 编译当前平台版本**

```bash
# Windows
go build -o find-large-files.exe

# Linux/macOS
go build -o find-large-files
```

编译完成后，可执行文件会生成在当前目录。

**3. 使用构建脚本（推荐）**

构建脚本支持灵活的编译选项：

```bash
# Linux/macOS
chmod +x scripts/build.sh

# 编译当前平台（默认）
./scripts/build.sh

# 编译所有平台
./scripts/build.sh all

# 编译特定平台
./scripts/build.sh windows-amd64
./scripts/build.sh linux-amd64
./scripts/build.sh darwin-arm64

# 编译某个操作系统的所有版本
./scripts/build.sh windows
./scripts/build.sh linux
./scripts/build.sh darwin

# 查看帮助
./scripts/build.sh help
```

```cmd
# Windows
# 编译当前平台（默认）
scripts\build.bat

# 编译所有平台
scripts\build.bat all

# 编译特定平台
scripts\build.bat windows-amd64
scripts\build.bat linux-arm64

# 查看帮助
scripts\build.bat help
```

编译后的文件会保存在 `build/` 目录：

```
build/
├── find-large-files-windows-amd64.exe    # Windows 64位
├── find-large-files-linux-amd64          # Linux 64位
├── find-large-files-darwin-amd64         # macOS Intel
└── find-large-files-darwin-arm64         # macOS Apple Silicon
```

**4. 安装到系统路径（可选）**

```bash
# Linux/macOS
sudo cp find-large-files /usr/local/bin/
find-large-files --version

# Windows（需要管理员权限）
# 将 find-large-files.exe 复制到 C:\Windows\System32\
# 或添加到 PATH 环境变量
```

## 🎯 使用方法

### 基本用法

```bash
# 扫描指定目录（使用默认混合模式）
find-large-files D:\

# 扫描当前目录
find-large-files .

# 显示帮助信息
find-large-files --help
```

### 三种扫描模式

#### 1. 混合模式（默认，推荐）⭐

分两阶段扫描，先快速找出大目录，再精确找出大文件：

```bash
find-large-files D:\
```

**特点**：
- 阶段1：5-10秒显示 Top 20 大目录
- 询问是否继续
- 阶段2：10-20秒显示 Top 20 大文件
- 总耗时：15-30秒

#### 2. 快速模式

使用启发式算法，快速定位大文件：

```bash
# 使用默认配置（深度3层，100MB阈值）
find-large-files --mode fast D:\

# 自定义配置
find-large-files --mode fast --max-depth 5 --min-size 50MB D:\

# 排除特定目录
find-large-files --mode fast --exclude node_modules,.git,vendor D:\
```

**特点**：
- 扫描速度：10-30秒
- 准确率：85-95%
- 适合快速定位

#### 3. 完整模式

全盘扫描，100% 准确：

```bash
find-large-files --mode full D:\
```

**特点**：
- 扫描速度：1-3分钟
- 准确率：100%
- 适合完整报告

### 常用参数

```bash
# 显示 Top 50 结果
find-large-files --top 50 D:\

# 只显示文件，不显示目录
find-large-files --include-dirs=false D:\

# 只显示目录，不显示文件
find-large-files --include-files=false D:\

# 排除多个目录
find-large-files --exclude node_modules,.git,vendor,target D:\

# 设置最小文件大小（快速模式）
find-large-files --mode fast --min-size 500MB D:\

# 设置最大扫描深度（快速模式）
find-large-files --mode fast --max-depth 2 D:\
```

### 输出格式

#### 表格格式（默认）

```bash
find-large-files D:\
```

输出示例：
```
================================================================================
Top 20 目录（按累计大小排序）
================================================================================
序号   大小           路径
--------------------------------------------------------------------------------
1    45.2 GB      D:\Projects
2    32.1 GB      D:\Downloads
3    28.5 GB      D:\Videos
...
```

#### JSON 格式

```bash
find-large-files --format json D:\ > result.json
```

#### CSV 导出

```bash
find-large-files --export-csv report D:\
```

生成两个文件：
- `report_dirs.csv` - 目录结果
- `report_files.csv` - 文件结果

## 📚 使用场景

### 场景 1：磁盘空间不足，快速定位大文件

```bash
find-large-files D:\
```

### 场景 2：清理项目目录

```bash
find-large-files --exclude node_modules,.git,vendor,target,build ./
```

### 场景 3：导出分析报告

```bash
# CSV 格式
find-large-files --export-csv report D:\

# JSON 格式
find-large-files --format json D:\ > report.json
```

### 场景 4：查找超大文件

```bash
find-large-files --mode fast --min-size 1GB D:\
```

### 场景 5：分析特定深度

```bash
# 只扫描前2层目录
find-large-files --mode fast --max-depth 2 D:\
```

## 📁 项目结构

```
find-large-files/
├── main.go                    # 程序入口
├── go.mod                     # Go 模块定义
├── go.sum                     # 依赖锁定文件
├── README.md                  # 本文档
├── internal/                  # 内部包
│   └── scanner/
│       ├── types.go           # 数据结构定义
│       ├── scanner.go         # 扫描器接口
│       ├── fast.go            # 快速模式扫描器
│       ├── full.go            # 完整模式扫描器
│       ├── hybrid.go          # 混合模式扫描器
│       └── concurrent.go      # 并发工具
├── pkg/                       # 公共包
│   └── utils/
│       └── size.go            # 大小格式化工具
├── scripts/                   # 构建脚本
│   ├── build.sh               # Linux/macOS 构建脚本
│   ├── build.bat              # Windows 构建脚本
│   └── find-largest-local.ps1 # PowerShell 版本（旧）
├── docs/                      # 文档
│   ├── DESIGN.md              # 技术设计文档
│   ├── DEV_GUIDE.md           # Go 语言开发指南
│   └── QUICKSTART.md          # 快速使用指南
└── build/                     # 编译输出目录
    ├── find-large-files-windows-amd64.exe
    ├── find-large-files-linux-amd64
    ├── find-large-files-darwin-amd64
    └── find-large-files-darwin-arm64
```

## 🔧 开发指南

### 环境准备

```bash
# 安装 Go 1.25.0+
# 下载地址：https://go.dev/dl/

# 验证安装
go version

# 克隆项目
git clone <repository-url>
cd find-large-files

# 下载依赖
go mod download
```

### 本地开发

```bash
# 运行程序（不编译）
go run main.go D:\

# 编译并运行
go build -o find-large-files.exe
./find-large-files.exe D:\

# 运行测试
go test ./...

# 代码格式化
go fmt ./...

# 静态检查
go vet ./...
```

### 添加新功能

1. 在 `internal/scanner/` 添加新的扫描器
2. 在 `pkg/utils/` 添加工具函数
3. 在 `main.go` 添加 CLI 参数
4. 更新文档

### 构建发布版本

使用构建脚本编译所有平台版本：

```bash
# Linux/macOS - 编译所有平台
./scripts/build.sh all

# Windows - 编译所有平台
scripts\build.bat all

# 编译特定平台
./scripts/build.sh windows        # 所有 Windows 版本
./scripts/build.sh linux          # 所有 Linux 版本
./scripts/build.sh darwin         # 所有 macOS 版本
./scripts/build.sh windows-amd64  # 仅 Windows 64位
```

所有编译产物保存在 `build/` 目录。

详细的构建脚本使用说明请参考 [scripts/README.md](scripts/README.md)。

## ⚙️ 性能优化

| 场景 | 推荐模式 | 预计耗时 | 准确率 |
|------|---------|---------|--------|
| 快速定位大文件 | 混合模式 | 15-30秒 | 95%+ |
| 自定义扫描参数 | 快速模式 | 10-30秒 | 85-95% |
| 完整准确报告 | 完整模式 | 1-3分钟 | 100% |

### 提速技巧

1. **减小扫描深度**：`--max-depth 2`
2. **排除大目录**：`--exclude node_modules,.git`
3. **提高大小阈值**：`--min-size 500MB`
4. **使用快速模式**：`--mode fast`

## ❓ 常见问题

### Q: 为什么快速模式找不到某些大文件？

A: 快速模式有深度限制（默认3层）和大小阈值（默认100MB）。如果大文件在更深的目录或小于阈值，会被过滤。建议使用混合模式或完整模式。

### Q: 如何提高扫描速度？

A:
1. 使用快速模式并减小深度：`--mode fast --max-depth 2`
2. 排除已知的大目录：`--exclude node_modules,.git`
3. 提高大小阈值：`--min-size 500MB`

### Q: 扫描时遇到权限错误怎么办？

A: 程序会自动跳过无权限的目录，不会中断扫描。如需扫描系统目录，请使用管理员权限运行：

```bash
# Windows（以管理员身份运行 PowerShell）
.\find-large-files.exe C:\

# Linux/macOS
sudo ./find-large-files /
```

### Q: 支持哪些大小单位？

A: 支持 B, KB, MB, GB, TB（不区分大小写）

示例：
- `--min-size 100MB`
- `--min-size 1.5GB`
- `--min-size 500KB`

### Q: 如何排除多个目录？

A: 使用逗号分隔：

```bash
find-large-files --exclude node_modules,.git,vendor,target,build D:\
```

### Q: 可以扫描网络驱动器吗？

A: 可以，但速度会较慢。建议使用快速模式并减小深度：

```bash
find-large-files --mode fast --max-depth 2 Z:\
```

## 📄 许可证

[MIT License](LICENSE)

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📞 联系方式

- 问题反馈：[GitHub Issues](../../issues)
- 功能建议：[GitHub Discussions](../../discussions)

## 🔗 相关文档

- [技术设计文档](docs/DESIGN.md)
- [开发指南](docs/DEV_GUIDE.md)
- [快速使用指南](docs/QUICKSTART.md)

---

**提示**：首次使用建议阅读 [快速使用指南](docs/QUICKSTART.md)

# Scripts 目录说明

本目录包含项目的构建脚本和工具脚本。

## 📁 文件列表

### build.sh
Linux/macOS 平台的构建脚本。

**用法**：
```bash
chmod +x scripts/build.sh
./scripts/build.sh [target]
```

**参数**：
- `(无参数)` - 编译当前平台版本（自动检测）
- `all` - 编译所有平台版本
- `windows` - 编译所有 Windows 版本
- `windows-amd64` - 编译 Windows 64位版本
- `windows-386` - 编译 Windows 32位版本
- `windows-arm64` - 编译 Windows ARM64版本
- `linux` - 编译所有 Linux 版本
- `linux-amd64` - 编译 Linux 64位版本
- `linux-386` - 编译 Linux 32位版本
- `linux-arm64` - 编译 Linux ARM64版本
- `linux-arm` - 编译 Linux ARM32版本
- `darwin` - 编译所有 macOS 版本
- `darwin-amd64` - 编译 macOS Intel版本
- `darwin-arm64` - 编译 macOS Apple Silicon版本
- `help` - 显示帮助信息

**示例**：
```bash
# 编译当前平台
./scripts/build.sh

# 编译所有平台
./scripts/build.sh all

# 只编译 Windows 64位
./scripts/build.sh windows-amd64

# 编译所有 macOS 版本
./scripts/build.sh darwin
```

### build.bat
Windows 平台的构建脚本。

**用法**：
```cmd
scripts\build.bat [target]
```

**参数**：与 build.sh 相同

**示例**：
```cmd
REM 编译当前平台
scripts\build.bat

REM 编译所有平台
scripts\build.bat all

REM 只编译 Windows 64位
scripts\build.bat windows-amd64

REM 编译所有 Linux 版本
scripts\build.bat linux
```

### find-largest-local.ps1
PowerShell 版本的文件扫描工具（旧版本，已被 Go 版本替代）。

保留此文件用于参考和对比性能。

### release.sh / release.bat
自动化发布脚本，用于创建新版本发布。

**用法**：
```bash
# Linux/macOS
chmod +x scripts/release.sh
./scripts/release.sh 0.3.0

# Windows
scripts\release.bat 0.3.0
```

**功能**：
- 自动更新 main.go 中的版本号
- 编译所有平台版本
- 创建 Git 标签
- 提供后续发布步骤指引

## 🚀 发布新版本流程

### 方法 1：使用发布脚本（推荐）

```bash
# 1. 运行发布脚本
./scripts/release.sh 0.3.0

# 2. 按照提示推送代码和标签
git push origin master
git push origin v0.3.0

# 3. 使用 GitHub CLI 创建发布（推荐）
gh release create v0.3.0 build/* \
  --title "v0.3.0" \
  --notes "Release version 0.3.0"

# 或手动在 GitHub 网页上创建 Release 并上传 build/ 目录下的文件
```

### 方法 2：手动发布

```bash
# 1. 更新版本号（编辑 main.go）
# 修改: version = "0.3.0"

# 2. 提交更改
git add main.go
git commit -m "Bump version to 0.3.0"

# 3. 编译所有平台
./scripts/build.sh all

# 4. 创建标签
git tag -a v0.3.0 -m "Release version 0.3.0"

# 5. 推送
git push origin master
git push origin v0.3.0

# 6. 在 GitHub 上创建 Release
# 访问: https://github.com/YOUR_USERNAME/find-large-files/releases/new
# 上传 build/ 目录下的所有文件
```

### 方法 3：使用 GitHub Actions 自动发布

项目已配置 GitHub Actions 自动发布工作流（`.github/workflows/release.yml`）。

只需推送标签即可自动触发：

```bash
# 1. 创建标签
git tag -a v0.3.0 -m "Release version 0.3.0"

# 2. 推送标签
git push origin v0.3.0

# GitHub Actions 会自动：
# - 编译所有平台版本
# - 创建 Release
# - 上传所有编译产物
```

## 📝 发布说明模板

创建 Release 时使用以下模板：

```markdown
## Find Large Files v0.3.0

一个高性能的跨平台文件系统分析工具，快速找出占用空间最大的文件和文件夹。

### ✨ 新特性

- 🚀 三种扫描模式：混合模式（推荐）、快速模式、完整模式
- 📊 多种输出格式：表格、JSON、CSV
- 🎯 智能过滤：支持目录排除、大小阈值、深度限制

### 📦 下载

根据你的操作系统选择对应的版本：

**Windows**
- Windows 64位 (推荐): `find-large-files-windows-amd64.exe`
- Windows 32位: `find-large-files-windows-386.exe`
- Windows ARM64: `find-large-files-windows-arm64.exe`

**Linux**
- Linux 64位 (推荐): `find-large-files-linux-amd64`
- Linux ARM64: `find-large-files-linux-arm64`

**macOS**
- macOS Intel: `find-large-files-darwin-amd64`
- macOS Apple Silicon (M1/M2/M3): `find-large-files-darwin-arm64`

### 🚀 快速开始

\`\`\`bash
# Windows
find-large-files-windows-amd64.exe D:\

# Linux/macOS
chmod +x find-large-files-linux-amd64
./find-large-files-linux-amd64 /home/user
\`\`\`

### 📚 文档

- [完整文档](../README.md)
- [快速使用指南](../docs/QUICKSTART.md)
```

## 🎯 常见使用场景

### 场景 1：本地开发测试
编译当前平台版本，快速测试：
```bash
# Linux/macOS
./scripts/build.sh

# Windows
scripts\build.bat
```

编译后的文件位于 `build/` 目录。

### 场景 2：发布新版本
编译所有平台版本，准备发布：
```bash
# Linux/macOS
./scripts/build.sh all

# Windows
scripts\build.bat all
```

### 场景 3：只编译特定平台
为特定平台编译：
```bash
# 只编译 Windows 版本
./scripts/build.sh windows

# 只编译 Linux ARM64（树莓派、ARM服务器）
./scripts/build.sh linux-arm64

# 只编译 macOS Apple Silicon
./scripts/build.sh darwin-arm64
```

## 📊 支持的平台架构

### Windows
| 架构 | 说明 | 覆盖率 |
|------|------|--------|
| amd64 | 64位 Intel/AMD | 99% Windows 用户 |
| 386 | 32位 Intel/AMD | 老旧系统 |
| arm64 | ARM64 | Surface Pro X 等 |

### Linux
| 架构 | 说明 | 覆盖率 |
|------|------|--------|
| amd64 | 64位 Intel/AMD | 大部分桌面/服务器 |
| 386 | 32位 Intel/AMD | 老旧系统 |
| arm64 | ARM64 | 树莓派4、ARM服务器 |
| arm | ARM32 | 树莓派3等 |

### macOS
| 架构 | 说明 | 覆盖率 |
|------|------|--------|
| amd64 | Intel 芯片 | 2020年前的 Mac |
| arm64 | Apple Silicon | M1/M2/M3 Mac |

## 🔧 编译选项说明

构建脚本使用以下 Go 编译选项：

```bash
go build -ldflags="-s -w" -o <output>
```

**参数说明**：
- `-ldflags="-s -w"` - 减小可执行文件大小
  - `-s` - 去除符号表
  - `-w` - 去除 DWARF 调试信息
- `-o <output>` - 指定输出文件名

**效果**：
- 可执行文件大小减少约 30%
- 不影响程序运行性能
- 调试信息被移除（生产环境推荐）

## 📝 输出文件命名规范

编译后的文件统一命名为：
```
find-large-files-{os}-{arch}{ext}
```

**示例**：
- `find-large-files-windows-amd64.exe`
- `find-large-files-linux-amd64`
- `find-large-files-darwin-arm64`

## ⚠️ 注意事项

1. **首次使用需要设置执行权限**（Linux/macOS）：
   ```bash
   chmod +x scripts/build.sh
   ```

2. **需要安装 Go 1.25.0+**：
   ```bash
   go version
   ```

3. **交叉编译可能需要额外依赖**：
   - 大部分情况下 Go 自动处理
   - 如遇到 CGO 相关错误，可能需要安装交叉编译工具链

4. **编译输出目录**：
   - 所有编译产物保存在 `build/` 目录
   - 该目录会自动创建

5. **清理编译产物**：
   ```bash
   # Linux/macOS
   rm -rf build/*

   # Windows
   del /Q build\*
   ```

## 🚀 性能对比

| 版本 | 文件大小 | 启动速度 | 扫描性能 |
|------|---------|---------|---------|
| PowerShell | N/A | 慢 | 慢 (基准) |
| Go (amd64) | ~3.3MB | 极快 | 10-30x 提升 |
| Go (arm64) | ~3.2MB | 极快 | 10-30x 提升 |

## 📚 相关文档

- [README.md](../README.md) - 项目主文档
- [docs/DESIGN.md](../docs/DESIGN.md) - 技术设计文档
- [docs/DEV_GUIDE.md](../docs/DEV_GUIDE.md) - 开发指南
- [docs/QUICKSTART.md](../docs/QUICKSTART.md) - 快速使用指南

## 🤝 贡献

如需添加新的构建脚本或改进现有脚本，欢迎提交 Pull Request！

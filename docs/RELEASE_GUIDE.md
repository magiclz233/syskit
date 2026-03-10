# GitHub 发布指南

本文档介绍如何在 GitHub 上发布 Find Large Files 的新版本。

## 📋 发布前检查清单

- [ ] 所有功能已完成并测试
- [ ] 代码已提交到 master 分支
- [ ] 更新了 CHANGELOG（如果有）
- [ ] 更新了文档（如果需要）
- [ ] 本地测试通过

## 🚀 发布方法

### 方法 1：使用发布脚本（最简单）⭐

```bash
# Linux/macOS
./scripts/release.sh 0.3.0

# Windows
scripts\release.bat 0.3.0
```

脚本会自动：
1. 更新 main.go 中的版本号
2. 提交版本更新
3. 编译所有平台版本
4. 创建 Git 标签
5. 显示后续步骤

然后按照提示操作：

```bash
# 推送代码和标签
git push origin master
git push origin v0.3.0

# 使用 GitHub CLI 创建发布（推荐）
gh release create v0.3.0 build/* \
  --title "v0.3.0 - Find Large Files" \
  --notes-file RELEASE_NOTES.md
```

### 方法 2：使用 GitHub Actions（全自动）⭐⭐⭐

项目已配置自动发布工作流，只需推送标签：

```bash
# 1. 更新版本号并提交
# 编辑 main.go，修改 version = "0.3.0"
git add main.go
git commit -m "Bump version to 0.3.0"
git push origin master

# 2. 创建并推送标签
git tag -a v0.3.0 -m "Release version 0.3.0"
git push origin v0.3.0

# GitHub Actions 会自动完成剩余工作！
```

GitHub Actions 会自动：
- ✅ 编译所有平台版本
- ✅ 创建 GitHub Release
- ✅ 上传所有编译产物
- ✅ 生成下载链接

### 方法 3：手动发布（完全控制）

#### 步骤 1：准备发布

```bash
# 1. 更新版本号
# 编辑 main.go，修改: version = "0.3.0"

# 2. 提交更改
git add main.go
git commit -m "Bump version to 0.3.0"

# 3. 编译所有平台
rm -rf build/*
./scripts/build.sh all

# 4. 验证编译产物
ls -lh build/
```

#### 步骤 2：创建标签

```bash
# 创建带注释的标签
git tag -a v0.3.0 -m "Release version 0.3.0

新特性：
- 三种扫描模式
- 多种输出格式
- 跨平台支持
"

# 推送代码和标签
git push origin master
git push origin v0.3.0
```

#### 步骤 3：在 GitHub 上创建 Release

1. 访问仓库的 Releases 页面：
   ```
   https://github.com/YOUR_USERNAME/find-large-files/releases
   ```

2. 点击 **"Draft a new release"**

3. 填写发布信息：
   - **Choose a tag**: 选择 `v0.3.0`
   - **Release title**: `v0.3.0 - Find Large Files`
   - **Description**: 使用下方的发布说明模板

4. 上传编译产物：
   - 点击 **"Attach binaries by dropping them here or selecting them"**
   - 上传 `build/` 目录下的所有文件（9个文件）

5. 点击 **"Publish release"**

## 📝 发布说明模板

```markdown
## Find Large Files v0.3.0

一个高性能的跨平台文件系统分析工具，快速找出占用空间最大的文件和文件夹。

### ✨ 新特性

- 🚀 三种扫描模式：混合模式（推荐）、快速模式、完整模式
- 📊 多种输出格式：表格、JSON、CSV
- 🎯 智能过滤：支持目录排除、大小阈值、深度限制
- 💻 跨平台支持：Windows、Linux、macOS
- ⚡ 高性能：并发扫描，15-30秒完成大盘扫描

### 📦 下载

根据你的操作系统选择对应的版本：

#### Windows
- **Windows 64位** (推荐): `find-large-files-windows-amd64.exe`
- **Windows 32位**: `find-large-files-windows-386.exe`
- **Windows ARM64** (Surface Pro X): `find-large-files-windows-arm64.exe`

#### Linux
- **Linux 64位** (推荐): `find-large-files-linux-amd64`
- **Linux 32位**: `find-large-files-linux-386`
- **Linux ARM64** (树莓派4、ARM服务器): `find-large-files-linux-arm64`
- **Linux ARM32** (树莓派3): `find-large-files-linux-arm`

#### macOS
- **macOS Intel**: `find-large-files-darwin-amd64`
- **macOS Apple Silicon** (M1/M2/M3): `find-large-files-darwin-arm64`

### 🚀 快速开始

```bash
# Windows
find-large-files-windows-amd64.exe D:\

# Linux/macOS (需要添加执行权限)
chmod +x find-large-files-linux-amd64
./find-large-files-linux-amd64 /home/user

# 查看帮助
find-large-files-windows-amd64.exe --help
```

### 📚 文档

- [完整文档](https://github.com/YOUR_USERNAME/find-large-files/blob/master/README.md)
- [快速使用指南](https://github.com/YOUR_USERNAME/find-large-files/blob/master/docs/QUICKSTART.md)
- [技术设计文档](https://github.com/YOUR_USERNAME/find-large-files/blob/master/docs/DESIGN.md)

### 🐛 已知问题

无

### 📝 更新日志

**v0.3.0** (2026-03-10)
- 初始发布
- 实现三种扫描模式（混合、快速、完整）
- 支持多种输出格式（表格、JSON、CSV）
- 跨平台支持（Windows、Linux、macOS）
- 高性能并发扫描
```

## 🔧 使用 GitHub CLI (gh)

如果安装了 GitHub CLI，可以更快速地创建发布：

```bash
# 安装 GitHub CLI
# Windows: winget install GitHub.cli
# macOS: brew install gh
# Linux: 参考 https://cli.github.com/

# 登录
gh auth login

# 创建发布（交互式）
gh release create v0.3.0 build/* \
  --title "v0.3.0 - Find Large Files" \
  --notes "Release version 0.3.0"

# 或使用发布说明文件
gh release create v0.3.0 build/* \
  --title "v0.3.0 - Find Large Files" \
  --notes-file RELEASE_NOTES.md

# 创建草稿发布（可以稍后编辑）
gh release create v0.3.0 build/* \
  --draft \
  --title "v0.3.0 - Find Large Files" \
  --notes "Release version 0.3.0"
```

## 📊 发布后检查

发布完成后，验证以下内容：

- [ ] Release 页面显示正常
- [ ] 所有平台的文件都已上传（9个文件）
- [ ] 下载链接可用
- [ ] 发布说明格式正确
- [ ] 标签已创建
- [ ] README 中的下载链接指向正确的 Release

## 🔄 更新现有 Release

如果需要更新已发布的版本：

```bash
# 删除远程标签
git push --delete origin v0.3.0

# 删除本地标签
git tag -d v0.3.0

# 重新创建标签
git tag -a v0.3.0 -m "Release version 0.3.0"

# 推送新标签
git push origin v0.3.0

# 在 GitHub 上删除旧的 Release，然后重新创建
```

## 📚 相关资源

- [GitHub Releases 文档](https://docs.github.com/en/repositories/releasing-projects-on-github)
- [GitHub CLI 文档](https://cli.github.com/manual/)
- [GitHub Actions 文档](https://docs.github.com/en/actions)
- [语义化版本规范](https://semver.org/lang/zh-CN/)

## 💡 最佳实践

1. **使用语义化版本**：`MAJOR.MINOR.PATCH`
   - MAJOR: 不兼容的 API 修改
   - MINOR: 向下兼容的功能性新增
   - PATCH: 向下兼容的问题修正

2. **编写清晰的发布说明**：
   - 列出新特性
   - 说明破坏性变更
   - 记录已知问题
   - 提供升级指南

3. **测试所有平台**：
   - 至少测试主要平台（Windows、Linux、macOS）
   - 验证下载链接可用
   - 确保程序可以正常运行

4. **保持一致的发布节奏**：
   - 定期发布小版本
   - 及时修复重要 bug
   - 收集用户反馈

---

**提示**：首次发布建议使用方法 1（发布脚本），熟悉流程后可以使用方法 2（GitHub Actions）实现全自动发布。

# 发布说明

## 发布前检查

发布前至少确认下面几项：

```bash
go test ./...
```

需要同步检查：

- `README.md`
- `docs/`
- `scripts/README.md`
- `syskit --help`
- [version.go](/e:/code/golang/find-large-files/internal/version/version.go) 中的版本号

## 支持发布的平台

当前 Release 资产包含以下目标：

- Windows x64
- Windows x86
- Windows ARM64
- Linux x64
- Linux x86
- Linux ARM64
- Linux ARMv7
- macOS x64
- macOS ARM64

产物命名规则：

```text
syskit-<platform>[.exe]
```

## 本地构建全部产物

Linux/macOS:

```bash
./scripts/build.sh all
```

Windows:

```powershell
scripts\build.bat all
```

构建结果输出到 `build/`。

## 推荐发布流程

### 方式 1：使用发布脚本

Linux/macOS:

```bash
./scripts/release.sh 0.4.0
```

Windows:

```powershell
scripts\release.bat 0.4.0
```

脚本会自动：

1. 检查工作区是否干净
2. 检查 tag 是否已存在
3. 更新版本号
4. 提交版本变更
5. 构建全部平台
6. 创建本地 tag

脚本完成后，继续执行：

```bash
git push origin <当前分支>
git push origin v0.4.0
```

或者：

```bash
git push origin <当前分支> --follow-tags
```

## GitHub Actions 自动 Release

仓库中已经配置了工作流 [release.yml](/e:/code/golang/find-large-files/.github/workflows/release.yml)。

触发条件：

- push 一个形如 `v*` 的 tag，例如 `v0.4.0`

工作流会自动执行：

1. `actions/checkout@v3`
2. `actions/setup-go@v4`
3. `./scripts/build.sh all`
4. 创建 GitHub Release
5. 逐个上传构建产物

这意味着正常情况下，发布脚本 + `git push --follow-tags` 就足够完成发布。

## 手动 GitHub Release 方式

### 方式 2：GitHub Web 页面

如果不想用 Actions 自动创建，也可以：

1. 本地先运行构建脚本
2. 手动 `git tag`
3. `git push` 分支和 tag
4. 打开 GitHub Releases 页面
5. 新建 Release
6. 上传 `build/` 目录下的全部文件

### 方式 3：GitHub CLI

```bash
gh release create v0.4.0 build/* --title "v0.4.0" --notes "Release 0.4.0"
```

前提：

- 本地已安装 `gh`
- 已完成 `gh auth login`
- 对应 tag 已存在并已推送

## Release 文案原则

Release 标题和说明必须反映项目当前真实行为。

当前项目只支持：

- 全量准确扫描
- Top 子目录输出
- Top 文件输出
- 表格 / JSON / CSV

不要在 Release 文案里引入任何未实现的扫描分支或近似行为描述。

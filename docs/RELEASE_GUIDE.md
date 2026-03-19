# 发布说明

## 发布前检查

发布前至少完成以下检查：

```bash
go test ./...
./scripts/verify-p0.sh
```

Windows:

```powershell
go test ./...
scripts\verify-p0.bat
```

同时确认以下内容已同步：

- `README.md`
- `docs/QUICKSTART.md`
- `docs/DEV_GUIDE.md`
- `docs/DESIGN.md`
- `scripts/README.md`
- `syskit --help`
- `internal/version/version.go`

## 正式支持目标

Release 资产只保留 6 个正式目标：

- `windows-amd64`
- `windows-arm64`
- `linux-amd64`
- `linux-arm64`
- `darwin-amd64`
- `darwin-arm64`

旧目标 `windows-386`、`linux-386`、`linux-arm` 已下线，构建脚本会直接报错。

产物命名规则：

```text
syskit-<platform>[.exe]
```

当前实际产物为：

- `build/syskit-windows-x64.exe`
- `build/syskit-windows-arm64.exe`
- `build/syskit-linux-x64`
- `build/syskit-linux-arm64`
- `build/syskit-macos-x64`
- `build/syskit-macos-arm64`

## 本地构建

Linux/macOS:

```bash
./scripts/build.sh all
```

Windows:

```powershell
scripts\build.bat all
```

## 推荐发布流程

### 方式 1：发布脚本

Linux/macOS:

```bash
./scripts/release.sh 0.4.0
```

Windows:

```powershell
scripts\release.bat 0.4.0
```

脚本会自动执行：

1. 检查工作区是否干净
2. 检查目标 tag 是否已存在
3. 更新 `internal/version/version.go`
4. 提交版本变更
5. 构建 6 个正式支持目标
6. 创建本地 tag

脚本完成后继续推送：

```bash
git push origin <当前分支>
git push origin v0.4.0
```

或者：

```bash
git push origin <当前分支> --follow-tags
```

## GitHub Actions Release

仓库中的 [release.yml](../.github/workflows/release.yml) 会在推送 `v*` tag 后执行：

1. `actions/checkout`
2. `actions/setup-go`
3. `./scripts/build.sh all`
4. 发布 `build/*` 下的产物

当前不会新增三平台 CI 测试矩阵；GitHub Actions 继续在单个 Ubuntu runner 上交叉编译 6 个正式目标并上传 Release 资产。

## 手动发布

如果不想使用 Actions 自动创建 Release，也可以手动执行：

```bash
gh release create v0.4.0 build/* --title "v0.4.0" --notes "Release 0.4.0"
```

前提：

- 已完成本地构建
- 对应 tag 已存在并已推送
- 本地已安装并登录 `gh`

## 平台验证记录

手工 smoke 验证步骤与记录模板见 [P0 平台验证清单](P0_PLATFORM_VERIFICATION.md)。

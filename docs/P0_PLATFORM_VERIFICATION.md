# P0 平台验证清单

## 目标

本清单用于统一记录 P0 在 Windows、Linux、macOS 上的编译与核心 smoke 验证结果。

## 正式支持目标

- `windows-amd64`
- `windows-arm64`
- `linux-amd64`
- `linux-arm64`
- `darwin-amd64`
- `darwin-arm64`

## 自动化验证脚本

Linux/macOS:

```bash
./scripts/verify-p0.sh
```

Windows:

```powershell
scripts\verify-p0.bat
```

## 核心 smoke 步骤

每个平台至少确认以下命令可用：

1. `syskit --help`
2. `syskit doctor all --fail-on never --format json`
3. `syskit disk --format json`
4. `syskit disk scan <path> --limit 3 --format json`
5. `syskit snapshot list --limit 1 --format json`
6. `syskit policy validate <config> --type config --format json`

## 记录表

| 平台 | 日期 | 验证方式 | 结果 | 备注 |
|---|---|---|---|---|
| Windows | 2026-03-19 | `scripts\\verify-p0.bat` | 已通过 | 已完成 `go test ./...`、六目标编译与核心 smoke；末尾存在少量 Windows 路径噪声输出，不影响退出码 |
| Linux | 待补充 | `./scripts/verify-p0.sh` | 待执行 | 需在 Linux 主机记录 |
| macOS | 待补充 | `./scripts/verify-p0.sh` | 待执行 | 需在 macOS 主机记录 |

## 手工补充项

自动化脚本通过后，建议额外人工抽查：

- `syskit disk scan --help`
- `syskit fix cleanup --help`
- `syskit snapshot --help`
- `syskit policy --help`
- 一个真实的 `disk scan` 输出示例

## 说明

- 当前 CI 不新增三平台测试矩阵。
- GitHub Actions 仍通过 `scripts/build.sh all` 交叉编译 6 个正式目标。
- Linux/macOS 结果需要在对应主机完成后回填本表。

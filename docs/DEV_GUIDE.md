# 开发说明

## 代码入口

- `cmd/syskit/main.go`: 二进制入口，负责启动 CLI 和返回退出码
- `internal/cli/`: Cobra 命令树、全局参数和路由
- `internal/scanner/scanner.go`: 唯一的准确扫描实现
- `internal/scanner/types.go`: 扫描参数和结果结构
- `pkg/utils/size.go`: 大小格式化和数字格式化

## 当前行为

程序始终执行全量准确扫描。

输入：

- 根目录路径
- Top N
- 排除目录列表
- 是否输出文件 / 子目录
- 输出格式

输出：

- Top 子目录
- Top 文件
- 总大小
- 文件数 / 目录数
- 扫描耗时

## 修改建议

如果以后继续迭代，优先保持这几个原则：

1. 不新增近似模式
2. 不新增基于深度限制的默认行为
3. 保持 README、`docs/`、脚本帮助和 CLI 参数同步
4. 所有输出语义都要明确说明根目录是否包含在结果中

## 常用命令

```bash
go test ./...
gofmt -w .
```

Windows:

```powershell
go test ./...
gofmt -w .
```

## 调整输出时要注意

- `TopDirs` 应继续排除根目录
- CSV、JSON、表格三种输出要保持字段一致
- 如果改了 CLI 参数，`README.md` 和 `scripts/find-largest-local.ps1` 要一起更新

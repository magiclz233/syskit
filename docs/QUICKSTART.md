# 快速使用指南

## 安装

### 方式 1：使用预编译版本

从 `build/` 目录选择对应平台的可执行文件：

- **Windows**: `find-large-files-windows-amd64.exe`
- **Linux**: `find-large-files-linux-amd64`
- **macOS (Intel)**: `find-large-files-darwin-amd64`
- **macOS (Apple Silicon)**: `find-large-files-darwin-arm64`

### 方式 2：从源码编译

```bash
# 克隆仓库
git clone <repository-url>
cd find_large_files

# 编译当前平台
go build -o find-large-files

# 或使用构建脚本编译所有平台
./build.sh        # Linux/macOS
build.bat         # Windows
```

## 基本使用

### 1. 混合模式（推荐）

最快速找到大文件和大目录的方式：

```bash
# Windows
find-large-files.exe D:\

# Linux/macOS
./find-large-files /home/user
```

**工作流程**：
1. 阶段 1（5-10秒）：快速扫描，显示 Top 20 大目录
2. 询问是否继续
3. 阶段 2（10-20秒）：深入扫描大目录，显示 Top 20 大文件

### 2. 快速模式

自定义扫描参数：

```bash
# 默认配置（深度3层，100MB阈值）
find-large-files --mode fast D:\

# 自定义深度和阈值
find-large-files --mode fast --max-depth 5 --min-size 50MB D:\

# 排除特定目录
find-large-files --mode fast --exclude node_modules,.git,vendor D:\
```

### 3. 完整模式

全盘扫描，100% 准确：

```bash
find-large-files --mode full D:\
```

## 常用场景

### 场景 1：磁盘空间不足，快速定位大文件

```bash
# 使用混合模式（推荐）
find-large-files D:\
```

### 场景 2：清理项目目录

```bash
# 排除依赖目录，只看源码
find-large-files --exclude node_modules,.git,vendor,target,build ./
```

### 场景 3：导出分析报告

```bash
# 导出为 CSV
find-large-files --export-csv report D:\

# 导出为 JSON
find-large-files --format json D:\ > report.json
```

### 场景 4：只查看大文件（不看目录）

```bash
find-large-files --include-dirs=false D:\
```

### 场景 5：只查看大目录（不看文件）

```bash
find-large-files --include-files=false D:\
```

## 输出格式

### 表格格式（默认）

```
================================================================================
Top 20 目录（按累计大小排序）
================================================================================
序号   大小           路径
--------------------------------------------------------------------------------
1    45.2 GB      D:\Projects
2    32.1 GB      D:\Downloads
...
```

### JSON 格式

```bash
find-large-files --format json D:\ > result.json
```

### CSV 格式

```bash
find-large-files --export-csv result D:\
```

生成两个文件：
- `result_dirs.csv` - 目录结果
- `result_files.csv` - 文件结果

## 性能提示

| 场景 | 推荐模式 | 预计耗时 |
|------|---------|---------|
| 快速定位大文件 | 混合模式 | 15-30秒 |
| 自定义扫描参数 | 快速模式 | 10-30秒 |
| 完整准确报告 | 完整模式 | 1-3分钟 |

## 常见问题

### Q: 为什么快速模式找不到某些大文件？

A: 快速模式有深度限制（默认3层）和大小阈值（默认100MB）。如果大文件在更深的目录或小于阈值，会被过滤。建议使用混合模式或完整模式。

### Q: 如何提高扫描速度？

A:
1. 使用快速模式并减小深度：`--mode fast --max-depth 2`
2. 排除已知的大目录：`--exclude node_modules,.git`
3. 提高大小阈值：`--min-size 500MB`

### Q: 扫描时遇到权限错误怎么办？

A: 程序会自动跳过无权限的目录，不会中断扫描。如需扫描系统目录，请使用管理员权限运行。

### Q: 支持哪些大小单位？

A: 支持 B, KB, MB, GB, TB（不区分大小写）
- 示例：`--min-size 100MB`、`--min-size 1.5GB`

## 更多帮助

```bash
# 查看所有参数
find-large-files --help

# 查看版本
find-large-files --version
```

## 技术文档

- [README.md](README.md) - 完整项目说明
- [DESIGN.md](DESIGN.md) - 技术设计文档
- [DEV_GUIDE.md](DEV_GUIDE.md) - Go 语言开发指南

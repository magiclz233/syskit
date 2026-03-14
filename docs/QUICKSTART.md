# 快速开始

## 目标

扫描一个目录树，准确输出：

- 最大的子目录
- 最大的文件

## 运行

### 直接运行

```bash
go run . D:\
```

### 编译后运行

```bash
go build -o syskit
./syskit /home/user
```

Windows:

```powershell
go build -o syskit.exe
.\syskit.exe D:\
```

## 常用示例

```bash
# 返回前 20 个子目录和文件
syskit D:\

# 返回前 50 个结果
syskit --top 50 D:\

# 排除依赖目录
syskit --exclude node_modules,.git,vendor,target D:\

# 只看文件
syskit --include-dirs=false D:\

# 只看子目录
syskit --include-files=false D:\
```

## 导出

```bash
# JSON
syskit --format json D:\ > result.json

# CSV
syskit --export-csv report D:\
```

会生成：

- `report_dirs.csv`
- `report_files.csv`

## 本地 PowerShell 包装脚本

```powershell
scripts\find-largest-local.ps1 -Path D:\
scripts\find-largest-local.ps1 -Path D:\ -Top 30
```

## 注意

- 目录结果不包含根目录本身
- 程序会跳过权限不足的条目
- 程序会跳过符号链接，避免递归循环

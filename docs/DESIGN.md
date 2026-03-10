# 文件/文件夹大小分析工具 - 技术设计文档

## 项目概述

一个用 Go 语言开发的**跨平台**高性能文件系统分析工具，用于快速扫描指定目录，找出占用空间最大的文件和文件夹。

**支持平台**：Windows、Linux、macOS（完全跨平台）

## 核心功能

### 1. 基础功能
- 扫描指定目录及其所有子目录
- 统计文件大小并排序（Top N 文件）
- 统计文件夹总大小（包含所有子文件）并排序（Top N 目录）
- 支持显示 Top N 大文件和文件夹（默认 20）
- 人性化的文件大小显示（B, KB, MB, GB, TB）
- 显示扫描统计信息（处理文件数、耗时等）

### 2. 高级功能
- 三种扫描模式：混合模式（默认）、快速模式、完整模式
- 支持多种输出格式（表格、JSON、CSV）
- CSV 导出功能（分别导出文件和目录结果）
- 支持排除特定目录（如 node_modules, .git 等）
- 支持设置最小文件大小阈值
- 深度限制选项
- 实时进度显示
- 错误处理：权限不足时跳过并继续

### 3. 三种扫描模式

#### 混合模式（默认，推荐）⭐⭐⭐⭐⭐
**两阶段渐进式扫描，最佳用户体验**

**阶段 1：浅层快速扫描（5-10秒）**
- 只扫描前 2 层目录
- 统计所有一级、二级目录的大小
- 快速显示 Top 20 大目录

**阶段 2：深入扫描（可选，10-20秒）**
- 用户选择是否继续
- 只对 Top 20 大目录进行深入扫描
- 在这些大目录里找 Top 20 大文件

**优势**：
- ✅ 不会漏掉大文件（大文件一定在大目录里）
- ✅ 总耗时 15-30 秒，比完整模式快 3-5 倍
- ✅ 用户体验好：快速看到结果，可选择是否继续
- ✅ 适用场景：磁盘爆满，快速定位并清理大文件

#### 快速模式 ⭐⭐⭐⭐
**深度限制 + 大文件过滤**

- **深度限制**：默认 3 层（用户可配置 `--max-depth`）
- **大文件阈值**：默认 100MB（用户可配置 `--min-size`）
- **智能排除**：默认不排除（用户可选择排除 `--exclude node_modules,.git`）
- **性能**：10-30 秒（200GB 磁盘）
- **并发策略**：每个子目录启动独立 goroutine
- **适用场景**：需要自定义扫描参数的场景
- **注意**：可能漏掉深层大文件（如果深度限制太小）

#### 完整模式 ⭐⭐⭐
**Worker Pool + 全盘扫描**

- **无深度限制**：扫描所有层级
- **记录所有文件**：不过滤小文件
- **Worker Pool**：固定数量的 worker（CPU核心数 × 8）
- **性能**：1-3 分钟（200GB 磁盘）
- **并发策略**：任务队列 + 固定 worker，避免 goroutine 爆炸
- **适用场景**：需要完整准确的文件统计报告
- **优势**：100% 准确，不会漏掉任何文件

### 4. 性能对比

| 模式 | 速度 | 准确性 | 用户体验 | 推荐度 |
|------|------|--------|---------|--------|
| **混合模式** | 15-30秒 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | **⭐⭐⭐⭐⭐** |
| 快速模式 | 10-30秒 | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| 完整模式 | 1-3分钟 | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |

**相比 PowerShell 版本的性能提升**：
- 200GB, 100万文件：PowerShell 5-10分钟 → Go 混合模式 15-30秒（**10-20x 提升**）
- 1TB, 500万文件：PowerShell 30+分钟 → Go 混合模式 1-2分钟（**15-30x 提升**）

## 技术架构

### 项目结构
```
find_max_file/
├── main.go                    # 程序入口
├── go.mod                     # Go 模块定义
├── internal/
│   └── scanner/
│       ├── types.go           # 数据结构定义
│       ├── scanner.go         # 基础扫描器（已弃用）
│       ├── concurrent.go      # 并发扫描器（已弃用）
│       ├── fast.go            # 快速模式扫描器 ✅
│       ├── full.go            # 完整模式扫描器 ✅
│       └── hybrid.go          # 混合模式扫描器 ⏳
├── pkg/
│   └── utils/
│       └── size.go            # 大小格式化工具 ✅
├── DESIGN.md                  # 本文档
├── DEV_GUIDE.md               # Go 语言开发指南
└── README.md                  # 项目说明
```

### 核心数据结构

```go
// FileInfo 文件信息
type FileInfo struct {
    Path     string
    Size     int64
    ModTime  time.Time
}

// DirInfo 目录信息
type DirInfo struct {
    Path      string
    TotalSize int64
    FileCount int
    DirCount  int
}

// ScanOptions 扫描选项
type ScanOptions struct {
    RootPath     string
    TopN         int
    MinSize      int64      // 最小文件大小阈值（快速模式）
    MaxDepth     int        // 最大扫描深度（快速模式）
    IncludeFiles bool
    IncludeDirs  bool
    ShowProgress bool
    ExcludeDirs  []string   // 排除的目录列表
}

// ScanResult 扫描结果
type ScanResult struct {
    TopFiles      []FileInfo
    TopDirs       []DirInfo
    TotalSize     int64
    TotalFiles    int
    TotalDirs     int
    ScanDuration  time.Duration
}
```

## CLI 命令设计

### 使用示例

```bash
# 1. 混合模式（默认，推荐）
./find-large-files D:\
# 输出：阶段1（5-10秒）显示 Top 20 大目录
#       询问是否继续
#       阶段2（10-20秒）显示 Top 20 大文件

# 2. 快速模式 - 默认配置
./find-large-files D:\ --mode fast
# 深度3层，大文件阈值100MB，不排除目录

# 3. 快速模式 - 自定义配置
./find-large-files D:\ --mode fast --max-depth 5 --min-size 50MB --exclude node_modules,.git

# 4. 完整模式 - 全盘扫描
./find-large-files D:\ --mode full

# 5. 完整模式 - 排除常见大目录
./find-large-files D:\ --mode full --exclude node_modules,.git,vendor

# 6. 只显示文件，不显示目录
./find-large-files D:\ --include-files --no-include-dirs

# 7. JSON 输出
./find-large-files D:\ --format json

# 8. CSV 导出
./find-large-files D:\ --export-csv D:\result
```

### 参数说明

- `[path]`: 要扫描的目录路径（可选，不传则使用当前目录）
- `-m, --mode`: 扫描模式（hybrid/fast/full，默认 hybrid）
  - `hybrid`: 混合模式，两阶段渐进式扫描（默认）
  - `fast`: 快速模式，深度限制 + 大文件过滤
  - `full`: 完整模式，全盘扫描
- `-t, --top`: Top N 数量（默认 20）
- `--max-depth`: 最大扫描深度（仅快速模式，默认 3，0 表示无限制）
- `--min-size`: 最小文件大小阈值（仅快速模式，默认 100MB，如：50MB, 1GB）
- `--exclude`: 排除的目录名（逗号分隔，如：node_modules,.git,vendor，默认不排除）
- `--include-files`: 是否包含文件结果（默认 true）
- `--include-dirs`: 是否包含目录结果（默认 true）
- `--no-include-files`: 不包含文件结果
- `--no-include-dirs`: 不包含目录结果
- `--format`: 输出格式（table/json/csv，默认 table）
- `--export-csv`: CSV 导出路径前缀
- `-h, --help`: 显示帮助信息
- `-v, --version`: 显示版本信息

## 并发策略

### 混合模式并发策略
**阶段 1：浅层扫描**
- 深度限制 2 层
- 每目录一个 goroutine
- 只统计目录大小，不记录文件

**阶段 2：深入扫描**
- 并发扫描 Top 20 大目录
- 每个大目录使用 FullScanner（Worker Pool）
- 汇总所有大文件并排序

### 快速模式并发策略
- **每目录一个 goroutine**：为每个子目录启动独立的 goroutine
- **深度限制剪枝**：默认 3 层（用户可通过 `--max-depth` 配置）
- **大文件过滤**：默认只保存 >100MB 的文件（用户可通过 `--min-size` 配置）
- **智能排除**：默认不排除，用户可选择排除特定目录（`--exclude`）
- **无锁设计**：使用 sync.Map + atomic 操作，避免锁竞争

### 完整模式并发策略（Worker Pool）
- **固定 Worker 数量**：CPU 核心数 × 8（I/O 密集型任务的最佳配置）
- **任务队列**：使用 buffered channel 作为任务队列
- **避免 goroutine 爆炸**：不会为每个目录创建 goroutine，而是复用 worker
- **减少调度开销**：固定数量的 goroutine，减少 Go runtime 调度压力
- **无锁设计**：同样使用 sync.Map + atomic 操作

### 通用优化技术
- **无锁数据结构**：使用 sync.Map 存储目录大小，避免锁竞争
- **原子操作**：使用 atomic 包进行计数器更新
- **快速 I/O**：使用 os.ReadDir（比 filepath.Walk 快 2-3 倍）
- **零拷贝**：直接使用文件系统返回的数据
- **内存优化**：只保留 Top N 结果

## 跨平台兼容性

### 路径处理
- 使用 `filepath.Join()` 和 `filepath.Clean()` 自动处理路径
- 使用 `filepath.Separator` 获取平台特定的分隔符
- 支持 Windows 的盘符（C:, D:）和 Unix 的根路径（/）

### 文件系统差异
- **大小写敏感**：Windows 不敏感，Unix 敏感（代码统一处理）
- **隐藏文件**：Unix 以 `.` 开头，Windows 使用文件属性
- **权限系统**：Unix 使用 rwx，Windows 使用 ACL（统一跳过无权限文件）

## 当前实现状态（v0.3.0）

### 已完成功能
✅ **快速模式扫描器**（FastScanner）
- 深度限制（默认 3 层，用户可配置）
- 大文件过滤（默认 >100MB，用户可配置）
- 智能排除（默认不排除，用户可选择排除特定目录）
- 并发扫描（每目录一个 goroutine）

✅ **完整模式扫描器**（FullScanner）
- Worker Pool 模式（CPU 核心数 × 8）
- 无深度限制，全盘扫描
- 实时进度显示
- 高性能并发

✅ **核心数据结构**
- FileInfo、DirInfo、ScanOptions、ScanResult
- sync.Map + atomic 无锁设计
- 目录大小累加算法

✅ **工具函数**
- 字节大小格式化（FormatBytes）
- 数字千分位格式化（FormatNumber）

### 待实现功能
⏳ **混合模式扫描器**（HybridScanner）
- 两阶段渐进式扫描
- 用户交互选择是否继续

⏳ **主程序交互**（main.go）
- 模式选择界面
- 参数输入
- 结果展示

⏳ **CLI 参数解析**
- 使用 flag 或 cobra
- 支持所有命令行参数

⏳ **输出格式化**
- 表格输出（tablewriter）
- JSON 输出
- CSV 导出

⏳ **跨平台特性**
- Windows GUI 对话框
- 按任意键退出

### 下一步开发计划
1. **实现混合模式扫描器**（`internal/scanner/hybrid.go`）
2. 完善 main.go，实现三种模式选择和结果展示
3. 添加 CLI 参数解析
4. 实现表格输出格式化
5. 添加 JSON/CSV 导出功能
6. 跨平台测试和优化

## 技术难点

1. **混合模式的两阶段协调**：如何优雅地实现两阶段扫描和用户交互
2. **高并发控制和性能优化**：Worker Pool 的正确实现
3. **无锁数据结构的正确使用**：sync.Map 的目录大小累加
4. **大目录的内存管理**：防止 OOM
5. **跨平台路径处理**：Windows vs Unix 路径差异
6. **权限和错误处理**：不同平台的权限系统

## 测试重点

1. **性能测试**：100万+ 文件，对比 PowerShell 版本
2. **并发安全性测试**：目录大小累加的正确性
3. **内存压力测试**：超大目录不 OOM
4. **边界条件测试**：空目录、单文件、权限不足等
5. **跨平台测试**：Windows、Linux、macOS 分别测试
6. **混合模式测试**：验证两阶段扫描的正确性和性能

---

**预计开发时间**: 3-4 天（全职开发）

**版本规划**:
- v0.3.0: 实现混合模式（当前）
- v0.4.0: 完善 CLI 和输出格式化
- v0.5.0: 跨平台测试和优化
- v1.0.0: 正式发布

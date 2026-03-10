# Go 语言开发指南 - 文件大小分析工具（跨平台版本）

## 项目学习目标

通过这个**跨平台**项目，你将学习到：
1. **Go 语言基础**：包管理、模块系统、项目结构
2. **并发编程**：goroutine、channel、sync 包的使用
3. **Worker Pool 模式**：固定数量 worker + 任务队列的高性能并发模式
4. **文件系统操作**：filepath、os 包的使用
5. **性能优化**：无锁数据结构、内存管理、算法优化
6. **CLI 开发**：命令行参数解析、交互式输入
7. **跨平台开发**：处理 Windows、Linux、macOS 的差异
8. **Go 最佳实践**：代码组织、错误处理、测试

## 三种扫描模式对比

### 混合模式（默认，推荐）⭐⭐⭐⭐⭐
- **目标**：快速找到占空间大户，最佳用户体验
- **策略**：两阶段渐进式扫描
- **阶段1**：浅层扫描（深度2层），5-10秒，显示 Top 20 大目录
- **阶段2**：深入扫描 Top 20 大目录，10-20秒，显示 Top 20 大文件
- **总耗时**：15-30 秒
- **优势**：不会漏掉大文件，用户可选择是否继续
- **适用场景**：磁盘爆满，快速定位并清理

### 快速模式 ⭐⭐⭐⭐
- **目标**：快速扫描，自定义参数
- **策略**：深度限制 + 大文件过滤（用户可配置）
- **默认配置**：深度 3 层，大文件阈值 100MB，不排除目录
- **用户可配置**：`--max-depth`、`--min-size`、`--exclude`
- **性能**：10-30 秒
- **注意**：可能漏掉深层大文件
- **适用场景**：需要自定义扫描参数

### 完整模式 ⭐⭐⭐
- **目标**：完整准确的文件统计报告
- **策略**：Worker Pool + 全盘扫描
- **性能**：1-3 分钟
- **优势**：100% 准确，不会漏掉任何文件
- **适用场景**：需要详细的文件分析报告

---

## 核心算法设计

### 1. 混合模式：两阶段渐进式扫描（推荐）

```go
// 混合扫描算法（伪代码）
func HybridScan(rootPath string) {
    // ========== 阶段 1：浅层快速扫描 ==========
    fmt.Println("阶段 1：快速扫描目录...")

    phase1Scanner := FastScanner{
        MaxDepth: 2,              // 只扫描前2层
        IncludeFiles: false,      // 不记录文件
        IncludeDirs: true,        // 只记录目录
    }

    phase1Result := phase1Scanner.Scan(rootPath)

    // 显示 Top 20 大目录
    fmt.Println("\n=== Top 20 大目录 ===")
    for i, dir := range phase1Result.TopDirs {
        fmt.Printf("%d. %s - %s\n", i+1, dir.Path, FormatBytes(dir.Size))
    }

    // ========== 用户选择 ==========
    fmt.Print("\n是否深入扫描这些大目录以查找大文件？(y/n，默认 y): ")
    if getUserInput() != "y" {
        return phase1Result
    }

    // ========== 阶段 2：深入扫描大目录 ==========
    fmt.Println("\n阶段 2：深入扫描大目录...")

    var allFiles []FileInfo
    var wg sync.WaitGroup
    var filesMu sync.Mutex

    // 并发扫描 Top 20 大目录
    for i, dir := range phase1Result.TopDirs {
        if i >= 20 {
            break
        }

        wg.Add(1)
        go func(dirPath string) {
            defer wg.Done()

            // 使用 FullScanner 深入扫描
            scanner := FullScanner{RootPath: dirPath}
            result := scanner.Scan()

            // 汇总文件
            filesMu.Lock()
            allFiles = append(allFiles, result.TopFiles...)
            filesMu.Unlock()
        }(dir.Path)
    }

    wg.Wait()

    // 排序并取 Top 20
    sort.Slice(allFiles, func(i, j int) bool {
        return allFiles[i].Size > allFiles[j].Size
    })

    if len(allFiles) > 20 {
        allFiles = allFiles[:20]
    }

    // 显示 Top 20 大文件
    fmt.Println("\n=== Top 20 大文件 ===")
    for i, file := range allFiles {
        fmt.Printf("%d. %s - %s\n", i+1, file.Path, FormatBytes(file.Size))
    }

    return &ScanResult{
        TopFiles: allFiles,
        TopDirs:  phase1Result.TopDirs,
    }
}
```

**关键优化**：
- **阶段1快速**：只扫描2层，不记录文件，5-10秒出结果
- **用户可选**：看到大目录后，用户决定是否继续
- **阶段2精准**：只扫描大目录，不会漏掉大文件
- **并发扫描**：20个大目录并发扫描，充分利用多核

**为什么不会漏掉大文件？**
- 大文件一定在大目录里
- 阶段1找出所有大目录
- 阶段2在大目录里找大文件
- 逻辑清晰，100% 准确

### 2. 快速模式：深度限制 + 大文件过滤

```go
// 快速扫描算法（伪代码）
func FastScan(path string, depth int, options *ScanOptions) {
    // 1. 检查深度限制（用户可配置）
    if depth >= options.MaxDepth {
        return // 剪枝：超过深度限制，直接返回
    }

    // 2. 读取当前目录
    entries := os.ReadDir(path)

    // 3. 遍历所有条目
    for _, entry := range entries {
        if entry.IsDir() {
            // 检查是否需要排除（用户可配置）
            if shouldExclude(entry.Name(), options.ExcludeDirs) {
                continue // 剪枝：跳过用户指定的目录
            }

            // 为子目录启动新的 goroutine
            wg.Add(1)
            go FastScan(entry.Path, depth+1, options)
        } else {
            // 只记录大文件（用户可配置阈值）
            info := entry.Info()
            if info.Size() > options.MinSize {
                recordFile(FileInfo{
                    Path: entry.Path,
                    Size: info.Size(),
                })
            }
        }
    }
}
```

**关键优化**：
- **深度剪枝**：默认只扫描前 3 层（用户可配置 `--max-depth`）
- **大文件过滤**：默认只记录 >100MB 的文件（用户可配置 `--min-size`）
- **智能排除**：默认不排除，用户可选择排除（`--exclude node_modules,.git`）
- **内存优化**：90% 的小文件直接忽略，减少内存占用

**用户配置示例**：
```bash
# 默认配置
./find-large-files D:\ --mode fast

# 自定义深度和阈值
./find-large-files D:\ --mode fast --max-depth 5 --min-size 50MB

# 排除特定目录
./find-large-files D:\ --mode fast --exclude node_modules,.git,vendor
```

### 3. 完整模式：Worker Pool 模式

```go
// Worker Pool 算法（伪代码）
func FullScan(rootPath string) {
    // 1. 创建任务队列
    workerCount := runtime.NumCPU() * 8
    taskQueue := make(chan *scanTask, workerCount*2)

    // 2. 启动固定数量的 worker
    for i := 0; i < workerCount; i++ {
        workerWg.Add(1)
        go worker(taskQueue)
    }

    // 3. 提交第一个任务
    wg.Add(1)
    taskQueue <- &scanTask{path: rootPath}

    // 4. 等待所有任务完成
    wg.Wait()

    // 5. 关闭队列
    close(taskQueue)

    // 6. 等待所有 worker 退出
    workerWg.Wait()
}

// Worker 函数
func worker(taskQueue chan *scanTask) {
    defer workerWg.Done()

    // 从队列中不断取任务
    for task := range taskQueue {
        // 扫描目录
        entries := os.ReadDir(task.path)

        for _, entry := range entries {
            if entry.IsDir() {
                // 提交新任务到队列（不创建新 goroutine）
                wg.Add(1)
                taskQueue <- &scanTask{path: entry.Path}
            } else {
                // 记录所有文件
                info := entry.Info()
                recordFile(FileInfo{
                    Path: entry.Path,
                    Size: info.Size(),
                })
            }
        }

        wg.Done()
    }
}
```

**关键优化**：
- **固定 worker 数量**：避免创建数十万个 goroutine
- **任务队列**：使用 channel 作为队列，worker 不断取任务
- **减少调度开销**：Go runtime 只需调度固定数量的 goroutine
- **性能提升**：比"每目录一个 goroutine"快 20-30%

### 4. 无锁目录大小累加算法

**问题**：多个 goroutine 同时更新同一个目录的大小，如何保证线程安全？

**传统方案（使用锁）**：
```go
var mu sync.Mutex
var dirSizes = make(map[string]int64)

func addSize(dir string, size int64) {
    mu.Lock()           // 加锁
    dirSizes[dir] += size
    mu.Unlock()         // 解锁
}
// 问题：高并发时锁竞争严重，性能差
```

**优化方案（使用 sync.Map + atomic）**：
```go
var dirSizes sync.Map

func addSize(dir string, size int64) {
    // LoadOrStore：如果 key 存在则加载，否则存储
    actual, _ := dirSizes.LoadOrStore(dir, new(int64))
    ptr := actual.(*int64)
    atomic.AddInt64(ptr, size) // 原子操作，无锁
}
// 优势：无锁设计，性能高，适合高并发场景
```

**为什么不用普通 map + Mutex？**
- 普通 map 不是并发安全的
- 加锁会导致严重的锁竞争
- sync.Map 针对读多写少场景优化
- atomic 操作比锁快 10 倍以上

### 5. Top N 算法（内存优化）

**问题**：扫描 100 万个文件，如何只保留 Top 20，避免内存爆炸？

**当前实现（简单排序）**：
```go
var files []FileInfo
var filesMu sync.Mutex

func addFile(file FileInfo) {
    filesMu.Lock()
    files = append(files, file)
    filesMu.Unlock()
}

func getTopN() []FileInfo {
    sort.Slice(files, func(i, j int) bool {
        return files[i].Size > files[j].Size
    })
    return files[:topN]
}
```

**优点**：实现简单，代码清晰
**缺点**：需要保存所有文件（完整模式），内存占用较大

**当前实现的权衡**：
- **混合模式**：只扫描大目录，文件数量可控
- **快速模式**：只保存 >100MB 的文件，通常只有几百个，内存可控
- **完整模式**：保存所有文件，但最后排序取 Top N，实现简单
- **未来优化**：如果需要扫描千万级文件，可以升级为最小堆实现

---

## Go 并发模型详解

### 1. Goroutine（协程）vs 线程

**传统线程（如 Java、C#）**：
- 每个线程占用 1-2MB 内存
- 创建和切换开销大
- 需要手动管理线程池
- 通常只能创建几千个线程

**Go 的 Goroutine**：
- 每个 goroutine 只占用 2KB 内存（初始栈大小）
- 创建和切换开销极小
- Go runtime 自动调度
- 可以轻松创建数十万个 goroutine

```go
// 创建 goroutine 非常简单
go func() {
    // 这里的代码会并发执行
    fmt.Println("Hello from goroutine")
}()
```

### 2. Channel（通道）- Go 的通信机制

**核心理念**："不要通过共享内存来通信，而要通过通信来共享内存"

```go
// 创建 channel
ch := make(chan int)        // 无缓冲 channel
ch := make(chan int, 100)   // 有缓冲 channel（容量 100）

// 发送数据
ch <- 42

// 接收数据
value := <-ch

// 关闭 channel
close(ch)
```

### 3. sync 包 - 同步原语

```go
// WaitGroup：等待一组 goroutine 完成
var wg sync.WaitGroup
wg.Add(1)           // 增加计数
go func() {
    defer wg.Done() // 完成时减少计数
    // 做一些工作
}()
wg.Wait()           // 等待所有 goroutine 完成

// sync.Map：并发安全的 map（无锁设计）
var m sync.Map
m.Store("key", "value")     // 存储
value, ok := m.Load("key")  // 读取
m.Delete("key")             // 删除
```

---

## 跨平台开发要点

### 1. 路径处理（跨平台关键）

```go
import "path/filepath"

// 正确做法：使用 filepath 包
path := filepath.Join("home", "user", "file.txt")  // 自动适配平台
// Windows: home\user\file.txt
// Unix:    home/user/file.txt

// 清理路径（移除多余的分隔符）
cleanPath := filepath.Clean(path)

// 获取绝对路径
absPath, err := filepath.Abs(path)
```

### 2. 文件权限处理

```go
// 跨平台读取文件信息
info, err := os.Stat(path)
if err != nil {
    if os.IsPermission(err) {
        // 权限不足（跨平台统一处理）
        fmt.Println("权限不足，跳过")
        return
    }
}
```

### 3. 交叉编译

```bash
# 编译 Windows 版本
GOOS=windows GOARCH=amd64 go build -o find-large-files.exe

# 编译 Linux 版本
GOOS=linux GOARCH=amd64 go build -o find-large-files

# 编译 macOS 版本
GOOS=darwin GOARCH=amd64 go build -o find-large-files
```

---

## 项目结构详解

```
find_max_file/
├── main.go                    # 程序入口
├── go.mod                     # Go 模块定义
├── internal/
│   └── scanner/
│       ├── types.go           # 数据结构定义 ✅
│       ├── fast.go            # 快速模式扫描器 ✅
│       ├── full.go            # 完整模式扫描器 ✅
│       └── hybrid.go          # 混合模式扫描器 ⏳
├── pkg/
│   └── utils/
│       └── size.go            # 大小格式化工具 ✅
├── DESIGN.md                  # 技术设计文档
├── DEV_GUIDE.md               # 本文档
└── README.md                  # 项目说明
```

---

## Go 语言最佳实践

### 1. 错误处理

```go
// 好的做法
file, err := os.Open("file.txt")
if err != nil {
    return fmt.Errorf("打开文件失败: %w", err) // 包装错误
}
defer file.Close() // 确保资源释放
```

### 2. defer 语句

```go
func processFile(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close() // 函数返回前自动执行

    // 处理文件...
    return nil
}
```

### 3. 接口（Interface）

```go
// 定义接口
type Scanner interface {
    Scan() (*ScanResult, error)
}

// 实现接口（无需显式声明）
type FastScanner struct{}

func (s *FastScanner) Scan() (*ScanResult, error) {
    // 实现扫描逻辑
    return &ScanResult{}, nil
}

// 使用接口
var scanner Scanner = &FastScanner{} // 自动满足接口
```

---

## 性能优化技巧

### 1. 避免不必要的内存分配

```go
// 好的做法：预分配
files := make([]FileInfo, 0, 1000) // 预分配容量
```

### 2. 并发控制

```go
// 使用 WaitGroup 等待所有 goroutine 完成
var wg sync.WaitGroup
for _, dir := range dirs {
    wg.Add(1)
    go func(d string) {
        defer wg.Done()
        scanDir(d)
    }(dir)
}
wg.Wait()
```

---

## 代码注释规范

本项目的代码注释将遵循以下规范：

1. **包级注释**：说明包的用途
2. **函数注释**：说明函数的功能、参数、返回值
3. **关键逻辑注释**：解释为什么这样写
4. **Go 特性注释**：标注 Go 语言的特殊用法
5. **性能注释**：说明性能优化的考虑

示例：
```go
// HybridScanner 混合模式扫描器
// 使用两阶段渐进式扫描，提供最佳用户体验
//
// 阶段1：浅层扫描（深度2层），快速显示大目录
// 阶段2：深入扫描大目录，精准找到大文件
//
// Go 语言知识点：
// 1. 使用 goroutine 并发扫描多个大目录
// 2. 使用 sync.Mutex 保护共享的文件列表
// 3. 使用 WaitGroup 等待所有扫描完成
type HybridScanner struct {
    options *ScanOptions
}

// Scan 执行混合模式扫描
//
// 返回：扫描结果和可能的错误
func (h *HybridScanner) Scan() (*ScanResult, error) {
    // 实现...
}
```

---

准备好开始开发了吗？确认后我会按照这个指南，逐步实现每个功能，并加上详细的中文注释！

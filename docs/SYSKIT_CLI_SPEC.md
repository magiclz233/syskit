# syskit CLI 命令规范文档

## 文档信息
- 文档版本：v1.1
- 对应 PRD：v1.3
- 文档日期：2026-03-11
- 目标读者：研发、测试、技术写作

## 1. 全局参数

所有命令支持以下全局参数：

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--output` | string | `table` | 输出格式：`table`/`json`/`markdown` |
| `--json` | bool | `false` | `--output json` 的快捷写法 |
| `--config` | string | 见配置章节 | 指定配置文件路径 |
| `--policy` | string | 见配置章节 | 指定策略文件路径 |
| `--verbose` / `-v` | bool | `false` | 详细输出模式 |
| `--quiet` / `-q` | bool | `false` | 静默模式，仅输出结果 |
| `--no-color` | bool | `false` | 禁用彩色输出 |
| `--help` / `-h` | bool | - | 显示帮助信息 |
| `--version` | bool | - | 显示版本信息 |

## 2. 命令分组与别名

### 2.1 inspect 命令组（只读检查）

#### 2.1.1 端口检查

**完整命令**：`syskit inspect port <port>`
**别名**：`syskit port <port>`

查询指定端口的占用情况。

**参数**：
- `<port>`：必填，端口号（1-65535）

**示例**：
```bash
syskit port 8080
syskit inspect port 8080 --output json
```

**输出字段**（JSON）：
```json
{
  "port": 8080,
  "status": "listening",
  "protocol": "tcp",
  "pid": 12345,
  "process_name": "node",
  "command": "node server.js",
  "user": "developer",
  "start_time": "2026-03-11T09:30:00Z",
  "parent_pid": 1234,
  "parent_name": "bash",
  "process_type": "dev_tool"
}
```

---

**完整命令**：`syskit inspect ports`
**别名**：`syskit ports`

列出所有监听端口。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--filter` | string | - | 过滤条件：`listening`/`established`/`all` |
| `--protocol` | string | `all` | 协议过滤：`tcp`/`udp`/`all` |
| `--limit` | int | 50 | 最多显示条数 |

**示例**：
```bash
syskit ports
syskit ports --filter listening --protocol tcp
```

---

#### 2.1.2 进程检查

**完整命令**：`syskit inspect proc top`
**别名**：`syskit top`

显示资源占用 Top 进程。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--by` | string | `cpu` | 排序维度：`cpu`/`mem`/`io` |
| `--limit` | int | 10 | 显示进程数 |
| `--threshold` | float | 0 | 最低占用阈值（百分比） |

**示例**：
```bash
syskit top
syskit top --by mem --limit 20
syskit inspect proc top --by cpu --threshold 5.0
```

---

**完整命令**：`syskit inspect proc tree <pid>`

显示进程树。

**参数**：
- `<pid>`：可选，根进程 PID，不指定则显示完整进程树

**示例**：
```bash
syskit inspect proc tree 12345
```

---

#### 2.1.3 内存检查

**完整命令**：`syskit inspect mem`
**别名**：`syskit mem`

显示内存使用情况。

**输出字段**（JSON）：
```json
{
  "total_mb": 16384,
  "used_mb": 12288,
  "free_mb": 4096,
  "available_mb": 5120,
  "usage_percent": 75.0,
  "swap_total_mb": 8192,
  "swap_used_mb": 1024,
  "swap_usage_percent": 12.5
}
```

---

#### 2.1.4 磁盘检查

**完整命令**：`syskit inspect disk`
**别名**：`syskit disk`

显示磁盘使用情况与风险。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--path` | string | - | 指定路径，不指定则显示所有分区 |
| `--show-growth` | bool | `false` | 显示增长趋势（需历史数据） |

**示例**：
```bash
syskit disk
syskit disk --path /var/log --show-growth
```

**输出字段**（JSON）：
```json
{
  "partitions": [
    {
      "mount_point": "/",
      "device": "/dev/sda1",
      "fstype": "ext4",
      "total_gb": 500,
      "used_gb": 450,
      "free_gb": 50,
      "usage_percent": 90.0,
      "inodes_total": 32000000,
      "inodes_used": 1500000,
      "inodes_usage_percent": 4.7,
      "growth_rate_gb_per_day": 2.5
    }
  ]
}
```

---

#### 2.1.5 网络检查

**完整命令**：`syskit inspect network`

显示网络连接统计。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--group-by` | string | `process` | 分组方式：`process`/`state`/`remote` |
| `--limit` | int | 20 | 最多显示条数 |

**示例**：
```bash
syskit inspect network
syskit inspect network --group-by state
```

---

#### 2.1.6 文件检查

**完整命令**：`syskit inspect file large <path>`

查找大文件。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `<path>` | string | `.` | 扫描路径 |
| `--min-size` | string | `100M` | 最小文件大小（支持 K/M/G） |
| `--limit` | int | 50 | 最多显示条数 |
| `--depth` | int | 5 | 最大扫描深度 |

**示例**：
```bash
syskit inspect file large /var/log --min-size 500M
```

---

**完整命令**：`syskit inspect file duplicate <path>`

查找重复文件。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `<path>` | string | `.` | 扫描路径 |
| `--min-size` | string | `1M` | 最小文件大小 |
| `--algorithm` | string | `sha256` | 哈希算法：`md5`/`sha256` |

---

**完整命令**：`syskit inspect file dir <path>`

查找大目录（按目录累计体积排序）。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `<path>` | string | `.` | 扫描路径 |
| `--min-size` | string | `1G` | 最小目录体积（支持 K/M/G） |
| `--limit` | int | 20 | 最多显示条数 |
| `--depth` | int | 8 | 最大扫描深度 |

---

**完整命令**：`syskit inspect file types <path>`

输出文件类型统计（数量、体积、占比）。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `<path>` | string | `.` | 扫描路径 |
| `--group-by` | string | `ext` | 分组方式：`ext`/`mime` |
| `--limit` | int | 20 | 最多显示条数 |

---

### 2.2 doctor 命令组（场景化诊断）

#### 2.2.1 一键体检

**完整命令**：`syskit doctor all`

执行全面系统诊断。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--mode` | string | `quick` | 扫描模式：`quick`/`deep` |
| `--severity` | string | `all` | 过滤级别：`critical`/`high`/`medium`/`low`/`all` |
| `--skip` | []string | - | 跳过模块：`port,cpu,mem,disk,network,io` |

**示例**：
```bash
syskit doctor all
syskit doctor all --mode deep --output json
syskit doctor all --mode quick --json
syskit doctor all --severity critical,high
syskit doctor all --skip network,io
```

**输出结构**（JSON）：
```json
{
  "timestamp": "2026-03-11T10:00:00Z",
  "host": "dev-machine",
  "mode": "quick",
  "health_score": 75,
  "health_level": "warning",
  "coverage": 100.0,
  "issues": [
    {
      "rule_id": "PORT-001",
      "severity": "critical",
      "summary": "端口 8080 被非预期进程占用",
      "evidence": {
        "port": 8080,
        "pid": 12345,
        "process": "node",
        "command": "node old-server.js",
        "start_time": "2026-03-10T15:30:00Z"
      },
      "impact": "新服务无法启动，影响本地开发调试",
      "suggestion": "终止旧进程或更改新服务端口",
      "fix_command": "syskit fix port 8080 --apply",
      "auto_fixable": true,
      "confidence": 95,
      "scope": "local"
    }
  ],
  "skipped_modules": [],
  "execution_time_ms": 3500
}
```

---

#### 2.2.2 端口诊断

**完整命令**：`syskit doctor port`

诊断端口冲突问题。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--port` | int | - | 指定端口，不指定则检查常见端口 |
| `--common-ports` | bool | `true` | 检查常见开发端口（3000/8080/8000/5000等） |

**示例**：
```bash
syskit doctor port --port 8080
syskit doctor port
```

**输出要求**：
1. 必须包含完整链路：`port -> pid -> process_name -> command -> parent_process -> start_time`。
2. 必须输出两种可执行处理路径：
- 保守路径：建议改服务端口。
- 直接路径：建议安全终止占用进程（可选自动修复）。

**输出结构**（JSON 示例）：
```json
{
  "rule_id": "PORT-001",
  "severity": "critical",
  "summary": "端口 8080 冲突",
  "evidence": {
    "port": 8080,
    "pid": 12345,
    "process_name": "node",
    "command": "node old-server.js",
    "parent_process": "bash(1234)",
    "start_time": "2026-03-10T15:30:00Z",
    "process_type": "dev_tool"
  },
  "suggestion_paths": [
    {
      "type": "conservative",
      "action": "修改目标服务端口",
      "command": "PORT=8081 npm run dev"
    },
    {
      "type": "direct",
      "action": "释放冲突端口",
      "command": "syskit fix port 8080 --apply"
    }
  ]
}
```

---

#### 2.2.3 CPU 诊断

**完整命令**：`syskit doctor cpu`

诊断 CPU 高占用问题。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--threshold` | float | 80.0 | CPU 占用阈值（百分比） |
| `--duration` | int | 10 | 采样时长（秒） |

**示例**：
```bash
syskit doctor cpu
syskit doctor cpu --threshold 90 --duration 30
```

---

#### 2.2.4 内存诊断

**完整命令**：`syskit doctor mem`

诊断内存不足问题。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--threshold` | float | 90.0 | 内存占用阈值（百分比） |

---

#### 2.2.5 IO 诊断

**完整命令**：`syskit doctor io`

诊断磁盘 IO 异常。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--duration` | int | 10 | 采样时长（秒） |

---

#### 2.2.6 磁盘诊断

**完整命令**：`syskit doctor disk`

诊断磁盘空间风险。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--threshold` | float | 85.0 | 磁盘占用阈值（百分比） |
| `--analyze-growth` | bool | `true` | 分析增长趋势 |

---

#### 2.2.7 网络诊断

**完整命令**：`syskit doctor network`

诊断网络连接异常。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--threshold` | int | 1000 | 连接数阈值 |

---

### 2.3 fix 命令组（自动修复）

**全局行为**：
- 默认 `--dry-run` 模式，仅显示将执行的操作
- 需要 `--apply` 才真正执行
- `destructive` 级别操作需要 `--apply --yes`
- 风险分级：`local`（当前用户影响）/`system`（系统级影响）/`destructive`（不可逆）
- `system` 与 `destructive` 动作需要管理员或 root 权限

#### 2.3.1 释放端口

**完整命令**：`syskit fix port <port>`

释放指定端口占用。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `<port>` | int | - | 必填，端口号 |
| `--apply` | bool | `false` | 执行修复（默认 dry-run） |
| `--force` | bool | `false` | 强制终止进程（即使是系统进程） |

**示例**：
```bash
syskit fix port 8080                    # dry-run
syskit fix port 8080 --apply            # 执行
syskit fix port 8080 --apply --force    # 强制执行
```

**输出**（dry-run）：
```
[DRY-RUN] Will terminate process:
  PID: 12345
  Name: node
  Command: node old-server.js
  Scope: local
  Risk: low (user-owned development process)

To execute: syskit fix port 8080 --apply
```

---

#### 2.3.2 清理临时文件

**完整命令**：`syskit fix cleanup`

清理临时文件与日志。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--target` | []string | `temp` | 清理目标：`temp`/`logs`/`cache` |
| `--age` | int | 7 | 保留天数 |
| `--apply` | bool | `false` | 执行清理 |
| `--yes` | bool | `false` | 跳过二次确认（destructive 操作需要） |

**示例**：
```bash
syskit fix cleanup --target temp,logs
syskit fix cleanup --target temp --age 3 --apply
syskit fix cleanup --target logs --apply --yes
```

---

### 2.4 monitor 命令组（持续监控）

#### 2.4.1 启动监控

**完整命令**：`syskit monitor start`

启动后台监控采样。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--interval` | int | 60 | 采样间隔（秒） |
| `--metrics` | []string | `all` | 监控指标：`cpu,mem,disk,network,all` |
| `--duration` | int | 0 | 监控时长（秒），0 表示持续 |

**示例**：
```bash
syskit monitor start --interval 30 --metrics cpu,mem
syskit monitor start --duration 3600
```

---

#### 2.4.2 停止监控

**完整命令**：`syskit monitor stop`

停止后台监控。

---

#### 2.4.3 查看监控状态

**完整命令**：`syskit monitor status`

显示监控运行状态。

---

### 2.5 snapshot 命令组（快照管理）

#### 2.5.1 创建快照

**完整命令**：`syskit snapshot create`

创建系统状态快照。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--name` | string | 自动生成 | 快照名称 |
| `--tag` | []string | - | 标签（如 `before-deploy`） |

**示例**：
```bash
syskit snapshot create --name baseline --tag production
```

---

#### 2.5.2 列出快照

**完整命令**：`syskit snapshot list`

列出所有快照。

---

#### 2.5.3 对比快照

**完整命令**：`syskit snapshot diff <snapshot1> <snapshot2>`

对比两个快照差异。

**参数**：
- `<snapshot1>`：快照名称或 ID
- `<snapshot2>`：快照名称或 ID，不指定则与当前状态对比

**示例**：
```bash
syskit snapshot diff baseline
syskit snapshot diff baseline current
```

---

#### 2.5.4 删除快照

**完整命令**：`syskit snapshot delete <snapshot>`

删除指定快照。

---

### 2.6 report 命令组（报告导出）

#### 2.6.1 生成报告

**完整命令**：`syskit report generate`

生成诊断报告。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--source` | string | `latest` | 数据源：`latest`/快照名称 |
| `--format` | string | `markdown` | 格式：`markdown`/`html`/`json` |
| `--output` | string | `stdout` | 输出路径 |
| `--include` | []string | `all` | 包含模块 |

**示例**：
```bash
syskit report generate --format html --output report.html
syskit report generate --source baseline --format markdown
```

---

### 2.7 policy 命令组（策略管理）

#### 2.7.1 验证策略

**完整命令**：`syskit policy validate`

验证当前系统是否符合策略。

**参数**：
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--policy` | string | 默认路径 | 策略文件路径 |
| `--fail-on` | string | `critical` | 失败级别：`critical`/`high`/`medium`/`low` |

**示例**：
```bash
syskit policy validate
syskit policy validate --policy team-policy.yaml --fail-on high
```

---

#### 2.7.2 显示策略

**完整命令**：`syskit policy show`

显示当前生效的策略配置。

---

## 3. 退出码规范

| 退出码 | 含义 | 使用场景 |
|---|---|---|
| `0` | 成功且无高风险问题 | 正常执行完成 |
| `1` | 执行失败 | 命令错误、权限不足、系统异常 |
| `2` | 成功但发现高风险问题 | CI/CD 质量门禁阻断 |

**CI/CD 集成建议**：
- 退出码 `2` 视为构建失败
- 模块部分失败通过 `skipped_modules` 与 `coverage` 字段表达，不新增退出码

---

## 4. 输出格式规范

### 4.1 Table 格式（默认）

人类友好的表格输出，带颜色标识：
- `critical`：红色
- `high`：橙色
- `medium`：黄色
- `low`：灰色

### 4.2 JSON 格式

机器友好的结构化输出，所有命令输出遵循统一结构：

```json
{
  "timestamp": "2026-03-11T10:00:00Z",
  "host": "hostname",
  "command": "syskit doctor all",
  "exit_code": 0,
  "data": { /* 命令特定数据 */ },
  "metadata": {
    "version": "1.0.0",
    "platform": "linux",
    "execution_time_ms": 1500
  }
}
```

### 4.3 Markdown 格式

适合报告归档与文档嵌入。

### 4.4 终端与 JSON 字段映射

| 终端输出 | JSON 字段 |
|---|---|
| `[HIGH] PORT-001 端口冲突` | `severity=high, rule_id=PORT-001, summary=端口冲突` |
| `Evidence:` | `evidence` |
| `Fix:` | `fix_command` |

---

## 5. 配置文件规范

### 5.1 配置文件路径优先级

1. 命令行 `--config` 指定
2. 用户级：`~/.config/syskit/config.yaml` (Linux/macOS) 或 `%APPDATA%/syskit/config.yaml` (Windows)
3. 系统级：`/etc/syskit/config.yaml` (Linux/macOS) 或 `%ProgramData%/syskit/config.yaml` (Windows)
4. 内置默认值

### 5.2 配置文件示例

```yaml
# 输出配置
output: table
no_color: false

# 数据保留
retention_days: 14
max_storage_mb: 500

# 风险控制
risk:
  require_confirm_for:
    - destructive
  auto_approve:
    - local

# 阈值配置
thresholds:
  cpu_percent: 80.0
  mem_percent: 90.0
  disk_percent: 85.0
  connection_count: 1000

# 监控配置
monitor:
  default_interval: 60
  default_metrics:
    - cpu
    - mem
    - disk

# 规则配置
rules:
  enabled:
    - PORT-001
    - DISK-001
  disabled: []
  custom_severity:
    PORT-001: critical
```

---

## 6. 策略文件规范

策略文件用于团队统一巡检标准。

### 6.1 策略文件示例

```yaml
name: "团队开发环境标准"
version: "1.0"
author: "DevOps Team"

# 必须通过的规则
required_rules:
  - rule_id: PORT-001
    max_severity: high
  - rule_id: DISK-001
    max_severity: critical

# 阈值覆盖
thresholds:
  cpu_percent: 85.0
  disk_percent: 90.0

# 禁止的进程
forbidden_processes:
  - name: "bitcoin-miner"
    severity: critical
  - name: "torrent"
    severity: high

# 必须运行的服务
required_services:
  - name: "docker"
    platform: ["linux", "darwin"]
  - name: "Docker Desktop"
    platform: ["windows"]
```

---

## 7. 环境变量

| 变量名 | 说明 | 示例 |
|---|---|---|
| `SYSKIT_CONFIG` | 配置文件路径 | `/path/to/config.yaml` |
| `SYSKIT_POLICY` | 策略文件路径 | `/path/to/policy.yaml` |
| `SYSKIT_OUTPUT` | 默认输出格式 | `json` |
| `SYSKIT_NO_COLOR` | 禁用颜色 | `1` |
| `SYSKIT_DATA_DIR` | 数据存储目录 | `/custom/path` |

---

## 8. 错误处理规范

### 8.1 错误输出格式

```json
{
  "error": {
    "code": "ERR_PERMISSION_DENIED",
    "message": "需要管理员权限执行此操作",
    "details": "fix 命令需要提升权限",
    "suggestion": "请使用 sudo 或管理员身份运行"
  }
}
```

### 8.2 常见错误码

| 错误码 | 说明 |
|---|---|
| `ERR_PERMISSION_DENIED` | 权限不足 |
| `ERR_INVALID_ARGUMENT` | 参数错误 |
| `ERR_NOT_FOUND` | 资源不存在 |
| `ERR_PLATFORM_UNSUPPORTED` | 平台不支持 |
| `ERR_EXECUTION_FAILED` | 执行失败 |
| `ERR_CONFIG_INVALID` | 配置文件无效 |

---

## 9. 平台差异说明

### 9.1 命令可用性

| 命令 | Windows | Linux | macOS |
|---|---|---|---|
| `inspect port` | ✓ | ✓ | ✓ |
| `inspect proc` | ✓ | ✓ | ✓ |
| `inspect mem` | ✓ | ✓ | ✓ |
| `inspect disk` | ✓ | ✓ | ✓ |
| `inspect network` | ✓ | ✓ | ✓ |
| `inspect file` | ✓ | ✓ | ✓ |
| `doctor all` | ✓ | ✓ | ✓ |
| `fix port` | ✓ | ✓ | ✓ |
| `fix cleanup` | ✓ | ✓ | ✓ |

### 9.2 平台特定行为

**Windows**：
- 进程终止使用 `taskkill`
- 临时目录：`%TEMP%`
- 需要管理员权限的操作会触发 UAC

**Linux/macOS**：
- 进程终止使用 `kill`
- 临时目录：`/tmp`
- 需要 root 权限的操作需要 `sudo`

---

## 10. 使用示例

### 10.1 快速排障流程

```bash
# 1. 一键体检
syskit doctor all

# 2. 针对性诊断
syskit doctor port --port 8080

# 3. 查看详细信息
syskit port 8080

# 4. 修复问题（dry-run）
syskit fix port 8080

# 5. 确认后执行
syskit fix port 8080 --apply
```

### 10.2 CI/CD 集成

```bash
# 构建前检查
syskit doctor all --mode quick --json > health.json
if [ $? -eq 2 ]; then
  echo "环境异常，阻断构建"
  exit 1
fi

# 策略验证
syskit policy validate --policy team-policy.yaml --fail-on high
```

### 10.3 定期巡检

```bash
# 创建基线快照
syskit snapshot create --name baseline --tag production

# 每日巡检
syskit doctor all --mode deep --json > daily-$(date +%Y%m%d).json

# 对比变化
syskit snapshot diff baseline
```

---

## 11. 版本兼容性

- 配置文件向后兼容，新版本可读取旧版本配置
- 命令别名永久保留，不会移除
- JSON 输出字段只增不减，新字段标记为 `optional`
- 退出码语义保持稳定

---

## 附录：完整命令树

```
syskit
├── inspect
│   ├── port <port>
│   ├── ports
│   ├── proc
│   │   ├── top
│   │   └── tree [pid]
│   ├── mem
│   ├── disk
│   ├── network
│   └── file
│       ├── large <path>
│       ├── duplicate <path>
│       ├── dir <path>
│       └── types <path>
├── doctor
│   ├── all
│   ├── port
│   ├── cpu
│   ├── mem
│   ├── io
│   ├── disk
│   └── network
├── fix
│   ├── port <port>
│   └── cleanup
├── monitor
│   ├── start
│   ├── stop
│   └── status
├── snapshot
│   ├── create
│   ├── list
│   ├── diff <snapshot1> [snapshot2]
│   └── delete <snapshot>
├── report
│   └── generate
└── policy
    ├── validate
    └── show
```

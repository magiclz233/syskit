// Package audit 提供真实写操作的审计日志落盘能力。
package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syskit/internal/errs"
	"syskit/internal/storage"
	"time"
)

// Event 表示一条审计事件。
// 该结构与 PRD/P0 约束对齐，覆盖写操作追溯所需核心字段。
type Event struct {
	Timestamp  time.Time      `json:"timestamp"`
	TraceID    string         `json:"trace_id,omitempty"`
	Operator   string         `json:"operator,omitempty"`
	Command    string         `json:"command"`
	Action     string         `json:"action"`
	Target     string         `json:"target,omitempty"`
	Before     any            `json:"before,omitempty"`
	After      any            `json:"after,omitempty"`
	Result     string         `json:"result"`
	ErrorMsg   string         `json:"error_msg,omitempty"`
	DurationMs int64          `json:"duration_ms,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Logger 负责将审计事件写入 data_dir/audit 下的 JSONL 文件。
type Logger struct {
	auditDir string
	now      func() time.Time
}

// NewLogger 使用 storage.data_dir 初始化审计日志写入器。
func NewLogger(dataDir string) (*Logger, error) {
	layout, err := storage.EnsureLayout(strings.TrimSpace(dataDir))
	if err != nil {
		return nil, err
	}
	return &Logger{
		auditDir: layout.AuditDir,
		now:      time.Now,
	}, nil
}

// Log 以 JSONL 方式落盘一条审计记录。
// 文件按日期滚动：YYYYMMDD.jsonl。
func (l *Logger) Log(ctx context.Context, event Event) error {
	if l == nil {
		return errs.InvalidArgument("审计日志写入器未初始化")
	}
	if strings.TrimSpace(event.Command) == "" {
		return errs.InvalidArgument("审计日志 command 不能为空")
	}
	if strings.TrimSpace(event.Action) == "" {
		return errs.InvalidArgument("审计日志 action 不能为空")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "写入审计日志已取消")
	default:
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = l.now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	if strings.TrimSpace(event.TraceID) == "" {
		event.TraceID = newTraceID()
	}
	if strings.TrimSpace(event.Operator) == "" {
		event.Operator = currentOperator()
	}
	if strings.TrimSpace(event.Result) == "" {
		event.Result = "success"
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return errs.ExecutionFailed("序列化审计日志失败", err)
	}
	path := filepath.Join(l.auditDir, event.Timestamp.Format("20060102")+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return errs.ExecutionFailed("打开审计日志文件失败", err)
	}
	defer file.Close()

	line := append(payload, '\n')
	if _, err := file.Write(line); err != nil {
		return errs.ExecutionFailed("写入审计日志失败", err)
	}
	return nil
}

func currentOperator() string {
	if current, err := user.Current(); err == nil {
		if name := strings.TrimSpace(current.Username); name != "" {
			return name
		}
	}
	for _, key := range []string{"SUDO_USER", "USERNAME", "USER"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return "unknown"
}

func newTraceID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

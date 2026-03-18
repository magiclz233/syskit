// Package output 负责把命令执行结果渲染成 table/json/markdown/csv 等格式。
package output

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"time"
)

const schemaVersion = "1.0"

// Presenter 定义“业务数据 -> 非 JSON 可读输出”的渲染接口。
// JSON 走统一结构体序列化；table/markdown/csv 则由各命令自己的 presenter 决定展示细节。
type Presenter interface {
	RenderTable(w io.Writer) error
	RenderMarkdown(w io.Writer) error
	RenderCSV(w io.Writer, prefix string) error
}

// NewSuccessResult 构造成功场景下的统一结果对象。
func NewSuccessResult(msg string, data any, startedAt time.Time) model.CommandResult {
	return model.CommandResult{
		Code:  0,
		Msg:   msg,
		Data:  data,
		Error: nil,
		Metadata: model.Metadata{
			SchemaVersion: schemaVersion,
			Timestamp:     time.Now().UTC(),
			Host:          hostname(),
			Command:       strings.Join(os.Args, " "),
			ExecutionMs:   time.Since(startedAt).Milliseconds(),
			Platform:      runtime.GOOS,
			TraceID:       newTraceID(),
		},
	}
}

// Render 根据 format 选择最终输出方式。
func Render(w io.Writer, format string, result model.CommandResult, presenter Presenter, csvPrefix string) error {
	switch format {
	case "json":
		return renderJSON(w, result)
	case "table":
		return presenter.RenderTable(w)
	case "markdown":
		return presenter.RenderMarkdown(w)
	case "csv":
		return presenter.RenderCSV(w, csvPrefix)
	default:
		return errs.InvalidArgument(fmt.Sprintf("不支持的输出格式: %s", format))
	}
}

// renderJSON 统一负责 JSON 序列化和写出。
func renderJSON(w io.Writer, result model.CommandResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errs.ExecutionFailed("JSON 序列化失败", err)
	}

	if _, err := fmt.Fprintln(w, string(data)); err != nil {
		return errs.ExecutionFailed("输出 JSON 失败", err)
	}

	return nil
}

// NewErrorResult 构造错误场景下的统一结果对象。
func NewErrorResult(err error, startedAt time.Time) model.CommandResult {
	return model.CommandResult{
		Code: errs.Code(err),
		Msg:  errs.Message(err),
		Data: nil,
		Error: &model.ErrorInfo{
			ErrorCode:    errs.ErrorCode(err),
			ErrorMessage: errs.Message(err),
			Suggestion:   errs.Suggestion(err),
		},
		Metadata: model.Metadata{
			SchemaVersion: schemaVersion,
			Timestamp:     time.Now().UTC(),
			Host:          hostname(),
			Command:       strings.Join(os.Args, " "),
			ExecutionMs:   time.Since(startedAt).Milliseconds(),
			Platform:      runtime.GOOS,
			TraceID:       newTraceID(),
		},
	}
}

// RenderError 渲染错误结果。
// JSON 仍走统一 CommandResult；非 JSON 则走更适合人读的简化文本。
func RenderError(w io.Writer, format string, err error, startedAt time.Time) error {
	switch format {
	case "json":
		return renderJSON(w, NewErrorResult(err, startedAt))
	case "markdown":
		_, writeErr := fmt.Fprintf(w, "# 错误\n\n- 退出码: %d\n- 错误码: %s\n- 信息: %s\n", errs.Code(err), errs.ErrorCode(err), errs.Message(err))
		if writeErr != nil {
			return errs.ExecutionFailed("输出错误信息失败", writeErr)
		}
		if degradeHint := degradeHintByErrorCode(errs.ErrorCode(err)); degradeHint != "" {
			if _, writeErr = fmt.Fprintf(w, "- 降级说明: %s\n", degradeHint); writeErr != nil {
				return errs.ExecutionFailed("输出降级说明失败", writeErr)
			}
		}
		if suggestion := errs.Suggestion(err); suggestion != "" {
			if _, writeErr = fmt.Fprintf(w, "- 建议: %s\n", suggestion); writeErr != nil {
				return errs.ExecutionFailed("输出错误建议失败", writeErr)
			}
		}
		return nil
	default:
		if _, writeErr := fmt.Fprintf(w, "错误: %s\n", errs.Message(err)); writeErr != nil {
			return errs.ExecutionFailed("输出错误信息失败", writeErr)
		}
		if _, writeErr := fmt.Fprintf(w, "错误码: %s\n", errs.ErrorCode(err)); writeErr != nil {
			return errs.ExecutionFailed("输出错误码失败", writeErr)
		}
		if degradeHint := degradeHintByErrorCode(errs.ErrorCode(err)); degradeHint != "" {
			if _, writeErr := fmt.Fprintf(w, "降级说明: %s\n", degradeHint); writeErr != nil {
				return errs.ExecutionFailed("输出降级说明失败", writeErr)
			}
		}
		if suggestion := errs.Suggestion(err); suggestion != "" {
			if _, writeErr := fmt.Fprintf(w, "建议: %s\n", suggestion); writeErr != nil {
				return errs.ExecutionFailed("输出错误建议失败", writeErr)
			}
		}
		return nil
	}
}

func degradeHintByErrorCode(code string) string {
	switch code {
	case errs.CodePermissionDenied:
		return "当前权限不足，已按可访问范围降级；可用管理员/root 重试以获取完整结果"
	case errs.CodeTimeout:
		return "命令执行超时，已终止当前阶段；可调大 --timeout 或缩小范围重试"
	case errs.CodePlatformUnsupported:
		return "当前平台不支持该能力，已跳过该能力执行"
	default:
		return ""
	}
}

// hostname 获取当前主机名；失败时返回空字符串而不是中断命令。
func hostname() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}

	return host
}

// newTraceID 生成一条执行链路的短 trace ID。
// 如果随机数源不可用，则退化为基于时间戳的兜底值。
func newTraceID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	return hex.EncodeToString(buf)
}

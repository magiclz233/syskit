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

type Presenter interface {
	RenderTable(w io.Writer) error
	RenderMarkdown(w io.Writer) error
	RenderCSV(w io.Writer, prefix string) error
}

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

func RenderError(w io.Writer, format string, err error, startedAt time.Time) error {
	switch format {
	case "json":
		return renderJSON(w, NewErrorResult(err, startedAt))
	case "markdown":
		_, writeErr := fmt.Fprintf(w, "# 错误\n\n- 退出码: %d\n- 错误码: %s\n- 信息: %s\n", errs.Code(err), errs.ErrorCode(err), errs.Message(err))
		if writeErr != nil {
			return errs.ExecutionFailed("输出错误信息失败", writeErr)
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
		if suggestion := errs.Suggestion(err); suggestion != "" {
			if _, writeErr := fmt.Fprintf(w, "建议: %s\n", suggestion); writeErr != nil {
				return errs.ExecutionFailed("输出错误建议失败", writeErr)
			}
		}
		return nil
	}
}

func hostname() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}

	return host
}

func newTraceID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	return hex.EncodeToString(buf)
}

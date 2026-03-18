package output

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"syskit/internal/domain/model"
	"syskit/internal/errs"
	"testing"
	"time"
)

type mockPresenter struct {
	tableCalled    int
	markdownCalled int
	csvCalled      int
}

func (p *mockPresenter) RenderTable(w io.Writer) error {
	p.tableCalled++
	_, _ = io.WriteString(w, "table")
	return nil
}

func (p *mockPresenter) RenderMarkdown(w io.Writer) error {
	p.markdownCalled++
	_, _ = io.WriteString(w, "markdown")
	return nil
}

func (p *mockPresenter) RenderCSV(w io.Writer, prefix string) error {
	p.csvCalled++
	_, _ = io.WriteString(w, "csv:"+prefix)
	return nil
}

func TestRenderRoutesByFormat(t *testing.T) {
	result := model.CommandResult{Code: 0, Msg: "ok", Metadata: model.Metadata{SchemaVersion: "1.0"}}

	t.Run("table", func(t *testing.T) {
		p := &mockPresenter{}
		var buf bytes.Buffer
		if err := Render(&buf, "table", result, p, ""); err != nil {
			t.Fatalf("Render(table) error = %v", err)
		}
		if p.tableCalled != 1 || p.markdownCalled != 0 || p.csvCalled != 0 {
			t.Fatalf("unexpected calls: table=%d md=%d csv=%d", p.tableCalled, p.markdownCalled, p.csvCalled)
		}
	})

	t.Run("markdown", func(t *testing.T) {
		p := &mockPresenter{}
		var buf bytes.Buffer
		if err := Render(&buf, "markdown", result, p, ""); err != nil {
			t.Fatalf("Render(markdown) error = %v", err)
		}
		if p.tableCalled != 0 || p.markdownCalled != 1 || p.csvCalled != 0 {
			t.Fatalf("unexpected calls: table=%d md=%d csv=%d", p.tableCalled, p.markdownCalled, p.csvCalled)
		}
	})

	t.Run("csv", func(t *testing.T) {
		p := &mockPresenter{}
		var buf bytes.Buffer
		if err := Render(&buf, "csv", result, p, "prefix"); err != nil {
			t.Fatalf("Render(csv) error = %v", err)
		}
		if p.tableCalled != 0 || p.markdownCalled != 0 || p.csvCalled != 1 {
			t.Fatalf("unexpected calls: table=%d md=%d csv=%d", p.tableCalled, p.markdownCalled, p.csvCalled)
		}
		if !strings.Contains(buf.String(), "csv:prefix") {
			t.Fatalf("csv output = %q, want contains prefix", buf.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		p := &mockPresenter{}
		var buf bytes.Buffer
		if err := Render(&buf, "json", result, p, ""); err != nil {
			t.Fatalf("Render(json) error = %v", err)
		}
		if p.tableCalled != 0 || p.markdownCalled != 0 || p.csvCalled != 0 {
			t.Fatalf("json should not call presenter: table=%d md=%d csv=%d", p.tableCalled, p.markdownCalled, p.csvCalled)
		}
		if !strings.Contains(buf.String(), `"code": 0`) {
			t.Fatalf("json output = %q, want code field", buf.String())
		}
	})
}

func TestRenderInvalidFormat(t *testing.T) {
	result := model.CommandResult{Code: 0, Msg: "ok", Metadata: model.Metadata{SchemaVersion: "1.0"}}
	p := &mockPresenter{}
	var buf bytes.Buffer

	err := Render(&buf, "bad", result, p, "")
	if err == nil {
		t.Fatal("Render(bad) error = nil, want invalid argument")
	}
	if got := errs.ErrorCode(err); got != errs.CodeInvalidArgument {
		t.Fatalf("errs.ErrorCode(err) = %s, want %s", got, errs.CodeInvalidArgument)
	}
}

func TestRenderErrorPlainIncludesErrorCodeAndDegradeHint(t *testing.T) {
	var buf bytes.Buffer
	err := errs.PermissionDenied("权限不足", "")
	if renderErr := RenderError(&buf, "table", err, time.Now()); renderErr != nil {
		t.Fatalf("RenderError() error = %v", renderErr)
	}
	text := buf.String()
	if !strings.Contains(text, "错误码: "+errs.CodePermissionDenied) {
		t.Fatalf("plain output missing error code: %s", text)
	}
	if !strings.Contains(text, "降级说明:") {
		t.Fatalf("plain output missing degrade hint: %s", text)
	}
}

func TestRenderErrorMarkdownIncludesDegradeHint(t *testing.T) {
	var buf bytes.Buffer
	err := errs.New(errs.ExitExecutionFailed, errs.CodePlatformUnsupported, "平台不支持")
	if renderErr := RenderError(&buf, "markdown", err, time.Now()); renderErr != nil {
		t.Fatalf("RenderError(markdown) error = %v", renderErr)
	}
	text := buf.String()
	if !strings.Contains(text, "- 错误码: "+errs.CodePlatformUnsupported) {
		t.Fatalf("markdown output missing error code: %s", text)
	}
	if !strings.Contains(text, "- 降级说明:") {
		t.Fatalf("markdown output missing degrade hint: %s", text)
	}
}

func TestRenderErrorJSONProtocol(t *testing.T) {
	var buf bytes.Buffer
	err := errs.New(errs.ExitInvalidArgument, errs.CodeInvalidArgument, "参数错误")
	if renderErr := RenderError(&buf, "json", err, time.Now()); renderErr != nil {
		t.Fatalf("RenderError(json) error = %v", renderErr)
	}
	text := buf.String()
	if !strings.Contains(text, `"error_code": "ERR_INVALID_ARGUMENT"`) {
		t.Fatalf("json error output missing error_code: %s", text)
	}
	if !strings.Contains(text, `"suggestion":`) {
		t.Fatalf("json error output missing suggestion: %s", text)
	}
}

func TestRenderJSONMarshalFailure(t *testing.T) {
	result := model.CommandResult{
		Code:     0,
		Msg:      "ok",
		Data:     map[string]any{"bad": func() {}},
		Metadata: model.Metadata{SchemaVersion: "1.0"},
	}
	p := &mockPresenter{}
	var buf bytes.Buffer

	err := Render(&buf, "json", result, p, "")
	if err == nil {
		t.Fatal("Render(json with func) error = nil, want execution failed")
	}
	if !errors.Is(err, err) {
		// no-op: 只为了避免静态分析认为未使用 errors 包。
	}
	if got := errs.ErrorCode(err); got != errs.CodeExecutionFailed {
		t.Fatalf("errs.ErrorCode(err) = %s, want %s", got, errs.CodeExecutionFailed)
	}
}

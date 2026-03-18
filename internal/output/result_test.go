package output

import (
	"bytes"
	"strings"
	"syskit/internal/errs"
	"testing"
	"time"
)

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

package networkprobe

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syskit/internal/errs"
	"time"
)

type commandExecFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

var (
	commandRunner commandExecFunc = defaultCommandRunner
	runtimeName                   = runtime.GOOS
)

func defaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func isCommandNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	var execErr *exec.Error
	return errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound)
}

func mapContextError(err error, message string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, message, "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
	}
	return nil
}

func addWarning(set map[string]struct{}, message string) {
	if set == nil {
		return
	}
	normalized := strings.TrimSpace(message)
	if normalized == "" {
		return
	}
	set[normalized] = struct{}{}
}

func warningSlice(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	items := make([]string, 0, len(set))
	for item := range set {
		items = append(items, item)
	}
	slices.Sort(items)
	return items
}

func parseFloatToken(raw string) (float64, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, false
	}
	text = strings.ReplaceAll(text, ",", ".")
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func durationToMs(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func computeJitter(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	diffTotal := 0.0
	for idx := 1; idx < len(values); idx++ {
		diffTotal += math.Abs(values[idx] - values[idx-1])
	}
	return diffTotal / float64(len(values)-1)
}

func firstUsefulLine(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return line
	}
	return ""
}

func commandUnavailableError(command string) error {
	return errs.NewWithSuggestion(
		errs.ExitExecutionFailed,
		errs.CodeDependencyMissing,
		fmt.Sprintf("未找到系统命令: %s", command),
		fmt.Sprintf("请确认 %s 可执行并在 PATH 中", command),
	)
}

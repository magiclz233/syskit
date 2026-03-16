// Package cpu 提供 CPU 总览采集能力，供 `cpu` 命令与后续 `doctor cpu` 复用。
package cpu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	proccollector "syskit/internal/collectors/proc"
	"syskit/internal/errs"

	gocpu "github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
)

const defaultTopN = 5

// CollectOptions 定义 CPU 采集参数。
type CollectOptions struct {
	Detail bool
	TopN   int
}

// TopProcess 表示高 CPU 进程摘要。
type TopProcess struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	User       string  `json:"user,omitempty"`
	CPUPercent float64 `json:"cpu_percent"`
	Command    string  `json:"command,omitempty"`
}

// Overview 是 `cpu` 命令的结构化输出。
type Overview struct {
	CPUCores     int          `json:"cpu_cores"`
	UsagePercent float64      `json:"usage_percent"`
	Load1        float64      `json:"load1,omitempty"`
	Load5        float64      `json:"load5,omitempty"`
	Load15       float64      `json:"load15,omitempty"`
	PerCPU       []float64    `json:"per_cpu,omitempty"`
	TopProcesses []TopProcess `json:"top_processes"`
	Warnings     []string     `json:"warnings,omitempty"`
}

// CollectOverview 采集 CPU 总览与高 CPU 进程列表。
func CollectOverview(ctx context.Context, opts CollectOptions) (*Overview, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.TopN <= 0 {
		opts.TopN = defaultTopN
	}

	cores, err := gocpu.CountsWithContext(ctx, true)
	if err != nil {
		return nil, mapCollectionError("读取 CPU 核心数失败", err)
	}

	usageList, err := gocpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return nil, mapCollectionError("读取 CPU 使用率失败", err)
	}

	overview := &Overview{
		CPUCores:     cores,
		UsagePercent: firstPercent(usageList),
		TopProcesses: make([]TopProcess, 0, opts.TopN),
	}

	if avg, avgErr := load.AvgWithContext(ctx); avgErr == nil {
		overview.Load1 = avg.Load1
		overview.Load5 = avg.Load5
		overview.Load15 = avg.Load15
	} else {
		overview.Warnings = appendUnique(overview.Warnings, "当前平台不支持 load1/load5/load15 采集，已跳过")
	}

	if opts.Detail {
		perCPU, perErr := gocpu.PercentWithContext(ctx, 0, true)
		if perErr != nil {
			overview.Warnings = appendUnique(overview.Warnings, "读取每核心 CPU 使用率失败，已跳过")
		} else {
			overview.PerCPU = perCPU
		}
	}

	top, topErr := proccollector.CollectTop(ctx, proccollector.TopOptions{
		By:   proccollector.SortByCPU,
		TopN: opts.TopN,
	})
	if topErr != nil {
		overview.Warnings = appendUnique(overview.Warnings, fmt.Sprintf("读取高 CPU 进程失败: %s", errs.Message(topErr)))
		return overview, nil
	}

	overview.TopProcesses = make([]TopProcess, 0, len(top.Processes))
	for _, item := range top.Processes {
		overview.TopProcesses = append(overview.TopProcesses, TopProcess{
			PID:        item.PID,
			Name:       item.Name,
			User:       item.User,
			CPUPercent: item.CPUPercent,
			Command:    item.Command,
		})
	}
	overview.Warnings = appendUnique(overview.Warnings, top.Warnings...)
	return overview, nil
}

func firstPercent(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func mapCollectionError(message string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.NewWithSuggestion(errs.ExitExecutionFailed, errs.CodeTimeout, "命令执行超时", "请调大 --timeout 后重试")
	}
	if errors.Is(err, context.Canceled) {
		return errs.New(errs.ExitExecutionFailed, errs.CodeTimeout, "命令已取消")
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "permission denied") || strings.Contains(text, "access denied") || strings.Contains(text, "operation not permitted") {
		return errs.PermissionDenied(message, "请提升权限后重试")
	}
	return errs.ExecutionFailed(message, err)
}

func appendUnique(items []string, values ...string) []string {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		exists := false
		for _, item := range items {
			if item == value {
				exists = true
				break
			}
		}
		if !exists {
			items = append(items, value)
		}
	}
	return items
}

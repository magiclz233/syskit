package proc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"syskit/internal/errs"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

const (
	killStatusKilled        = "killed"
	killStatusFailed        = "failed"
	killStatusAlreadyExited = "already_exited"
)

// BuildKillPlan 按 discover/plan 阶段生成终止计划。
func BuildKillPlan(ctx context.Context, pid int32, tree bool, force bool) (*KillPlan, error) {
	processes, childrenByPID, warnings, err := BuildProcessMap(ctx)
	if err != nil {
		return nil, err
	}

	root, ok := processes[pid]
	if !ok {
		return nil, errs.New(errs.ExitExecutionFailed, errs.CodeNotFound, fmt.Sprintf("未找到 PID=%d 的进程", pid))
	}

	plan := &KillPlan{
		RootPID:  pid,
		RootName: root.Name,
		Force:    force,
		Tree:     tree,
		Warnings: warnings,
		Steps: []string{
			"discover: 定位目标进程",
			"plan: 生成 dry-run 计划",
			"confirm: 检查 --apply 与 --yes",
			"apply: 执行终止动作",
			"verify: 校验进程是否退出",
			"audit: 审计日志（P0-022 接入后落盘）",
		},
	}

	targets := []KillTarget{
		{
			PID:   root.PID,
			Name:  root.Name,
			Depth: 0,
		},
	}

	if tree {
		visited := map[int32]bool{pid: true}
		descendants := collectDescendants(pid, 1, childrenByPID, processes, visited)
		targets = append(descendants, targets...)
	}

	sortTargetsForKill(targets)
	plan.Targets = targets
	return plan, nil
}

// ExecuteKillPlan 按计划执行终止并返回 verify 结果。
func ExecuteKillPlan(ctx context.Context, plan *KillPlan) (*KillResult, error) {
	if plan == nil {
		return nil, errs.InvalidArgument("kill 计划不能为空")
	}
	if len(plan.Targets) == 0 {
		return nil, errs.InvalidArgument("kill 计划没有可执行目标")
	}

	result := &KillResult{
		Plan:    plan,
		Applied: true,
		Results: make([]KillTargetResult, 0, len(plan.Targets)),
	}

	for _, target := range plan.Targets {
		if err := ctx.Err(); err != nil {
			return nil, timeoutError(err)
		}

		processRef, err := process.NewProcessWithContext(ctx, target.PID)
		if err != nil {
			if isMissingProcess(err) {
				result.Results = append(result.Results, KillTargetResult{
					PID:    target.PID,
					Name:   target.Name,
					Status: killStatusAlreadyExited,
				})
				continue
			}
			if permission := permissionError(fmt.Sprintf("终止 PID=%d 失败", target.PID), err); permission != nil {
				return nil, permission
			}
			result.Results = append(result.Results, KillTargetResult{
				PID:     target.PID,
				Name:    target.Name,
				Status:  killStatusFailed,
				Message: err.Error(),
			})
			result.FailedPIDs = append(result.FailedPIDs, target.PID)
			continue
		}

		var killErr error
		if plan.Force {
			killErr = processRef.KillWithContext(ctx)
		} else {
			killErr = processRef.TerminateWithContext(ctx)
		}

		if killErr != nil {
			if isMissingProcess(killErr) {
				result.Results = append(result.Results, KillTargetResult{
					PID:    target.PID,
					Name:   target.Name,
					Status: killStatusAlreadyExited,
				})
				continue
			}
			if permission := permissionError(fmt.Sprintf("终止 PID=%d 失败", target.PID), killErr); permission != nil {
				return nil, permission
			}
			result.Results = append(result.Results, KillTargetResult{
				PID:     target.PID,
				Name:    target.Name,
				Status:  killStatusFailed,
				Message: killErr.Error(),
			})
			result.FailedPIDs = append(result.FailedPIDs, target.PID)
			continue
		}

		exited, verifyErr := waitUntilExit(ctx, target.PID, 3*time.Second)
		if verifyErr != nil {
			if timeout := timeoutError(verifyErr); timeout != nil {
				return nil, timeout
			}
			result.Results = append(result.Results, KillTargetResult{
				PID:     target.PID,
				Name:    target.Name,
				Status:  killStatusFailed,
				Message: "终止后校验失败: " + verifyErr.Error(),
			})
			result.FailedPIDs = append(result.FailedPIDs, target.PID)
			continue
		}
		if !exited {
			result.Results = append(result.Results, KillTargetResult{
				PID:     target.PID,
				Name:    target.Name,
				Status:  killStatusFailed,
				Message: "进程仍在运行",
			})
			result.FailedPIDs = append(result.FailedPIDs, target.PID)
			continue
		}

		result.Results = append(result.Results, KillTargetResult{
			PID:    target.PID,
			Name:   target.Name,
			Status: killStatusKilled,
		})
	}

	result.Verified = len(result.FailedPIDs) == 0
	if !result.Verified {
		result.Warnings = append(result.Warnings, fmt.Sprintf("共有 %d 个进程终止失败", len(result.FailedPIDs)))
	}
	return result, nil
}

func collectDescendants(
	pid int32,
	depth int,
	childrenByPID map[int32][]int32,
	processes map[int32]ProcessSnapshot,
	visited map[int32]bool,
) []KillTarget {
	children := childrenByPID[pid]
	if len(children) == 0 {
		return nil
	}

	result := make([]KillTarget, 0, len(children))
	for _, childPID := range children {
		if visited[childPID] {
			continue
		}
		visited[childPID] = true
		child, ok := processes[childPID]
		if !ok {
			continue
		}

		result = append(result, KillTarget{
			PID:   childPID,
			Name:  child.Name,
			Depth: depth,
		})

		next := collectDescendants(childPID, depth+1, childrenByPID, processes, visited)
		result = append(result, next...)
	}

	return result
}

func sortTargetsForKill(targets []KillTarget) {
	sort.Slice(targets, func(i int, j int) bool {
		if targets[i].Depth != targets[j].Depth {
			return targets[i].Depth > targets[j].Depth
		}
		return targets[i].PID < targets[j].PID
	})
}

func waitUntilExit(ctx context.Context, pid int32, timeout time.Duration) (bool, error) {
	verifyCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		verifyCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for {
		exists, err := process.PidExistsWithContext(verifyCtx, pid)
		if err != nil {
			if isMissingProcess(err) {
				return true, nil
			}
			return false, err
		}
		if !exists {
			return true, nil
		}

		select {
		case <-verifyCtx.Done():
			if errors.Is(verifyCtx.Err(), context.DeadlineExceeded) {
				return false, nil
			}
			return false, verifyCtx.Err()
		case <-ticker.C:
		}
	}
}

func isMissingProcess(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "process does not exist") ||
		strings.Contains(message, "process not found") ||
		strings.Contains(message, "no such process") ||
		strings.Contains(message, "not found")
}

// CountKillFailures 统计执行结果中的失败项数量，供命令层决定退出策略。
func CountKillFailures(result *KillResult) int {
	if result == nil {
		return 0
	}
	return len(result.FailedPIDs)
}

// FailedTargetPIDs 返回失败目标 PID 列表的副本，避免命令层误改内部结构。
func FailedTargetPIDs(result *KillResult) []int32 {
	if result == nil || len(result.FailedPIDs) == 0 {
		return nil
	}
	dup := make([]int32, len(result.FailedPIDs))
	copy(dup, result.FailedPIDs)
	slices.Sort(dup)
	return dup
}

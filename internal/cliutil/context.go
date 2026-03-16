// Package cliutil 放置多个命令都会复用的 CLI 小工具。
package cliutil

import (
	"context"
	"fmt"
	"strings"
	"syskit/internal/errs"
	"time"

	"github.com/spf13/cobra"
)

// CommandContext 根据全局 `--timeout` 计算本次命令执行上下文。
// 当 timeout 未设置或 <=0 时返回可取消上下文，避免调用方分支处理。
func CommandContext(cmd *cobra.Command) (context.Context, context.CancelFunc, error) {
	rawTimeout := strings.TrimSpace(ResolveStringFlag(cmd, "timeout"))
	if rawTimeout == "" || rawTimeout == "0" || rawTimeout == "0s" {
		ctx, cancel := context.WithCancel(context.Background())
		return ctx, cancel, nil
	}

	timeout, err := time.ParseDuration(rawTimeout)
	if err != nil {
		return nil, nil, errs.InvalidArgument(fmt.Sprintf("无效的 --timeout: %s", rawTimeout))
	}
	if timeout <= 0 {
		ctx, cancel := context.WithCancel(context.Background())
		return ctx, cancel, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return ctx, cancel, nil
}

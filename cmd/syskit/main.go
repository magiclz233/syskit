// cmd/syskit/main.go 是 syskit 二进制的进程入口。
// 它只负责调用 CLI 并按统一错误协议返回退出码，不承载业务逻辑。
package main

import (
	"os"
	"syskit/internal/cli"
	"syskit/internal/errs"
	"syskit/internal/version"
)

func main() {
	if err := cli.Execute(version.Value); err != nil {
		os.Exit(errs.Code(err))
	}
}

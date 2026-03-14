package main

import (
	"fmt"
	"os"
	"syskit/internal/cli"
	"syskit/internal/errs"
	"syskit/internal/version"
)

func main() {
	if err := cli.Execute(version.Value); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(errs.Code(err))
	}
}

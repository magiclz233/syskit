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

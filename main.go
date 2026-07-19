package main

import (
	"os"
	"codeberg.org/kehvyn/pglantern-cli/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

package main

import (
	"os"

	"codeberg.org/kehvyn/horton-cli/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

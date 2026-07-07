package main

import (
	"os"

	"github.com/kjrocker/horton/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

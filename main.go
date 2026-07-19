package main

import (
	"os"

	"codeberg.org/kehvyn/pglantern-cli/internal/cli"
	"codeberg.org/kehvyn/pglantern-cli/internal/output"
)

func main() {
	err := cli.NewRootCmd().Execute()
	// Cobra skips PersistentPostRun when RunE errors, so close the pager here
	// too before exiting; it holds the terminal until the user quits.
	output.StopPager()
	if err != nil {
		os.Exit(cli.ExitCode(err))
	}
}

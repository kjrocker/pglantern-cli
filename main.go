package main

import (
	"fmt"
	"os"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/cli"
	"git.kehvyn.dev/kevin/pglantern-cli/internal/output"
)

func main() {
	err := cli.NewRootCmd().Execute()
	// Cobra skips PersistentPostRun when RunE errors, so close the pager here
	// too before exiting; it holds the terminal until the user quits.
	output.StopPager()
	if err != nil {
		// Cobra has already printed the server's message; add the follow-up
		// action when there is one (e.g. a read-only key refused a write).
		if hint := cli.Hint(err); hint != "" {
			fmt.Fprintln(os.Stderr, hint)
		}
		os.Exit(cli.ExitCode(err))
	}
}

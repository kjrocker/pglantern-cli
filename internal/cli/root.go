// Package cli defines the lantern command tree.
//
// lantern is a deliberately thin client for the pgLantern JSON API: it validates
// required arguments and flag types, passes everything else through verbatim,
// and surfaces the server's error envelope as-is.
package cli

import (
	"errors"

	"codeberg.org/kehvyn/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

var version = "dev"

// NewRootCmd builds the root command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "lantern",
		Short:         "Command-line client for the pgLantern mailing-list archive API",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	flags := root.PersistentFlags()
	flags.Bool("json", false, "print the raw JSON response instead of a table")
	flags.Bool("no-pager", false, "do not pipe output through a pager")
	flags.String("host", "", "API host (default $LANTERN_HOST, config file, or https://pglantern.com)")
	flags.String("api-key", "", "API key (default $LANTERN_API_KEY or config file)")

	// Flag-parse failures (unknown flags, enum rejections, bad ints) are the
	// caller's usage errors — mark them so main exits 2.
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &usageError{err: err}
	})

	// Interactive runs page through less-alike; commands that prompt on the
	// terminal or write only to stderr keep it directly.
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if noPager, _ := cmd.Flags().GetBool("no-pager"); noPager {
			return
		}
		switch cmd.Name() {
		case "login", "logout", "generate-skill", "open":
			return
		}
		output.StartPager()
	}
	root.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		output.StopPager()
	}

	root.AddCommand(
		newLoginCmd(),
		newLogoutCmd(),
		newListsCmd(),
		newMessagesCmd(),
		newSearchCmd(),
		newCommitsCmd(),
		newSendersCmd(),
		newThreadsCmd(),
		newAttachmentsCmd(),
		newVersionsCmd(),
		newActivityCmd(),
		newImportsCmd(),
		newAnalyticsCmd(),
		newAPICmd(),
		newGenerateSkillCmd(),
	)

	wrapArgsErrors(root)
	return root
}

// wrapArgsErrors marks every command's positional-args validation failures as
// usage errors, covering cobra.NoArgs and friends alongside our own
// requireArg-style validators (already wrapped; not double-wrapped here).
func wrapArgsErrors(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			err := validate(c, args)
			var usage *usageError
			if err == nil || errors.As(err, &usage) {
				return err
			}
			return &usageError{err: err}
		}
	}
	for _, sub := range cmd.Commands() {
		wrapArgsErrors(sub)
	}
}

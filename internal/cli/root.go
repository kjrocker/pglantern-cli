// Package cli defines the horton command tree.
//
// horton is a deliberately thin client for the Horton JSON API: it validates
// required arguments and flag types, passes everything else through verbatim,
// and surfaces the server's error envelope as-is.
package cli

import "github.com/spf13/cobra"

var version = "0.1.0"

// NewRootCmd builds the root command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "horton",
		Short:         "Command-line client for the Horton mailing-list archive API",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	flags := root.PersistentFlags()
	flags.Bool("json", false, "print the raw JSON response instead of a table")
	flags.String("host", "", "API host (default $HORTON_HOST, config file, or http://localhost:4000)")
	flags.String("api-key", "", "API key (default $HORTON_API_KEY or config file)")

	root.AddCommand(
		newLoginCmd(),
		newLogoutCmd(),
		newListsCmd(),
	)

	return root
}

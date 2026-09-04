package cli

import (
	"os"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
	"git.kehvyn.dev/kevin/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

func newImportsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "imports",
		Short: "Show mbox files imported into the archive",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "list")
			return getRender(cmd, "/imports", q, func(page api.Page[api.ImportFile]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no results")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, f := range page.Data {
					rows = append(rows, []string{
						f.Basename, output.HumanBytes(f.Size), f.InsertedAt,
					})
				}
				output.Table(os.Stdout, []string{"FILE", "SIZE", "IMPORTED AT"}, rows)
			})
		},
	}
	cmd.Flags().String("list", "", "mailing list name (server default pgsql-hackers)")
	return cmd
}

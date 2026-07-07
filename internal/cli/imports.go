package cli

import (
	"os"
	"strconv"

	"codeberg.org/kehvyn/horton-cli/internal/api"
	"codeberg.org/kehvyn/horton-cli/internal/output"
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
				rows := make([][]string, 0, len(page.Data))
				for _, f := range page.Data {
					rows = append(rows, []string{
						f.Basename, strconv.FormatInt(f.Size, 10), f.InsertedAt,
					})
				}
				output.Table(os.Stdout, []string{"FILE", "BYTES", "IMPORTED AT"}, rows)
			})
		},
	}
	cmd.Flags().String("list", "", "mailing list name (server default pgsql-hackers)")
	return cmd
}

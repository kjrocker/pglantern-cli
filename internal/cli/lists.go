package cli

import (
	"os"

	"codeberg.org/kehvyn/horton-cli/internal/api"
	"codeberg.org/kehvyn/horton-cli/internal/output"
	"github.com/spf13/cobra"
)

func newListsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lists",
		Short: "List the mailing lists in the archive",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getRender(cmd, "/lists", nil, func(page api.Page[api.List]) {
				rows := make([][]string, 0, len(page.Data))
				for _, l := range page.Data {
					active := "yes"
					if !l.Active {
						active = "no"
					}
					rows = append(rows, []string{l.Name, active, l.ShortDesc})
				}
				output.Table(os.Stdout, []string{"NAME", "ACTIVE", "DESCRIPTION"}, rows)
			})
		},
	}
}

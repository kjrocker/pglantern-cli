package cli

import (
	"os"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
	"git.kehvyn.dev/kevin/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

func newListsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lists",
		Short: "List the mailing lists in the archive",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getRender(cmd, "/lists", nil, func(page api.Page[api.List]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no results")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, l := range page.Data {
					active := "yes"
					if !l.Active {
						active = "no"
					}
					rows = append(rows, []string{l.Name, active, lastActivity(l.Stats), l.ShortDesc})
				}
				output.Table(os.Stdout, []string{"NAME", "ACTIVE", "LAST ACTIVITY", "DESCRIPTION"}, rows)
			})
		},
	}
}

// lastActivity renders a list's latest post as a date, or "-" when the list has
// no messages or its stats haven't been computed yet.
func lastActivity(s *api.ListStats) string {
	if s == nil || s.LastMessageAt == nil || len(*s.LastMessageAt) < 10 {
		return output.OrDash(nil)
	}
	return (*s.LastMessageAt)[:10]
}

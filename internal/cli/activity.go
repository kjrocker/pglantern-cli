package cli

import (
	"os"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
	"git.kehvyn.dev/kevin/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

func newActivityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity <path>",
		Short: "Merged commit + discussion activity for a source-tree path",
		Args:  requireArg("a source-tree path"),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "major", "limit", "after", "before")
			q.Set("path", args[0])
			return getRenderPage(cmd, "/source/activity", q, func(page api.Page[api.ActivityRow]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no results")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, r := range page.Data {
					ref := r.SHA
					if r.Kind == "message" {
						ref = r.MessageID
					}
					rows = append(rows, []string{
						r.Kind, r.ActivityAt, output.Truncate(r.Subject, 64), ref,
					})
				}
				output.Table(os.Stdout, []string{"KIND", "AT", "SUBJECT", "REF"}, rows)
				output.CursorFooter(page.NextCursor)
			})
		},
	}
	cmd.Flags().String("major", "", "narrow commits to this major's containment (16, 9.6, master)")
	addPaginationFlags(cmd)
	return cmd
}

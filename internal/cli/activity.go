package cli

import (
	"os"

	"github.com/kjrocker/horton/internal/api"
	"github.com/kjrocker/horton/internal/output"
	"github.com/spf13/cobra"
)

func newActivityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity <path>",
		Short: "Merged commit + discussion activity for a source-tree path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "major", "limit", "after", "before")
			q.Set("path", args[0])
			return getRender(cmd, "/source/activity", q, func(page api.Page[api.ActivityRow]) {
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
	cmd.Flags().Int("limit", 0, "page size (server default 25, max 100)")
	cmd.Flags().String("after", "", "page cursor")
	cmd.Flags().String("before", "", "page cursor")
	return cmd
}

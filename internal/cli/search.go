package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>...",
		Short: "Full-text search over messages",
		Args:  requireArgs("a search query"),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd,
				"sort", "sender", "committed", "path", "major",
				"from", "to", "limit", "after", "before")
			q.Set("q", strings.Join(args, " "))
			return renderMessagePage(cmd, "/search", q, true)
		},
	}
	addEnumFlag(cmd, "sort", "sort order", "relevance", "sent_at")
	cmd.Flags().String("sender", "", "substring match on sender email or name")
	cmd.Flags().Bool("committed", false, "only threads with a landed commit")
	cmd.Flags().String("path", "", "threads whose landed commit or patch touches this path prefix")
	cmd.Flags().String("major", "", "threads that landed a commit first shipped in this major (16, 9.6, master)")
	cmd.Flags().String("from", "", "ISO-8601 lower bound on sent_at")
	cmd.Flags().String("to", "", "ISO-8601 upper bound on sent_at")
	cmd.Flags().Int("limit", 0, "page size (server default 25, max 100)")
	cmd.Flags().String("after", "", "page cursor")
	cmd.Flags().String("before", "", "page cursor")
	addFullFlag(cmd)
	return cmd
}

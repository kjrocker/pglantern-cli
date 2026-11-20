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
			q := collectQuery(cmd, append([]string{
				"sort", "dir", "sender", "committed", "path", "major", "cf-status",
				"from", "to", "limit", "after", "before",
			}, threadFilterScalars...)...)
			collectArrayQuery(cmd, q, threadFilterArrays...)
			q.Set("q", strings.Join(args, " "))
			return renderMessagePage(cmd, "/search", q, true)
		},
	}
	addEnumFlag(cmd, "sort", "sort order", "relevance", "sent_at")
	addEnumFlag(cmd, "dir", "sort direction, only with --sort sent_at", "asc", "desc")
	cmd.Flags().String("sender", "", "substring match on sender email or name")
	cmd.Flags().Bool("committed", false, "only threads with a landed commit")
	cmd.Flags().String("path", "", "threads whose landed commit or patch touches this path prefix")
	cmd.Flags().String("major", "", "threads that landed a commit first shipped in this major (16, 9.6, master)")
	addEnumFlag(cmd, "cf-status", "threads with a commitfest patch in this status", cfSearchStatuses...)
	cmd.Flags().String("from", "", "ISO-8601 lower bound on sent_at")
	cmd.Flags().String("to", "", "ISO-8601 upper bound on sent_at")
	addThreadFilterFlags(cmd,
		"messages posted to any of these lists",
		"drop messages posted to this list")
	addPaginationFlags(cmd)
	addFullFlag(cmd)
	return cmd
}

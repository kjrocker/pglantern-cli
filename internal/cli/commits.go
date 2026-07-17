package cli

import (
	"fmt"
	"net/url"
	"os"

	"codeberg.org/kehvyn/horton-cli/internal/api"
	"codeberg.org/kehvyn/horton-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCommitsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commits",
		Short: "Browse postgres commits linked to the archive",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "path", "author", "major", "from", "to", "limit", "after", "before")
			return getRender(cmd, "/commits", q, func(page api.Page[api.CommitSummary]) {
				output.Table(os.Stdout, []string{"SHA", "AUTHOR", "COMMITTED AT", "SUBJECT"},
					commitRows(page.Data))
				output.CursorFooter(page.NextCursor)
			})
		},
	}
	cmd.Flags().String("path", "", "commits touching files under this path prefix")
	cmd.Flags().String("author", "", "substring match on author name or email")
	cmd.Flags().String("major", "", "commits first shipped in this major (16, 9.6, master)")
	cmd.Flags().String("from", "", "ISO-8601 lower bound on committed_at")
	cmd.Flags().String("to", "", "ISO-8601 upper bound on committed_at")
	cmd.Flags().Int("limit", 0, "page size (server default 25, max 100)")
	cmd.Flags().String("after", "", "page cursor")
	cmd.Flags().String("before", "", "page cursor")

	cmd.AddCommand(newCommitsGetCmd(), newCommitsThreadCmd())
	return cmd
}

func newCommitsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <sha>",
		Short: "Show one commit (full 40-hex sha)",
		Args:  requireArg("a commit sha"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/commits/" + url.PathEscape(args[0])
			return getRender(cmd, path, nil, func(item api.Item[api.CommitFull]) {
				c := item.Data
				pairs := [][2]string{
					{"Commit", c.SHA},
					{"Author", fmt.Sprintf("%s <%s>", c.AuthorName, c.AuthorEmail)},
					{"Authored", c.AuthoredAt},
					{"Committer", fmt.Sprintf("%s <%s>", c.CommitterName, c.CommitterEmail)},
					{"Committed", c.CommittedAt},
				}
				for _, r := range c.Releases {
					release := r.Branch
					if r.FirstTag != "" {
						release = fmt.Sprintf("%s (%s)", r.Branch, r.FirstTag)
					}
					pairs = append(pairs, [2]string{"Release", release})
				}
				output.Detail(os.Stdout, pairs)
				// The body already begins with the subject line.
				if c.Body != "" {
					fmt.Printf("\n%s\n", c.Body)
				} else {
					fmt.Printf("\n%s\n", c.Subject)
				}
				if len(c.Files) > 0 {
					fmt.Println()
					rows := make([][]string, 0, len(c.Files))
					for _, f := range c.Files {
						rows = append(rows, []string{
							f.Path, f.ChangeType,
							fmt.Sprintf("+%d", f.Additions), fmt.Sprintf("-%d", f.Deletions),
						})
					}
					output.Table(os.Stdout, []string{"PATH", "CHANGE", "ADD", "DEL"}, rows)
				}
			})
		},
	}
}

func newCommitsThreadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "thread <sha>",
		Short: "Show the mailing-list discussion behind a commit",
		Args:  requireArg("a commit sha"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/commits/" + url.PathEscape(args[0]) + "/thread"
			return getRender(cmd, path, nil, func(item api.Item[api.CommitThread]) {
				t := item.Data
				fmt.Printf("# %s %s\n", t.Commit.SHA, t.Commit.Subject)
				for _, thread := range t.Threads {
					fmt.Printf("\n## thread %s\n", thread.ThreadID)
					messageTable(thread.Messages)
				}
				if len(t.Threads) == 0 {
					fmt.Fprintln(os.Stderr, "no archived discussion found")
				}
				for _, ref := range t.UnresolvedRefs {
					fmt.Fprintf(os.Stderr, "# unresolved ref (%s): %s\n", ref.Source, ref.RefMessageID)
				}
			})
		},
	}
}

package cli

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
	"git.kehvyn.dev/kevin/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

// cfStatuses are the per-entry status slugs plus the open/closed groupings the
// patches endpoint accepts.
var cfStatuses = []string{
	"needs_review", "waiting_on_author", "ready_for_committer", "committed",
	"rejected", "returned", "withdrawn", "moved", "open", "closed",
}

// cfSearchStatuses are the values --cf-status takes on search and threads.
var cfSearchStatuses = []string{
	"needs_review", "waiting_on_author", "ready_for_committer", "committed",
	"rejected", "returned", "withdrawn", "open", "closed", "any",
}

func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return output.OrDash(&s)
}

// cfEntryStatus renders an entry's status, or a dash when the patch has none.
func cfEntryStatus(e *api.CfEntry) string {
	if e == nil {
		return "-"
	}
	return e.Status
}

func cfEntryName(e *api.CfEntry) string {
	if e == nil {
		return "-"
	}
	return e.CfName
}

// cfCounts renders "open/total" for a commitfest: entries still being worked
// (not committed, rejected, returned, withdrawn or moved) over all entries.
func cfCounts(counts map[string]int) (string, string) {
	total, open := 0, 0
	for status, n := range counts {
		total += n
		switch status {
		case "needs_review", "waiting_on_author", "ready_for_committer":
			open += n
		}
	}
	return strconv.Itoa(open), strconv.Itoa(total)
}

func newCommitfestsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commitfests",
		Short: "Browse commitfests and their patches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "limit", "after", "before")
			return getRenderPage(cmd, "/commitfests", q, func(page api.Page[api.Commitfest]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no results")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, c := range page.Data {
					open, total := cfCounts(c.EntryCounts)
					rows = append(rows, []string{
						strconv.Itoa(c.ID), c.Name, c.Status,
						dateOnly(c.StartDate), dateOnly(c.EndDate), open, total,
					})
				}
				output.Table(os.Stdout,
					[]string{"ID", "NAME", "STATUS", "START", "END", "OPEN", "TOTAL"}, rows)
				output.CursorFooter(page.NextCursor)
			})
		},
	}
	addPaginationFlags(cmd)
	cmd.AddCommand(newCommitfestsPatchesCmd(), newCommitfestsPatchCmd())
	return cmd
}

func newCommitfestsPatchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patches <commitfest-id>",
		Short: "List one commitfest's patches",
		Args:  requireArg("a commitfest id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "status", "limit", "after", "before")
			path := "/commitfests/" + url.PathEscape(args[0]) + "/patches"
			return getRenderPage(cmd, path, q, func(page api.Page[api.CfPatchSummary]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no results")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, p := range page.Data {
					entry := p.Entry
					if entry == nil {
						entry = p.Current
					}
					rows = append(rows, []string{
						strconv.Itoa(p.ID), cfEntryStatus(entry),
						output.Truncate(p.Name, 64),
						output.Truncate(strings.Join(p.Authors, ", "), 40),
					})
				}
				output.Table(os.Stdout, []string{"PATCH", "STATUS", "NAME", "AUTHORS"}, rows)
				output.CursorFooter(page.NextCursor)
			})
		},
	}
	addEnumFlag(cmd, "status", "only entries with this status", cfStatuses...)
	addPaginationFlags(cmd)
	return cmd
}

func newCommitfestsPatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "patch <patch-id>",
		Short: "Show one patch: history, cited threads and landed commits",
		Args:  requireArg("a patch id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/cf-patches/" + url.PathEscape(args[0])
			return getRender(cmd, path, nil, func(item api.Item[api.CfPatch]) {
				renderCfPatch(item.Data)
			})
		},
	}
}

func renderCfPatch(p api.CfPatch) {
	fmt.Printf("# patch %d: %s\n", p.ID, p.Name)
	if p.Current != nil {
		fmt.Printf("status: %s (%s)\n", p.Current.Status, p.Current.CfName)
	}
	if len(p.Authors) > 0 {
		fmt.Printf("authors: %s\n", strings.Join(p.Authors, ", "))
	}
	if p.SourceURL != "" {
		fmt.Printf("url: %s\n", p.SourceURL)
	}

	fmt.Println()
	fmt.Println("history")
	if len(p.History) == 0 {
		output.EmptyNote("no commitfest entries")
	} else {
		rows := make([][]string, 0, len(p.History))
		for _, e := range p.History {
			rows = append(rows, []string{
				e.CfName, e.Status, dateOnly(e.EnteredAt), output.OrDash(leftDate(e.LeftAt)),
			})
		}
		output.Table(os.Stdout, []string{"CF", "STATUS", "ENTERED", "LEFT"}, rows)
	}

	fmt.Println()
	fmt.Println("threads")
	if len(p.Threads) == 0 {
		output.EmptyNote("no cited threads")
	} else {
		rows := make([][]string, 0, len(p.Threads))
		for _, t := range p.Threads {
			rows = append(rows, []string{
				output.OrDash(t.ThreadID),
				output.Truncate(output.OrDash(t.Subject), 64),
				output.OrDash(leftDate(t.LatestMessageAt)),
				t.MessageID,
			})
		}
		output.Table(os.Stdout, []string{"THREAD", "SUBJECT", "LATEST", "MESSAGE-ID"}, rows)
	}

	fmt.Println()
	fmt.Println("commits")
	if len(p.Commits) == 0 {
		output.EmptyNote("no commits landed from these threads")
	} else {
		output.Table(os.Stdout, commitHeader, commitRows(p.Commits))
	}
}

// leftDate trims an optional timestamp to its date, nil-preserving.
func leftDate(s *string) *string {
	if s == nil {
		return nil
	}
	d := dateOnly(*s)
	return &d
}

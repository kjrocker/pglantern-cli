package cli

import (
	"os"
	"strconv"

	"codeberg.org/kehvyn/horton-cli/internal/api"
	"codeberg.org/kehvyn/horton-cli/internal/output"
	"github.com/spf13/cobra"
)

// threadStarterCell renders the "who kicked it off" column: display name if we
// have one, else the raw email, else a dash for threads with no ingested start.
func threadStarterCell(s *api.ThreadStarter) string {
	if s == nil || s.Sender == nil {
		return "-"
	}
	if s.Sender.DisplayName != "" {
		return s.Sender.DisplayName
	}
	return output.OrDash(&s.Sender.Email)
}

func threadMessageIDCell(s *api.ThreadStarter) string {
	if s == nil {
		return "-"
	}
	return output.OrDash(&s.MessageID)
}

func newThreadsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "threads",
		Short: "Browse discussion threads across the lists",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "q", "from", "to", "limit", "after", "before")
			return getRender(cmd, "/threads", q, func(page api.Page[api.ThreadSummary]) {
				rows := make([][]string, 0, len(page.Data))
				for _, t := range page.Data {
					rows = append(rows, []string{
						output.Truncate(t.Subject, 48),
						strconv.Itoa(t.MessageCount),
						t.LastActivityAt,
						output.Truncate(threadStarterCell(t.Starter), 32),
						threadMessageIDCell(t.Starter),
					})
				}
				output.Table(os.Stdout,
					[]string{"SUBJECT", "MESSAGES", "LAST ACTIVITY", "STARTER", "MESSAGE-ID"}, rows)
				output.CursorFooter(page.NextCursor)
			})
		},
	}
	cmd.Flags().String("q", "", "substring on subject or full-text over member messages")
	cmd.Flags().String("from", "", "only threads active on/after this date (ISO-8601)")
	cmd.Flags().String("to", "", "only threads active on/before this date (ISO-8601)")
	cmd.Flags().Int("limit", 0, "page size (server default 25, max 100)")
	cmd.Flags().String("after", "", "page cursor")
	cmd.Flags().String("before", "", "page cursor")
	return cmd
}

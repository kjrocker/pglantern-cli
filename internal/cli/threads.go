package cli

import (
	"fmt"
	"os"
	"strconv"

	"codeberg.org/kehvyn/horton-cli/internal/api"
	"codeberg.org/kehvyn/horton-cli/internal/output"
	"github.com/spf13/cobra"
)

// threadStarterCell renders the "who kicked it off" column as "Name <email>",
// matching the From column elsewhere: both parts if we have them, else the bare
// email, else a dash for threads with no ingested start.
func threadStarterCell(s *api.ThreadStarter) string {
	if s == nil || s.Sender == nil {
		return "-"
	}
	if s.Sender.DisplayName != "" && s.Sender.Email != "" {
		return fmt.Sprintf("%s <%s>", s.Sender.DisplayName, s.Sender.Email)
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
						output.Truncate(threadStarterCell(t.Starter), 32),
						strconv.Itoa(t.MessageCount),
						t.LastActivityAt,
						threadMessageIDCell(t.Starter),
					})
				}
				output.Table(os.Stdout,
					[]string{"SUBJECT", "STARTER", "MESSAGES", "LAST ACTIVITY", "MESSAGE-ID"}, rows)
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

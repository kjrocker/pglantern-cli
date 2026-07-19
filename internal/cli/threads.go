package cli

import (
	"fmt"
	"os"
	"strconv"

	"codeberg.org/kehvyn/pglantern-cli/internal/api"
	"codeberg.org/kehvyn/pglantern-cli/internal/output"
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
		Use:   "threads [query...]",
		Short: "Browse discussion threads across the lists",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "q", "sort", "dir", "from", "to", "limit", "after", "before")
			if err := positionalQuery(cmd, args, q); err != nil {
				return err
			}
			return getRenderPage(cmd, "/threads", q, func(page api.Page[api.ThreadSummary]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no results")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, t := range page.Data {
					rows = append(rows, []string{
						output.Truncate(t.Subject, 64),
						output.Truncate(threadStarterCell(t.Starter), 40),
						strconv.Itoa(t.MessageCount),
						output.OrDash(&t.StartedAt),
						t.LastActivityAt,
						threadMessageIDCell(t.Starter),
					})
				}
				output.Table(os.Stdout,
					[]string{"SUBJECT", "STARTER", "MESSAGES", "FIRST", "LAST ACTIVITY", "MESSAGE-ID"}, rows)
				output.CursorFooter(page.NextCursor)
			})
		},
	}
	cmd.Flags().String("q", "", "substring on subject or full-text over member messages")
	addEnumFlag(cmd, "sort", "sort key", "messages", "first", "last")
	addEnumFlag(cmd, "dir", "sort direction", "asc", "desc")
	cmd.Flags().String("from", "", "only threads active on/after this date (ISO-8601)")
	cmd.Flags().String("to", "", "only threads active on/before this date (ISO-8601)")
	addPaginationFlags(cmd)
	return cmd
}

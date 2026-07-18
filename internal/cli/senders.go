package cli

import (
	"fmt"
	"os"
	"strconv"

	"codeberg.org/kehvyn/horton-cli/internal/api"
	"codeberg.org/kehvyn/horton-cli/internal/output"
	"github.com/spf13/cobra"
)

func senderStatCells(s *api.SenderStats) (count, first, last string) {
	if s == nil {
		return "-", "-", "-"
	}
	return strconv.Itoa(s.MessageCount), output.OrDash(s.FirstMessageAt), output.OrDash(s.LastMessageAt)
}

func newSendersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "senders",
		Short: "Browse people who have posted to the lists",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectPaginationWithIDs(cmd); err != nil {
				return err
			}
			q := collectQuery(cmd, "q", "sort", "dir", "limit", "after", "before")
			ids, err := collectIDs(cmd, cmd.InOrStdin())
			if err != nil {
				return err
			}
			// Passed through unchecked: the server drops non-integers silently,
			// and erroring here would break subset semantics on a piped list.
			for _, id := range ids {
				q.Add("ids[]", id)
			}
			return getRender(cmd, "/senders", q, func(page api.Page[api.SenderSummary]) {
				rows := make([][]string, 0, len(page.Data))
				for _, s := range page.Data {
					count, first, last := senderStatCells(s.Stats)
					rows = append(rows, []string{
						strconv.Itoa(s.ID), output.Truncate(s.DisplayName, 32),
						s.Email, count, first, last,
					})
				}
				output.Table(os.Stdout,
					[]string{"ID", "NAME", "EMAIL", "MESSAGES", "FIRST", "LAST"}, rows)
				output.CursorFooter(page.NextCursor)
			})
		},
	}
	cmd.Flags().String("q", "", "substring match on email or display name")
	addEnumFlag(cmd, "sort", "sort key", "messages", "first", "last")
	addEnumFlag(cmd, "dir", "sort direction", "asc", "desc")
	cmd.Flags().Int("limit", 0, "page size (server default 25, max 100)")
	cmd.Flags().String("after", "", "page cursor")
	cmd.Flags().String("before", "", "page cursor")
	addIDFlag(cmd, "fetch exactly this sender id")

	cmd.AddCommand(newSendersGetCmd())
	return cmd
}

func newSendersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Show one sender and their recent messages",
		Args:  requireArg("a sender id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("sender id must be an integer, got %q", args[0])
			}
			return getRender(cmd, fmt.Sprintf("/senders/%d", id), nil,
				func(item api.Item[api.SenderFull]) {
					s := item.Data
					count, first, last := senderStatCells(s.Stats)
					output.Detail(os.Stdout, [][2]string{
						{"Id", strconv.Itoa(s.ID)},
						{"Name", s.DisplayName},
						{"Email", s.Email},
						{"Messages", count},
						{"First", first},
						{"Last", last},
					})
					if len(s.Messages) > 0 {
						fmt.Println()
						rows := make([][]string, 0, len(s.Messages))
						for _, m := range s.Messages {
							rows = append(rows, []string{
								m.SentAt, output.Truncate(m.Subject, 64), m.MessageID,
							})
						}
						output.Table(os.Stdout, []string{"SENT AT", "SUBJECT", "MESSAGE-ID"}, rows)
					}
				})
		},
	}
}

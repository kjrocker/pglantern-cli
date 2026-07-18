package cli

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"codeberg.org/kehvyn/horton-cli/internal/api"
	"codeberg.org/kehvyn/horton-cli/internal/output"
	"github.com/spf13/cobra"
)

// normalizeMessageID strips the optional <> a mail header wraps a Message-Id
// in; the archive stores (and routes on) the bracket-stripped RFC id.
func normalizeMessageID(arg string) string {
	return strings.Trim(arg, "<>")
}

// messagePath builds a /messages path, percent-encoding the raw Message-Id into
// a single path segment (url.PathEscape turns "/" into "%2F"), which the API
// percent-decodes back to the raw id.
func messagePath(arg string, suffix string) string {
	return "/messages/" + url.PathEscape(normalizeMessageID(arg)) + suffix
}

// senderDisplay renders the "From" column as "Name <email>". The thread and
// single-message endpoints preload the sender, so we assemble that form from
// its parts (display name + email); search doesn't preload it, so we fall back
// to from_raw, which the archive already stores in the same "Name <email>"
// form. A preloaded sender with no display name degrades to the bare email.
func senderDisplay(m api.MessageSummary) string {
	if m.Sender != nil && m.Sender.Email != "" {
		if m.Sender.DisplayName != "" {
			return fmt.Sprintf("%s <%s>", m.Sender.DisplayName, m.Sender.Email)
		}
		return m.Sender.Email
	}
	return m.FromRaw
}

func messageRows(msgs []api.MessageSummary) [][]string {
	rows := make([][]string, 0, len(msgs))
	for _, m := range msgs {
		rows = append(rows, []string{
			m.SentAt,
			output.Truncate(senderDisplay(m), 32),
			output.Truncate(m.Subject, 64),
			m.MessageID,
		})
	}
	return rows
}

func messageTable(msgs []api.MessageSummary) {
	output.Table(os.Stdout, []string{"SENT AT", "FROM", "SUBJECT", "MESSAGE-ID"}, messageRows(msgs))
}

func commitRows(commits []api.CommitSummary) [][]string {
	rows := make([][]string, 0, len(commits))
	for _, c := range commits {
		rows = append(rows, []string{
			c.SHA,
			output.Truncate(fmt.Sprintf("%s <%s>", c.AuthorName, c.AuthorEmail), 40),
			c.CommittedAt,
			output.Truncate(c.Subject, 64),
		})
	}
	return rows
}

func newMessagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Browse archived messages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectPaginationWithIDs(cmd); err != nil {
				return err
			}
			q := collectQuery(cmd, "list", "from", "to", "dir", "limit", "after", "before")
			ids, err := collectIDs(cmd, cmd.InOrStdin())
			if err != nil {
				return err
			}
			for _, id := range ids {
				q.Add("ids[]", normalizeMessageID(id))
			}
			return getRender(cmd, "/messages", q, func(page api.Page[api.MessageSummary]) {
				messageTable(page.Data)
				output.CursorFooter(page.NextCursor)
			})
		},
	}
	cmd.Flags().String("list", "", "restrict to one mailing list by name")
	cmd.Flags().String("from", "", "ISO-8601 lower bound on sent_at")
	cmd.Flags().String("to", "", "ISO-8601 upper bound on sent_at")
	addEnumFlag(cmd, "dir", "sort direction", "asc", "desc")
	cmd.Flags().Int("limit", 0, "page size (server default 25, max 100)")
	cmd.Flags().String("after", "", "page cursor")
	cmd.Flags().String("before", "", "page cursor")
	addIDFlag(cmd, "fetch exactly this Message-Id")

	cmd.AddCommand(
		newMessagesGetCmd(),
		newMessagesThreadCmd(),
		newMessagesCommitsCmd(),
		newMessagesRefsCmd(),
	)
	return cmd
}

func newMessagesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <message-id>",
		Short: "Show one message by Message-Id",
		Args:  requireArg("a message id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := messagePath(args[0], "")
			return getRender(cmd, path, nil, func(item api.Item[api.MessageFull]) {
				m := item.Data
				pairs := [][2]string{
					{"Message-Id", m.MessageID},
					{"Subject", m.Subject},
					{"From", m.FromRaw},
					{"To", m.ToRaw},
				}
				if m.CcRaw != "" {
					pairs = append(pairs, [2]string{"Cc", m.CcRaw})
				}
				pairs = append(pairs,
					[2]string{"Sent at", m.SentAt},
					[2]string{"Thread", m.ThreadID},
				)
				output.Detail(os.Stdout, pairs)
				if len(m.Attachments) > 0 {
					fmt.Println()
					rows := make([][]string, 0, len(m.Attachments))
					for _, a := range m.Attachments {
						patch := ""
						if a.IsPatch {
							patch = "patch"
						}
						rows = append(rows, []string{fmt.Sprint(a.ID), a.Filename, a.ContentType, patch})
					}
					output.Table(os.Stdout, []string{"ATTACHMENT", "FILENAME", "TYPE", ""}, rows)
				}
				fmt.Println()
				fmt.Println(m.BodyText)
			})
		},
	}
}

func newMessagesThreadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "thread <message-id>",
		Short: "Show the whole thread containing a message",
		Args:  requireArg("a message id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := messagePath(args[0], "/thread")
			return getRender(cmd, path, nil, func(page api.Page[api.MessageSummary]) {
				messageTable(page.Data)
			})
		},
	}
}

func newMessagesCommitsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commits <message-id>",
		Short: "Show commits that landed from a message's thread",
		Args:  requireArg("a message id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := messagePath(args[0], "/commits")
			return getRender(cmd, path, nil, func(page api.Page[api.CommitGroup]) {
				if len(page.Data) == 0 {
					fmt.Fprintln(os.Stderr, "no commits landed from this thread")
					return
				}
				for i, group := range page.Data {
					if i > 0 {
						fmt.Println()
					}
					fmt.Printf("# %s\n", group.Subject)
					output.Table(os.Stdout, []string{"SHA", "COMMITTED AT", "AUTHOR", "SUBJECT"},
						commitRows(group.Commits))
				}
			})
		},
	}
}

func newMessagesRefsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refs <message-id>",
		Short: "Show references extracted from a message (shas, paths, CVEs, …)",
		Args:  requireArg("a message id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := messagePath(args[0], "/refs")
			return getRender(cmd, path, nil, func(page api.Page[api.Ref]) {
				rows := make([][]string, 0, len(page.Data))
				for _, r := range page.Data {
					rows = append(rows, []string{r.RefType, r.Value, output.OrDash(r.ResolvedSHA)})
				}
				output.Table(os.Stdout, []string{"TYPE", "VALUE", "RESOLVED SHA"}, rows)
			})
		},
	}
}

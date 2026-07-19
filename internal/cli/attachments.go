package cli

import (
	"fmt"
	"os"
	"strconv"

	"codeberg.org/kehvyn/pglantern-cli/internal/api"
	"codeberg.org/kehvyn/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAttachmentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachments",
		Short: "Browse message attachments",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "limit", "after", "before")
			return getRenderPage(cmd, "/attachments", q, func(page api.Page[api.AttachmentRow]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no results")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, a := range page.Data {
					rows = append(rows, []string{
						strconv.Itoa(a.ID), output.Truncate(a.Filename, 40), a.ContentType,
						output.HumanBytes(int64(a.Size)), patchCell(a.IsPatch), output.OrDash(a.MessageID),
					})
				}
				output.Table(os.Stdout,
					[]string{"ID", "FILENAME", "TYPE", "SIZE", "PATCH", "MESSAGE-ID"}, rows)
				output.CursorFooter(page.NextCursor)
			})
		},
	}
	addPaginationFlags(cmd)

	cmd.AddCommand(newAttachmentsGetCmd(), newAttachmentsPatchCmd())
	return cmd
}

func attachmentID(arg string) (int, error) {
	id, err := strconv.Atoi(arg)
	if err != nil {
		return 0, usagef("attachment id must be an integer, got %q", arg)
	}
	return id, nil
}

func newAttachmentsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Show one attachment's metadata",
		Args:  requireArg("an attachment id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := attachmentID(args[0])
			if err != nil {
				return err
			}
			return getRender(cmd, fmt.Sprintf("/attachments/%d", id), nil,
				func(item api.Item[api.AttachmentRow]) {
					a := item.Data
					output.Detail(os.Stdout, [][2]string{
						{"Id", strconv.Itoa(a.ID)},
						{"Filename", a.Filename},
						{"Type", a.ContentType},
						{"Size", output.HumanBytes(int64(a.Size))},
						{"Patch", strconv.FormatBool(a.IsPatch)},
						{"Message-Id", output.OrDash(a.MessageID)},
					})
				})
		},
	}
}

func newAttachmentsPatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "patch <id>",
		Short: "Show the parsed patch summary for a patch attachment",
		Args:  requireArg("an attachment id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := attachmentID(args[0])
			if err != nil {
				return err
			}
			return getRender(cmd, fmt.Sprintf("/attachments/%d/patch", id), nil,
				func(item api.Item[api.Patch]) {
					p := item.Data
					version := "-"
					if p.SeriesVersion != nil {
						version = fmt.Sprintf("v%d", *p.SeriesVersion)
						if p.SeriesSeq != nil {
							version = fmt.Sprintf("%s patch %d", version, *p.SeriesSeq)
						}
					}
					output.Detail(os.Stdout, [][2]string{
						{"Attachment", strconv.Itoa(id)},
						{"Format", p.Format},
						{"Series", version},
						{"Subject", p.Subject},
					})
					if len(p.Files) > 0 {
						fmt.Println()
						rows := make([][]string, 0, len(p.Files))
						for _, f := range p.Files {
							rows = append(rows, []string{
								f.Path, fmt.Sprintf("+%d", f.Additions), fmt.Sprintf("-%d", f.Deletions),
							})
						}
						output.Table(os.Stdout, []string{"PATH", "ADD", "DEL"}, rows)
					}
				})
		},
	}
}

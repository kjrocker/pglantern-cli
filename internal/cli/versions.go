package cli

import (
	"fmt"
	"net/url"
	"os"

	"codeberg.org/kehvyn/horton-cli/internal/api"
	"codeberg.org/kehvyn/horton-cli/internal/output"
	"github.com/spf13/cobra"
)

func newVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "List postgres major versions and their releases",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getRender(cmd, "/versions", nil, func(page api.Page[api.Major]) {
				rows := make([][]string, 0, len(page.Data))
				for _, m := range page.Data {
					latest := "-"
					if n := len(m.Releases); n > 0 {
						latest = m.Releases[n-1].Tag
					}
					rows = append(rows, []string{
						m.Major, output.OrDash(m.ReleasedOn), output.OrDash(m.EolOn), latest,
					})
				}
				output.Table(os.Stdout, []string{"MAJOR", "RELEASED", "EOL", "LATEST TAG"}, rows)
			})
		},
	}
	cmd.AddCommand(newVersionsGucsCmd())
	cmd.AddCommand(newVersionsDocsCmd())
	return cmd
}

func newVersionsDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs <major>",
		Short: "Show a major's documentation table of contents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/versions/" + url.PathEscape(args[0]) + "/docs"
			return getRender(cmd, path, nil, func(item api.Item[api.DocPages]) {
				rows := make([][]string, 0, len(item.Data.Docs))
				for _, d := range item.Data.Docs {
					title := d.SgmlSource
					if d.Title != nil {
						title = *d.Title
					}
					rows = append(rows, []string{title, d.SgmlSource, d.URL})
				}
				output.Table(os.Stdout, []string{"TITLE", "SOURCE", "URL"}, rows)
			})
		},
	}
}

func gucChangeRows(changes []api.GucChange) [][]string {
	rows := make([][]string, 0, len(changes))
	for _, c := range changes {
		rows = append(rows, []string{c.Name, c.Vartype, c.From, c.To})
	}
	return rows
}

func gucRows(gucs []api.Guc) [][]string {
	rows := make([][]string, 0, len(gucs))
	for _, g := range gucs {
		rows = append(rows, []string{g.Name, g.Vartype, g.BootVal, g.Context, g.Category})
	}
	return rows
}

func newVersionsGucsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gucs <major>",
		Short: "Show a major's GUC catalog, or its diff against another major",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/versions/" + url.PathEscape(args[0]) + "/gucs"
			q := collectQuery(cmd, "changed-since")

			// The response shape switches with the flag: catalog without
			// --changed-since, diff with it.
			if !cmd.Flags().Changed("changed-since") {
				return getRender(cmd, path, q, func(item api.Item[api.GucCatalog]) {
					output.Table(os.Stdout,
						[]string{"NAME", "TYPE", "DEFAULT", "CONTEXT", "CATEGORY"},
						gucRows(item.Data.Gucs))
				})
			}
			return getRender(cmd, path, q, func(item api.Item[api.GucDiff]) {
				d := item.Data
				sections := []struct {
					title  string
					header []string
					rows   [][]string
				}{
					{"added", []string{"NAME", "TYPE", "DEFAULT", "CONTEXT", "CATEGORY"}, gucRows(d.Added)},
					{"removed", []string{"NAME", "TYPE", "DEFAULT", "CONTEXT", "CATEGORY"}, gucRows(d.Removed)},
					{"default changed", []string{"NAME", "TYPE", "FROM", "TO"}, gucChangeRows(d.DefaultChanged)},
					{"context changed", []string{"NAME", "TYPE", "FROM", "TO"}, gucChangeRows(d.ContextChanged)},
				}
				first := true
				for _, s := range sections {
					if len(s.rows) == 0 {
						continue
					}
					if !first {
						fmt.Println()
					}
					first = false
					fmt.Printf("# %s (%s → %s)\n", s.title, d.ChangedSince, d.Major)
					output.Table(os.Stdout, s.header, s.rows)
				}
				if first {
					fmt.Fprintf(os.Stderr, "no GUC changes between %s and %s\n", d.ChangedSince, d.Major)
				}
			})
		},
	}
	cmd.Flags().String("changed-since", "", "diff against this base major (16, 9.6, master)")
	return cmd
}

package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Table prints a header row and data rows aligned with tabwriter.
func Table(w io.Writer, header []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(header, "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}

// Detail prints aligned "key:  value" pairs for single-resource views.
func Detail(w io.Writer, pairs [][2]string) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	for _, p := range pairs {
		fmt.Fprintf(tw, "%s:\t%s\n", p[0], p[1])
	}
	tw.Flush()
}

// CursorFooter surfaces the next-page cursor on stderr so tables stay
// pipe-clean while pagination remains discoverable.
func CursorFooter(next *string) {
	if next != nil && *next != "" {
		fmt.Fprintf(os.Stderr, "# next: --after %s\n", *next)
	}
}

// OrDash renders optional strings in table cells.
func OrDash(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}

// Truncate shortens long free-text cells (subjects, senders) so rows stay
// readable; IDs and cursors are never truncated.
func Truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

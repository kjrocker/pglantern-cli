package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"unicode"
)

// Table prints a header row and data rows aligned with tabwriter.
func Table(w io.Writer, header []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, sanitizeRow(header))
	for _, row := range rows {
		fmt.Fprintln(tw, sanitizeRow(row))
	}
	tw.Flush()
}

// Detail prints aligned "key:  value" pairs for single-resource views.
func Detail(w io.Writer, pairs [][2]string) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	for _, p := range pairs {
		fmt.Fprintf(tw, "%s:\t%s\n", sanitizeCell(p[0]), sanitizeCell(p[1]))
	}
	tw.Flush()
}

// sanitizeRow sanitizes each cell then joins with the tab column delimiter.
func sanitizeRow(cells []string) string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = sanitizeCell(c)
	}
	return strings.Join(out, "\t")
}

// sanitizeCell collapses any control characters (newlines, tabs, carriage
// returns) inside a value into single spaces. Mail headers get folded across
// physical lines, so raw subjects and sender strings can carry embedded
// newlines/tabs; left alone they split a logical row across multiple physical
// lines and corrupt columns, breaking the one-record-per-line, \t-delimited
// contract awk/sed pipes rely on.
func sanitizeCell(s string) string {
	if !strings.ContainsFunc(s, unicode.IsControl) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsControl(r) || r == ' ' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimRight(b.String(), " ")
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

// HumanBytes renders a byte count as a compact, human-readable size
// (e.g. 512 B, 1.4 KB, 3.0 MB) using 1024-based units.
func HumanBytes(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for i := int64(n) / unit; i >= unit; i /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
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

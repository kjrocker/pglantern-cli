// Package output renders API responses: raw JSON passthrough or tabwriter
// tables.
package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
)

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// Interactive reports whether stdout was a terminal at startup, computed once
// before any pager reassigns os.Stdout. Table truncation and JSON indenting
// key off it; tests may override.
var Interactive = isTTY(os.Stdout)

// JSON writes the raw response body: indented for a terminal, verbatim
// (compact, jq-friendly) for a pipe. The os.Stdout comparison keeps the
// terminal treatment when a pager has replaced stdout with a pipe.
func JSON(w io.Writer, body []byte) error {
	if isTTY(w) || (w == os.Stdout && Interactive) {
		var buf bytes.Buffer
		if err := json.Indent(&buf, body, "", "  "); err == nil {
			buf.WriteByte('\n')
			_, err := w.Write(buf.Bytes())
			return err
		}
	}
	if len(body) > 0 && body[len(body)-1] != '\n' {
		body = append(body, '\n')
	}
	_, err := w.Write(body)
	return err
}

// JSONLine writes one response body as a single compact line — the --all
// streaming shape: one JSON doc per page, so `jq .data[]` flattens the run.
func JSONLine(w io.Writer, body []byte) error {
	var buf bytes.Buffer
	if err := json.Compact(&buf, body); err != nil {
		buf.Reset()
		buf.Write(bytes.TrimSpace(body))
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

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

// JSON writes the raw response body: indented for a terminal, verbatim
// (compact, jq-friendly) for a pipe.
func JSON(w io.Writer, body []byte) error {
	if isTTY(w) {
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

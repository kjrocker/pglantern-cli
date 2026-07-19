package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestJSONNonTTYIsVerbatim(t *testing.T) {
	var buf bytes.Buffer
	body := `{"data":[{"a":1}]}`
	if err := JSON(&buf, []byte(body)); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != body+"\n" {
		t.Errorf("got %q, want raw body + newline", got)
	}
}

// setInteractive overrides the TTY detection for the duration of a test.
func setInteractive(t *testing.T, v bool) {
	t.Helper()
	orig := Interactive
	Interactive = v
	t.Cleanup(func() { Interactive = orig })
}

func TestTruncate(t *testing.T) {
	setInteractive(t, true)
	tests := []struct{ in, want string }{
		{"short", "short"},
		{"exactly-ten", "exactly-t…"},
		{"héllo wörld again", "héllo wör…"},
	}
	for _, tt := range tests {
		if got := Truncate(tt.in, 10); got != tt.want {
			t.Errorf("Truncate(%q, 10) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTruncateNonInteractivePassesThrough(t *testing.T) {
	// Pipes get full data — truncation is a terminal-readability concern only.
	setInteractive(t, false)
	long := strings.Repeat("x", 200)
	if got := Truncate(long, 10); got != long {
		t.Errorf("non-interactive Truncate altered the value: %q", got)
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{3145728, "3.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		if got := HumanBytes(tt.in); got != tt.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSanitizeCell(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain subject", "plain subject"},
		{"Re: patch\n follow-up", "Re: patch follow-up"},
		{"a\tb\tc", "a b c"},
		{"trailing\n", "trailing"},
		{"crlf\r\nfold", "crlf fold"},
	}
	for _, tt := range tests {
		if got := sanitizeCell(tt.in); got != tt.want {
			t.Errorf("sanitizeCell(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// A cell carrying an embedded newline must not split its row across physical
// lines or shift the trailing column — awk/sed pipes depend on one record per
// line with strict \t-delimited fields.
func TestTableStaysOneLinePerRow(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, []string{"SENT AT", "FROM", "SUBJECT", "MESSAGE-ID"}, [][]string{
		{"2026-01-02", "a@b.com", "Re: patch\n follow-up wrap", "<abc123@mail>"},
	})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (header + row):\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[1], "<abc123@mail>") {
		t.Errorf("message-id not on the data row: %q", lines[1])
	}
}

func TestEmptyNote(t *testing.T) {
	origErr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	EmptyNote("no results")
	w.Close()
	os.Stderr = origErr
	b, _ := io.ReadAll(r)
	if got := string(b); got != "# no results\n" {
		t.Errorf("EmptyNote wrote %q, want %q", got, "# no results\n")
	}
}

func TestJSONLine(t *testing.T) {
	var buf bytes.Buffer
	if err := JSONLine(&buf, []byte("{\n  \"data\": [1]\n}\n")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != `{"data":[1]}`+"\n" {
		t.Errorf("JSONLine = %q, want one compact line", got)
	}
}

func TestOrDash(t *testing.T) {
	if got := OrDash(nil); got != "-" {
		t.Errorf("OrDash(nil) = %q", got)
	}
	empty := ""
	if got := OrDash(&empty); got != "-" {
		t.Errorf("OrDash(&\"\") = %q", got)
	}
	v := "x"
	if got := OrDash(&v); got != "x" {
		t.Errorf("OrDash(&\"x\") = %q", got)
	}
}

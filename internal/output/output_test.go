package output

import (
	"bytes"
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

func TestTruncate(t *testing.T) {
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

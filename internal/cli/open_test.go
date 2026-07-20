package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEncodeUnreserved(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// A Message-Id's @ is percent-encoded — stricter than url.PathEscape,
		// matching the server's URI.encode(s, &URI.char_unreserved?/1).
		{"a1@example.com", "a1%40example.com"},
		{"a/b", "a%2Fb"},
		// The RFC 3986 unreserved set survives verbatim.
		{"~-_.", "~-_."},
		{"AZaz09", "AZaz09"},
		{"CAFiTN-sF_J8=2+x@mail.gmail.com", "CAFiTN-sF_J8%3D2%2Bx%40mail.gmail.com"},
	}
	for _, tt := range tests {
		if got := encodeUnreserved(tt.in); got != tt.want {
			t.Errorf("encodeUnreserved(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsFullSHA(t *testing.T) {
	full := "51cd5d6f052306e9288ff8c162ca9596432a5d2e" // 40 hex
	if !isFullSHA(full) {
		t.Errorf("isFullSHA(%q) = false, want true", full)
	}
	tests := []string{
		full[:39],             // 39
		full + "a",            // 41
		"51cd5d6g" + full[8:], // non-hex g at position 7
		"51cd",                // short prefix
		"",                    // empty
	}
	for _, s := range tests {
		if isFullSHA(s) {
			t.Errorf("isFullSHA(%q) = true, want false", s)
		}
	}
}

func TestMessageOpenPrint(t *testing.T) {
	// Default: the upstream archive page, host-independent.
	out := captureStdout(t, func() {
		root := NewRootCmd()
		root.SetArgs([]string{"messages", "open", "<a1@example.com>", "--print"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if got := strings.TrimSpace(out); got != "https://postgr.es/m/a1%40example.com" {
		t.Errorf("archive url = %q", got)
	}

	// --site: our own browse page, on the resolved host.
	out = captureStdout(t, func() {
		root := NewRootCmd()
		root.SetArgs([]string{"messages", "open", "<a1@example.com>",
			"--site", "--print", "--host", "https://lantern.example.com/"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if got := strings.TrimSpace(out); got != "https://lantern.example.com/messages/a1%40example.com" {
		t.Errorf("site url = %q", got)
	}
}

func TestCommitOpenFullSHASkipsAPI(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	defer srv.Close()

	full := "51cd5d6f052306e9288ff8c162ca9596432a5d2e"
	out := captureStdout(t, func() {
		root := NewRootCmd()
		root.SetArgs([]string{"commits", "open", full, "--print",
			"--host", srv.URL, "--api-key", "k"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if hits != 0 {
		t.Errorf("full sha hit the server %d times, want 0", hits)
	}
	if got := strings.TrimSpace(out); got != "https://postgr.es/c/"+full {
		t.Errorf("archive url = %q", got)
	}
}

func TestCommitOpenPrefixHitsAPI(t *testing.T) {
	const (
		wantArchive = "https://postgr.es/c/51cd5d6f052306e9288ff8c162ca9596432a5d2e"
		wantHTML    = "https://pglantern.com/commits/51cd5d6f052306e9288ff8c162ca9596432a5d2e"
	)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		fmt.Fprintf(w, `{"data":{"sha":"51cd5d6f052306e9288ff8c162ca9596432a5d2e",`+
			`"archive_url":%q,"html_url":%q}}`, wantArchive, wantHTML)
	}))
	defer srv.Close()

	// A prefix resolves server-side and opens the server's canonical archive URL.
	out := captureStdout(t, func() {
		root := NewRootCmd()
		root.SetArgs([]string{"commits", "open", "51cd5d6", "--print",
			"--host", srv.URL, "--api-key", "k"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if hits != 1 {
		t.Errorf("prefix made %d requests, want 1", hits)
	}
	if got := strings.TrimSpace(out); got != wantArchive {
		t.Errorf("archive url = %q, want %q", got, wantArchive)
	}

	// --site opens the server's html_url instead.
	out = captureStdout(t, func() {
		root := NewRootCmd()
		root.SetArgs([]string{"commits", "open", "51cd5d6", "--site", "--print",
			"--host", srv.URL, "--api-key", "k"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if got := strings.TrimSpace(out); got != wantHTML {
		t.Errorf("site url = %q, want %q", got, wantHTML)
	}
}

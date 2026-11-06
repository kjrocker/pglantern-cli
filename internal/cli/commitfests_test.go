package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// runStub runs the CLI against a stub server returning body, and reports the
// request path, raw query and captured stdout.
func runStub(t *testing.T, body string, args ...string) (path, query, out string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.EscapedPath()
		query = r.URL.RawQuery
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	out = captureStdout(t, func() {
		root := NewRootCmd()
		root.SetArgs(append(args, "--host", srv.URL, "--api-key", "k"))
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	return
}

func TestCommitfestsList(t *testing.T) {
	body := `{"data":[{"id":52,"name":"2025-03","status":"in_progress","status_label":"In Progress",
	  "draft":false,"start_date":"2025-03-01","end_date":"2025-03-31",
	  "entry_counts":{"needs_review":120,"committed":34,"moved":80},"source_url":"x"}]}`
	path, query, out := runStub(t, body, "commitfests", "--limit", "5")
	if path != "/api/v1/commitfests" && path != "/commitfests" {
		t.Errorf("path = %q", path)
	}
	if !strings.Contains(query, "limit=5") {
		t.Errorf("query = %q", query)
	}
	for _, want := range []string{"2025-03", "in_progress", "2025-03-31", "120", "234"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCommitfestsPatches(t *testing.T) {
	body := `{"data":[{"id":4242,"name":"Btree split heuristic","authors":["Tom Example"],
	  "last_email_at":null,"current":null,"entry":{"cf_id":52,"cf_name":"2025-03","status":"needs_review",
	  "status_label":"Needs review","entered_at":"2025-01-20T10:15:00Z","left_at":null},"source_url":"x"}]}`
	path, query, out := runStub(t, body, "commitfests", "patches", "52", "--status", "ready_for_committer")
	if !strings.HasSuffix(path, "/commitfests/52/patches") {
		t.Errorf("path = %q", path)
	}
	if !strings.Contains(query, "status=ready_for_committer") {
		t.Errorf("query = %q", query)
	}
	for _, want := range []string{"4242", "needs_review", "Btree split heuristic", "Tom Example"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if err := newCommitfestsPatchesCmd().Flags().Parse([]string{"--status", "banana"}); err == nil {
		t.Error("invalid --status accepted")
	}
}

func TestCommitfestsPatchDetail(t *testing.T) {
	body := `{"data":{"id":4242,"name":"Btree split heuristic","authors":["Tom Example"],
	  "last_email_at":null,"source_url":"https://commitfest.postgresql.org/patch/4242/",
	  "current":{"cf_id":52,"cf_name":"2025-03","status":"needs_review","status_label":"Needs review",
	    "entered_at":"2025-01-20T10:15:00Z","left_at":null},
	  "history":[{"cf_id":51,"cf_name":"2025-01","status":"moved","status_label":"Moved",
	    "entered_at":"2024-11-02T09:00:00Z","left_at":"2025-01-20T10:15:00Z"}],
	  "threads":[{"message_id":"CAunresolved@x","thread_id":null,"subject":"Gone thread",
	    "latest_message_id":null,"latest_message_at":null,"has_attachment":false}],
	  "commits":[]}}`
	path, _, out := runStub(t, body, "commitfests", "patch", "4242")
	if !strings.HasSuffix(path, "/cf-patches/4242") {
		t.Errorf("path = %q", path)
	}
	for _, want := range []string{"patch 4242", "needs_review (2025-03)", "Tom Example", "2024-11-02", "CAunresolved@x"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// the unresolved thread's id column is a dash
	var threadLine string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "Gone thread") {
			threadLine = l
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(threadLine), "-") {
		t.Errorf("unresolved thread line = %q, want leading dash", threadLine)
	}
}

func TestMessagesCommitfest(t *testing.T) {
	body := `{"data":[{"id":4242,"name":"Btree split heuristic","authors":[],"last_email_at":null,
	  "current":{"cf_id":52,"cf_name":"2025-03","status":"needs_review","status_label":"x",
	  "entered_at":"2025-01-20T10:15:00Z","left_at":null},"source_url":"x"}]}`
	path, _, out := runStub(t, body, "messages", "commitfest", "abc@example.com")
	if !strings.HasSuffix(path, "/commitfest") || !strings.Contains(path, "/messages/") {
		t.Errorf("path = %q", path)
	}
	for _, want := range []string{"4242", "needs_review", "2025-03", "Btree split heuristic"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	_, _, out = runStub(t, `{"data":[]}`, "messages", "commitfest", "abc@example.com")
	// the empty note goes to stderr; stdout stays clean for pipes
	if out != "" {
		t.Errorf("empty stdout = %q, want nothing", out)
	}
}

func TestCfStatusFlag(t *testing.T) {
	for _, args := range [][]string{
		{"search", "vacuum", "--cf-status", "waiting_on_author"},
		{"threads", "vacuum", "--cf-status", "waiting_on_author"},
	} {
		_, query, _ := runStub(t, `{"data":[]}`, args...)
		if !strings.Contains(query, "cf_status=waiting_on_author") {
			t.Errorf("%v: query = %q", args, query)
		}
	}
	if err := newSearchCmd().Flags().Parse([]string{"--cf-status", "banana"}); err == nil {
		t.Error("invalid --cf-status accepted")
	}
}

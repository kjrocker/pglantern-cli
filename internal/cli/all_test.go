package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"codeberg.org/kehvyn/pglantern-cli/internal/api"
	"github.com/spf13/cobra"
)

type row struct {
	N int `json:"n"`
}

// allCmd is a bare command wired to hit srv with the flags getRenderAll reads.
func allCmd(host string) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("json", false, "")
	cmd.Flags().String("host", host, "")
	cmd.Flags().String("api-key", "k", "")
	addPaginationFlags(cmd)
	addIDFlag(cmd, "an id")
	return cmd
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = orig
	b, _ := io.ReadAll(r)
	return string(b)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	b, _ := io.ReadAll(r)
	return string(b)
}

// pagedServer serves total rows in pages of the requested limit, minting
// numeric cursors, and records each request's limit and after params.
func pagedServer(t *testing.T, total int, gotLimits, gotAfters *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		*gotLimits = append(*gotLimits, q.Get("limit"))
		*gotAfters = append(*gotAfters, q.Get("after"))
		start := 0
		fmt.Sscanf(q.Get("after"), "c%d", &start)
		limit := 0
		fmt.Sscanf(q.Get("limit"), "%d", &limit)
		end := start + limit
		if end > total {
			end = total
		}
		var items []string
		for i := start; i < end; i++ {
			items = append(items, fmt.Sprintf(`{"n":%d}`, i))
		}
		next := "null"
		if end < total {
			next = fmt.Sprintf(`"c%d"`, end)
		}
		fmt.Fprintf(w, `{"data":[%s],"next_cursor":%s}`, strings.Join(items, ","), next)
	}))
}

func TestAllFollowsCursors(t *testing.T) {
	var gotLimits, gotAfters []string
	srv := pagedServer(t, 150, &gotLimits, &gotAfters)
	defer srv.Close()

	cmd := allCmd(srv.URL)
	if err := cmd.Flags().Parse([]string{"--all"}); err != nil {
		t.Fatal(err)
	}
	var got []row
	stderr := captureStderr(t, func() {
		if err := getRenderAll(cmd, "/things", url.Values{}, func(page api.Page[row]) {
			got = page.Data
		}); err != nil {
			t.Error(err)
		}
	})
	if len(got) != 150 {
		t.Fatalf("got %d rows, want all 150", len(got))
	}
	// With --limit unset, --all asks for full 100-row pages, strictly in
	// sequence: page one uncursored, page two after c100.
	if strings.Join(gotLimits, ",") != "100,100" {
		t.Errorf("limits = %v, want [100 100]", gotLimits)
	}
	if strings.Join(gotAfters, ",") != ",c100" {
		t.Errorf("afters = %v", gotAfters)
	}
	// Nothing was truncated, so no resume note.
	if stderr != "" {
		t.Errorf("unexpected stderr: %q", stderr)
	}
}

func TestAllStopsAtMaxWithResumeNote(t *testing.T) {
	var gotLimits, gotAfters []string
	srv := pagedServer(t, 500, &gotLimits, &gotAfters)
	defer srv.Close()

	cmd := allCmd(srv.URL)
	if err := cmd.Flags().Parse([]string{"--all", "--max", "250"}); err != nil {
		t.Fatal(err)
	}
	var got []row
	stderr := captureStderr(t, func() {
		if err := getRenderAll(cmd, "/things", url.Values{}, func(page api.Page[row]) {
			got = page.Data
		}); err != nil {
			t.Error(err)
		}
	})
	// Exactly the cap: the final page's limit is trimmed to the remainder.
	if len(got) != 250 {
		t.Fatalf("got %d rows, want exactly 250", len(got))
	}
	if strings.Join(gotLimits, ",") != "100,100,50" {
		t.Errorf("limits = %v, want [100 100 50]", gotLimits)
	}
	// The note carries the cursor of the next unfetched row, so the run is
	// resumable, never silently truncated.
	if !strings.Contains(stderr, "stopped at 250 rows") || !strings.Contains(stderr, "--after c250") {
		t.Errorf("resume note missing or wrong: %q", stderr)
	}
}

func TestAllAbortsOnPageErrorWithResumeCursor(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			fmt.Fprint(w, `{"data":[{"n":1}],"next_cursor":"c1"}`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"code":"internal","message":"boom"}}`)
	}))
	defer srv.Close()

	cmd := allCmd(srv.URL)
	if err := cmd.Flags().Parse([]string{"--all"}); err != nil {
		t.Fatal(err)
	}
	var err error
	stderr := captureStderr(t, func() {
		err = getRenderAll(cmd, "/things", url.Values{}, func(page api.Page[row]) {})
	})
	if err == nil {
		t.Fatal("mid-run page failure did not abort")
	}
	if !strings.Contains(stderr, "--after c1") {
		t.Errorf("stderr %q does not name the resume cursor of the last good page", stderr)
	}
}

func TestAllJSONStreamsOnePagePerLine(t *testing.T) {
	var gotLimits, gotAfters []string
	srv := pagedServer(t, 150, &gotLimits, &gotAfters)
	defer srv.Close()

	cmd := allCmd(srv.URL)
	if err := cmd.Flags().Parse([]string{"--all", "--json"}); err != nil {
		t.Fatal(err)
	}
	origOut := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	rendered := false
	runErr := getRenderAll(cmd, "/things", url.Values{}, func(page api.Page[row]) { rendered = true })
	w.Close()
	os.Stdout = origOut
	if runErr != nil {
		t.Fatal(runErr)
	}
	b, _ := io.ReadAll(r)
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want one per page (2):\n%s", len(lines), b)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, `{"data":[`) {
			t.Errorf("line is not a raw page doc: %q", line)
		}
	}
	// JSON mode streams; the table render callback must not also fire.
	if rendered {
		t.Error("render callback fired in --json mode")
	}
}

func TestAllFlagConflicts(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--all", "--before", "c"}, "--before"},
		{[]string{"--all", "--id", "x"}, "--id"},
		{[]string{"--all", "--max", "0"}, "--max"},
		{[]string{"--all", "--max", "-5"}, "--max"},
	}
	for _, tt := range tests {
		cmd := allCmd("http://unused.invalid")
		if err := cmd.Flags().Parse(tt.args); err != nil {
			t.Fatal(err)
		}
		err := getRenderAll(cmd, "/things", url.Values{}, func(page api.Page[row]) {})
		if err == nil {
			t.Errorf("args %v accepted, want usage error", tt.args)
			continue
		}
		if ExitCode(err) != 2 {
			t.Errorf("args %v: exit %d, want 2", tt.args, ExitCode(err))
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("args %v: message %q does not name %s", tt.args, err, tt.want)
		}
	}

	// --max without --all is rejected on the single-page path too.
	cmd := allCmd("http://unused.invalid")
	if err := cmd.Flags().Parse([]string{"--max", "10"}); err != nil {
		t.Fatal(err)
	}
	err := getRenderPage(cmd, "/things", url.Values{}, func(page api.Page[row]) {})
	if err == nil || ExitCode(err) != 2 {
		t.Errorf("--max without --all: err %v, want usage error", err)
	}
}

// keylessCmd is a bare command with a host but no api-key set, for exercising
// the anonymous-tier path where nothing resolves a key.
func keylessCmd(host string) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("json", false, "")
	cmd.Flags().String("host", host, "")
	cmd.Flags().String("api-key", "", "")
	return cmd
}

func TestClientFromNoKeySucceeds(t *testing.T) {
	// Isolate config: point UserConfigDir at an empty temp dir and clear the
	// env key, so nothing resolves a key.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LANTERN_API_KEY", "")
	t.Setenv("LANTERN_HOST", "")

	cmd := keylessCmd("http://unused.invalid")
	client, err := clientFrom(cmd)
	if err != nil {
		t.Fatalf("clientFrom errored with no key: %v", err)
	}
	if client == nil {
		t.Fatal("clientFrom returned nil client")
	}
	if client.Key != "" {
		t.Errorf("client key = %q, want empty", client.Key)
	}
}

func TestKeylessRequestSendsNoAuthHeader(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LANTERN_API_KEY", "")
	t.Setenv("LANTERN_HOST", "")

	var hadAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header["Authorization"]
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	cmd := keylessCmd(srv.URL)
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() {
		if err := getRender(cmd, "/lists", url.Values{}, func(api.Page[row]) {}); err != nil {
			t.Error(err)
		}
	})
	if hadAuth {
		t.Error("keyless request sent an Authorization header")
	}
}

func TestPositionalQuery(t *testing.T) {
	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{Use: "threads"}
		cmd.Flags().String("q", "", "")
		return cmd
	}

	// No args leaves q alone (whatever --q set, or nothing).
	cmd := newCmd()
	q := url.Values{}
	if err := positionalQuery(cmd, nil, q); err != nil || q.Get("q") != "" {
		t.Errorf("no args: err=%v q=%q", err, q.Get("q"))
	}

	// Multi-word positional query joins with spaces, like search.
	cmd = newCmd()
	q = url.Values{}
	if err := positionalQuery(cmd, []string{"index", "only", "scan"}, q); err != nil {
		t.Fatal(err)
	}
	if got := q.Get("q"); got != "index only scan" {
		t.Errorf("q = %q", got)
	}

	// Positional and --q together is a usage error, not a silent preference.
	cmd = newCmd()
	if err := cmd.Flags().Parse([]string{"--q", "vacuum"}); err != nil {
		t.Fatal(err)
	}
	err := positionalQuery(cmd, []string{"vacuum"}, url.Values{})
	if err == nil || ExitCode(err) != 2 {
		t.Errorf("positional + --q: err %v, want usage error", err)
	}
}

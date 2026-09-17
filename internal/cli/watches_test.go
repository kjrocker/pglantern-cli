package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
)

// recorded is what a test server saw: the one request the command made.
type recorded struct {
	method string
	path   string
	body   []byte
}

// watchServer records the request and replies with reply (status 200 unless
// the reply is empty, in which case it answers 204 like a real delete).
func watchServer(t *testing.T, got *recorded, status int, reply string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*got = recorded{method: r.Method, path: r.URL.Path, body: b}
		w.WriteHeader(status)
		if reply != "" {
			w.Write([]byte(reply))
		}
	}))
}

// runLantern drives the real command tree end to end against host, capturing
// both streams — watches split their output across them (tables on stdout, the
// delete receipt on stderr), so both have to be observed.
func runLantern(t *testing.T, host string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LANTERN_API_KEY", "")
	t.Setenv("LANTERN_HOST", "")

	full := append([]string{}, args...)
	full = append(full, "--host", host, "--api-key", "k", "--no-pager")

	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			root := NewRootCmd()
			root.SetArgs(full)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SilenceErrors = true
			err = root.Execute()
		})
	})
	return stdout, stderr, err
}

// unreachableServer fails the test if any request reaches it — the client-side
// validations must short-circuit before the round-trip.
func unreachableServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
}

const sampleWatch = `{"id":"11111111-2222-3333-4444-555555555555","type":"query",` +
	`"params":{"q":"io_uring","list":"pgsql-hackers"},"channel":"email","endpoint_id":null,` +
	`"cadence":"daily","active":true,"inserted_at":"2026-09-18T09:14:03Z",` +
	`"updated_at":"2026-09-18T09:14:03Z"}`

func TestWatchesCreatePostsBody(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusCreated, `{"data":`+sampleWatch+`}`)
	defer srv.Close()

	stdout, _, err := runLantern(t, srv.URL, "watches", "create",
		"--type", "query", "--q", "io_uring", "--list", "pgsql-hackers", "--cadence", "weekly")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/watches" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	var sent map[string]any
	if err := json.Unmarshal(got.body, &sent); err != nil {
		t.Fatalf("body %q: %v", got.body, err)
	}
	// channel is required server-side with no default, so it's always sent.
	want := map[string]any{
		"type":    "query",
		"channel": "email",
		"cadence": "weekly",
		"params":  map[string]any{"q": "io_uring", "list": "pgsql-hackers"},
	}
	if !reflect.DeepEqual(sent, want) {
		t.Errorf("body = %v, want %v", sent, want)
	}
	if !strings.Contains(stdout, "11111111-2222-3333-4444-555555555555") {
		t.Errorf("stdout does not render the created watch:\n%s", stdout)
	}
}

// A sender watch's id must go over the wire as a JSON number, not a string.
func TestWatchesCreateSenderIDIsAnInteger(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusCreated, `{"data":`+sampleWatch+`}`)
	defer srv.Close()

	if _, _, err := runLantern(t, srv.URL, "watches", "create",
		"--type", "sender", "--sender-id", "42"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got.body), `"sender_id":42`) {
		t.Errorf("body = %s, want sender_id as a number", got.body)
	}
}

// A thread watch takes the Message-Id as a table prints it; the <> a mail
// header wraps it in is stripped, like every other Message-Id the CLI takes.
func TestWatchesCreateStripsMessageIDBrackets(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusCreated, `{"data":`+sampleWatch+`}`)
	defer srv.Close()

	if _, _, err := runLantern(t, srv.URL, "watches", "create",
		"--type", "thread", "--message-id", "<a1@example.com>"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got.body), `"message_id":"a1@example.com"`) {
		t.Errorf("body = %s", got.body)
	}
}

// --channel webhook rides along with the endpoint id; cadence stays out when
// unset, since the server rejects unknown/empty top-level keys.
func TestWatchesCreateWebhookChannel(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusCreated, `{"data":`+sampleWatch+`}`)
	defer srv.Close()

	if _, _, err := runLantern(t, srv.URL, "watches", "create",
		"--type", "path", "--path", "src/backend/access/",
		"--channel", "webhook", "--endpoint", "e1"); err != nil {
		t.Fatal(err)
	}
	var sent map[string]any
	if err := json.Unmarshal(got.body, &sent); err != nil {
		t.Fatalf("body %q: %v", got.body, err)
	}
	want := map[string]any{
		"type":        "path",
		"channel":     "webhook",
		"endpoint_id": "e1",
		"params":      map[string]any{"path": "src/backend/access/"},
	}
	if !reflect.DeepEqual(sent, want) {
		t.Errorf("body = %v, want %v", sent, want)
	}
}

func TestWatchesPausePatchesActive(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusOK, `{"data":`+sampleWatch+`}`)
	defer srv.Close()

	if _, _, err := runLantern(t, srv.URL, "watches", "pause", "w1"); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPatch || got.path != "/api/v1/watches/w1" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if string(got.body) != `{"active":false}` {
		t.Errorf("body = %s", got.body)
	}
}

func TestWatchesResumePatchesActive(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusOK, `{"data":`+sampleWatch+`}`)
	defer srv.Close()

	if _, _, err := runLantern(t, srv.URL, "watches", "resume", "w1"); err != nil {
		t.Fatal(err)
	}
	if string(got.body) != `{"active":true}` {
		t.Errorf("body = %s", got.body)
	}
}

func TestWatchesCadencePatchesCadence(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusOK, `{"data":`+sampleWatch+`}`)
	defer srv.Close()

	if _, _, err := runLantern(t, srv.URL, "watches", "cadence", "w1", "weekly"); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPatch || got.path != "/api/v1/watches/w1" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if string(got.body) != `{"cadence":"weekly"}` {
		t.Errorf("body = %s", got.body)
	}
}

// Delete is bodiless both ways: no request body, no stdout. The receipt goes to
// stderr so a delete in a pipeline contributes nothing.
func TestWatchesDelete(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusNoContent, "")
	defer srv.Close()

	stdout, stderr, err := runLantern(t, srv.URL, "watches", "delete", "w1")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodDelete || got.path != "/api/v1/watches/w1" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "deleted w1") {
		t.Errorf("stderr = %q, want the delete receipt", stderr)
	}
}

func TestWatchesJSONPrintsRawBody(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusCreated, `{"data":`+sampleWatch+`}`)
	defer srv.Close()

	stdout, _, err := runLantern(t, srv.URL, "watches", "create",
		"--type", "guc", "--name", "work_mem", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("stdout is not the raw JSON body (%v):\n%s", err, stdout)
	}
	if _, ok := decoded["data"]; !ok {
		t.Errorf("stdout lost the data envelope:\n%s", stdout)
	}
}

func TestWatchesUsageErrors(t *testing.T) {
	srv := unreachableServer(t)
	defer srv.Close()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing --type", []string{"watches", "create", "--q", "io_uring"}, "--type"},
		{"missing subject flag", []string{"watches", "create", "--type", "query"}, "--q"},
		{"missing subject flag for guc", []string{"watches", "create", "--type", "guc"}, "--name"},
		{"unknown type", []string{"watches", "create", "--type", "banana"}, "must be one of"},
		{"bad cadence arg", []string{"watches", "cadence", "w1", "hourly"}, "daily"},
		{"cadence without an id", []string{"watches", "cadence", "w1"}, "cadence"},
		{"delete without an id", []string{"watches", "delete"}, "watch id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := runLantern(t, srv.URL, tt.args...)
			if err == nil {
				t.Fatalf("args %v accepted, want a usage error", tt.args)
			}
			if got := ExitCode(err); got != 2 {
				t.Errorf("exit %d, want 2 (%v)", got, err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("message %q does not mention %q", err, tt.want)
			}
		})
	}
}

func TestWatchesListRendersTable(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusOK, `{"data":[`+sampleWatch+`]}`)
	defer srv.Close()

	stdout, _, err := runLantern(t, srv.URL, "watches")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/watches" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	for _, want := range []string{"ID", "TYPE", "PARAMS", "DELIVERY", "CADENCE", "STATUS",
		"query", "list=pgsql-hackers q=io_uring", "email", "daily", "active"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("table is missing %q:\n%s", want, stdout)
		}
	}
}

func TestWatchesEndpointsRendersTable(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusOK,
		`{"data":[{"id":"e1","url":"https://hooks.example.com/x","active":false,`+
			`"disabled_at":"2026-09-01T00:00:00Z"}]}`)
	defer srv.Close()

	stdout, _, err := runLantern(t, srv.URL, "watches", "endpoints")
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/api/v1/webhook-endpoints" {
		t.Errorf("path = %s", got.path)
	}
	for _, want := range []string{"URL", "e1", "https://hooks.example.com/x", "disabled 2026-09-01"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("table is missing %q:\n%s", want, stdout)
		}
	}
}

// A read-only key is refused by the server; the exit code stays 3 and the CLI
// adds the one thing the server's message doesn't say — where to get a key that
// can write.
func TestReadOnlyKeyErrorCarriesHint(t *testing.T) {
	var got recorded
	srv := watchServer(t, &got, http.StatusForbidden,
		`{"error":{"code":"key_read_only","message":"This API key is read-only."}}`)
	defer srv.Close()

	_, _, err := runLantern(t, srv.URL, "watches", "delete", "w1")
	if err == nil {
		t.Fatal("read-only rejection did not surface as an error")
	}
	if code := ExitCode(err); code != 3 {
		t.Errorf("exit %d, want 3", code)
	}
	if hint := Hint(err); !strings.Contains(hint, "/users/api-keys") {
		t.Errorf("hint = %q, want the manage-key pointer", hint)
	}
	if Hint(&api.Error{Status: 404, Code: "not_found"}) != "" {
		t.Error("unrelated errors must not carry the manage-key hint")
	}
}

func TestWatchParamsCell(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"sorted keys", map[string]any{"q": "io_uring", "list": "hackers"}, "list=hackers q=io_uring"},
		{"whole numbers print as integers", map[string]any{"sender_id": float64(42)}, "sender_id=42"},
		{"empty params", nil, "-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := watchParamsCell(tt.params); got != tt.want {
				t.Errorf("watchParamsCell() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWatchDeliveryCell(t *testing.T) {
	endpoint := "e1"
	webhook := api.Watch{Channel: "webhook", EndpointID: &endpoint}
	if got := watchDeliveryCell(webhook); got != "webhook:e1" {
		t.Errorf("webhook delivery = %q", got)
	}
	if got := watchDeliveryCell(api.Watch{Channel: "email"}); got != "email" {
		t.Errorf("email delivery = %q", got)
	}
}

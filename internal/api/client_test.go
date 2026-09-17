package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDecodeError(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		code    string
		message string
		details string
	}{
		{
			name:    "server envelope",
			status:  404,
			body:    `{"error":{"code":"not_found","message":"No message with that id."}}`,
			code:    "not_found",
			message: "No message with that id.",
		},
		{
			name:    "envelope with details",
			status:  422,
			body:    `{"error":{"code":"invalid_params","message":"limit is out of range","details":{"limit":"max 100"}}}`,
			code:    "invalid_params",
			message: "limit is out of range",
			details: `{"limit":"max 100"}`,
		},
		{
			name:    "enum shape details",
			status:  422,
			body:    `{"error":{"code":"invalid_params","message":"Invalid value for enum","details":{"errors":[{"path":"/sort","reason":"Invalid value for enum"}]}}}`,
			code:    "invalid_params",
			message: "Invalid value for enum",
			details: `{"errors":[{"path":"/sort","reason":"Invalid value for enum"}]}`,
		},
		{
			name:   "non-envelope body",
			status: 502,
			body:   `<html>bad gateway</html>`,
		},
		{
			name:   "empty body",
			status: 500,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DecodeError(tt.status, []byte(tt.body))
			if err.Status != tt.status || err.Code != tt.code || err.Message != tt.message {
				t.Errorf("got %+v, want status=%d code=%q message=%q",
					err, tt.status, tt.code, tt.message)
			}
			if string(err.Details) != tt.details {
				t.Errorf("details = %q, want %q", err.Details, tt.details)
			}
		})
	}
}

func TestErrorString(t *testing.T) {
	withMessage := &Error{Status: 404, Code: "not_found", Message: "No such thing."}
	if got := withMessage.Error(); got != "No such thing." {
		t.Errorf("got %q", got)
	}
	bare := &Error{Status: 502}
	if got := bare.Error(); got != "HTTP 502" {
		t.Errorf("got %q", got)
	}

	// Enum shape (schema-validated): the terse message doesn't name the param,
	// so Error() appends it from details.errors[].path.
	enum := DecodeError(422, []byte(`{"error":{"code":"invalid_params","message":"Invalid value for enum","details":{"errors":[{"path":"/sort","reason":"Invalid value for enum"}]}}}`))
	if got := enum.Error(); got != "Invalid value for enum (parameter: sort)" {
		t.Errorf("enum shape: got %q", got)
	}

	// Domain shape (FallbackController): the message already quotes the param,
	// so Error() must not duplicate it with a "(parameter: ...)" suffix.
	domain := DecodeError(422, []byte(`{"error":{"code":"invalid_params","message":"The 'limit' parameter must be an integer between 1 and 100.","details":{"param":"limit"}}}`))
	got := domain.Error()
	if strings.Contains(got, "(parameter:") {
		t.Errorf("domain shape: got %q, want no duplicated param suffix", got)
	}
	if got != "The 'limit' parameter must be an integer between 1 and 100." {
		t.Errorf("domain shape: got %q", got)
	}
}

func TestClientGet(t *testing.T) {
	var gotAuth, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	body, err := New(srv.URL+"/", "sekrit").Get("/messages", url.Values{"limit": {"3"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"data":[]}` {
		t.Errorf("body = %q", body)
	}
	if gotAuth != "Bearer sekrit" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotPath != "/api/v1/messages" {
		t.Errorf("path = %q (trailing host slash not trimmed?)", gotPath)
	}
	if gotQuery != "limit=3" {
		t.Errorf("query = %q", gotQuery)
	}
}

func TestClientGetNoKeyOmitsAuthHeader(t *testing.T) {
	// A client built with an empty key must send NO Authorization header, so the
	// server serves the anonymous per-IP tier. An empty `Bearer ` would instead
	// be read as an invalid key and 401.
	var gotAuth string
	var hadAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header["Authorization"]
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "").Get("/lists", nil); err != nil {
		t.Fatal(err)
	}
	if hadAuth {
		t.Errorf("Authorization header present with empty key: %q", gotAuth)
	}
}

func TestClientGetExactIDs(t *testing.T) {
	// Exact-ids mode goes over the wire as repeated `ids[]=` (percent-encoded),
	// which is the only form Plug keeps as a list.
	var gotIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIDs = r.URL.Query()["ids[]"]
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, "sekrit").Get("/messages", url.Values{"ids[]": {"a@host", "b@host"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(gotIDs, ",") != "a@host,b@host" {
		t.Errorf("ids[] = %v, want [a@host b@host]", gotIDs)
	}
}

func TestClientPost(t *testing.T) {
	var gotMethod, gotPath, gotType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"w1"}}`))
	}))
	defer srv.Close()

	payload := map[string]any{"type": "query", "params": map[string]any{"q": "io_uring"}}
	body, err := New(srv.URL, "sekrit").Post("/watches", payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"data":{"id":"w1"}}` {
		t.Errorf("body = %q", body)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/watches" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotType != "application/json" {
		t.Errorf("content-type = %q", gotType)
	}
	if gotBody != `{"params":{"q":"io_uring"},"type":"query"}` {
		t.Errorf("request body = %q", gotBody)
	}
}

func TestClientPatch(t *testing.T) {
	var gotMethod, gotPath, gotType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Write([]byte(`{"data":{"id":"w1","active":false}}`))
	}))
	defer srv.Close()

	body, err := New(srv.URL, "sekrit").Patch("/watches/w1", map[string]any{"active": false})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"data":{"id":"w1","active":false}}` {
		t.Errorf("body = %q", body)
	}
	if gotMethod != http.MethodPatch || gotPath != "/api/v1/watches/w1" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotType != "application/json" {
		t.Errorf("content-type = %q", gotType)
	}
	if gotBody != `{"active":false}` {
		t.Errorf("request body = %q", gotBody)
	}
}

// A delete answers 204 with no body — the client must report that as
// (nil, nil), not an empty body a caller would try to decode.
func TestClientDeleteNoContent(t *testing.T) {
	var gotMethod, gotPath, gotType string
	var hadBody bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		hadBody = len(b) > 0
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	body, err := New(srv.URL, "sekrit").Delete("/watches/w1")
	if err != nil {
		t.Fatalf("delete errored: %v", err)
	}
	if body != nil {
		t.Errorf("body = %q, want nil on 204", body)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/v1/watches/w1" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if hadBody {
		t.Error("delete sent a request body")
	}
	if gotType != "" {
		t.Errorf("content-type = %q, want none for a bodiless request", gotType)
	}
}

func TestClientWriteErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"code":"key_read_only","message":"This API key is read-only."}}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, "ro").Post("/watches", map[string]any{"type": "query"})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *Error, got %T: %v", err, err)
	}
	if apiErr.Status != 403 || apiErr.Code != "key_read_only" {
		t.Errorf("got %+v", apiErr)
	}
}

func TestClientGetErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":"unauthorized","message":"A valid API key is required."}}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, "bad").Get("/lists", nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *Error, got %T: %v", err, err)
	}
	if apiErr.Status != 401 || apiErr.Code != "unauthorized" {
		t.Errorf("got %+v", apiErr)
	}
}

package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDecodeError(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		code    string
		message string
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

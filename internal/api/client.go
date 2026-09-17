// Package api is a thin HTTP client for the pgLantern JSON API under /api/v1.
// It sets the auth header, issues the request, and decodes the server's error
// envelope; response bodies are returned as raw bytes so --json output is
// exactly what the server sent.
//
// Reads (GET) are the bulk of the surface; the watch endpoints add writes
// (POST/PATCH/DELETE), which the server only accepts from a key minted with
// manage access — a read-only key gets a 403 `key_read_only`.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiPrefix = "/api/v1"

// Error is the server's error envelope ({"error":{code,message}}) plus the
// HTTP status, or a bare status when the body isn't the envelope.
type Error struct {
	Status  int
	Code    string
	Message string
	Details json.RawMessage // raw error.details, nil when absent
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", e.Status)
	}
	// Append the offending parameter(s) only when details name one and the
	// message doesn't already carry it — this enriches the terse schema-shaped
	// messages ("Invalid value for enum") without duplicating the domain ones
	// that already quote the param.
	if p := e.detailParams(); p != "" && !strings.Contains(msg, p) {
		msg += " (parameter: " + p + ")"
	}
	return msg
}

// detailParams decodes Details against the two settled error.details shapes and
// returns the offending parameter name(s), or "" for nil/unrecognized shapes so
// unknown future shapes degrade to today's behavior.
func (e *Error) detailParams() string {
	if len(e.Details) == 0 {
		return ""
	}
	// Shape 1 (FallbackController): {"param":"limit"}.
	var shape1 struct {
		Param string `json:"param"`
	}
	if json.Unmarshal(e.Details, &shape1) == nil && shape1.Param != "" {
		return shape1.Param
	}
	// Shape 2 (OpenApiErrorRenderer): {"errors":[{"path":"/sort","reason":...}]}.
	var shape2 struct {
		Errors []struct {
			Path string `json:"path"`
		} `json:"errors"`
	}
	if json.Unmarshal(e.Details, &shape2) == nil && len(shape2.Errors) > 0 {
		var params []string
		for _, err := range shape2.Errors {
			if p := strings.TrimPrefix(err.Path, "/"); p != "" {
				params = append(params, p)
			}
		}
		return strings.Join(params, ", ")
	}
	return ""
}

// DecodeError parses a non-2xx body into an Error, falling back to the bare
// status when the body isn't the expected envelope.
func DecodeError(status int, body []byte) *Error {
	var env struct {
		Error struct {
			Code    string          `json:"code"`
			Message string          `json:"message"`
			Details json.RawMessage `json:"details"`
		} `json:"error"`
	}
	e := &Error{Status: status}
	if json.Unmarshal(body, &env) == nil {
		e.Code = env.Error.Code
		e.Message = env.Error.Message
		e.Details = env.Error.Details
	}
	return e
}

type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
}

func New(baseURL, key string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Key:     key,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Get requests /api/v1/<path> and returns the raw response body. Non-2xx
// responses come back as *Error. A client with an empty Key omits the
// Authorization header entirely, so the server serves the request on its
// anonymous per-IP tier — sending an empty `Bearer ` token would instead be
// rejected as an invalid key.
func (c *Client) Get(path string, query url.Values) ([]byte, error) {
	return c.do(http.MethodGet, path, query, nil)
}

// Post sends body as JSON to /api/v1/<path> and returns the raw response body
// (201 {"data":…} on the watch endpoints).
func (c *Client) Post(path string, body any) ([]byte, error) {
	return c.do(http.MethodPost, path, nil, body)
}

// Patch sends a partial update as JSON and returns the raw response body.
func (c *Client) Patch(path string, body any) ([]byte, error) {
	return c.do(http.MethodPatch, path, nil, body)
}

// Delete removes the resource at path. The server answers 204 with no body, so
// a successful delete returns (nil, nil).
func (c *Client) Delete(path string) ([]byte, error) {
	return c.do(http.MethodDelete, path, nil, nil)
}

// do is the one round-trip every verb goes through: build the URL, attach the
// key, JSON-encode a non-nil body, and split 2xx bodies from the error
// envelope. A 204 has no body to return, so it comes back as (nil, nil).
func (c *Client) do(method, path string, query url.Values, body any) ([]byte, error) {
	u := c.BaseURL + apiPrefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, u, payload)
	if err != nil {
		return nil, err
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if resp.StatusCode == http.StatusNoContent {
			return nil, nil
		}
		return respBody, nil
	}
	return nil, DecodeError(resp.StatusCode, respBody)
}

// Package api is a thin HTTP client for the Horton JSON API under /api/v1.
// It sets the auth header, issues GETs, and decodes the server's error
// envelope; response bodies are returned as raw bytes so --json output is
// exactly what the server sent.
package api

import (
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
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

// DecodeError parses a non-2xx body into an Error, falling back to the bare
// status when the body isn't the expected envelope.
func DecodeError(status int, body []byte) *Error {
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	e := &Error{Status: status}
	if json.Unmarshal(body, &env) == nil {
		e.Code = env.Error.Code
		e.Message = env.Error.Message
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
// responses come back as *Error.
func (c *Client) Get(path string, query url.Values) ([]byte, error) {
	u := c.BaseURL + apiPrefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, nil
	}
	return nil, DecodeError(resp.StatusCode, body)
}

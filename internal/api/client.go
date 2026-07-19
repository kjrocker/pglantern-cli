// Package api is a thin HTTP client for the pgLantern JSON API under /api/v1.
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

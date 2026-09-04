package cli

import (
	"errors"
	"fmt"
	"net/http"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
)

// usageError marks an error as the caller's misuse of the CLI — bad flags,
// missing arguments, conflicting options — so main can exit 2, distinct from
// server-side and transport failures.
type usageError struct{ err error }

func (u *usageError) Error() string { return u.err.Error() }
func (u *usageError) Unwrap() error { return u.err }

func usagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

// ExitCode maps the error Execute returned to the process exit code: 0 ok,
// 2 usage, 3 auth (401/403), 4 not found (404), 1 everything else.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var usage *usageError
	if errors.As(err, &usage) {
		return 2
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return 3
		case http.StatusNotFound:
			return 4
		}
	}
	return 1
}

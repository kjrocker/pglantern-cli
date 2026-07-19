package cli

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"codeberg.org/kehvyn/pglantern-cli/internal/api"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"usage", usagef("bad flags"), 2},
		{"wrapped usage", fmt.Errorf("context: %w", usagef("bad")), 2},
		{"unauthorized", &api.Error{Status: 401}, 3},
		{"forbidden", &api.Error{Status: 403}, 3},
		{"not found", &api.Error{Status: 404}, 4},
		{"wrapped api error", fmt.Errorf("key rejected: %w", &api.Error{Status: 401}), 3},
		{"unprocessable", &api.Error{Status: 422}, 1},
		{"server error", &api.Error{Status: 500}, 1},
		{"generic", errors.New("dial tcp: connection refused"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// Flag-parse failures surface from Execute as usage errors (exit 2) — the
// SetFlagErrorFunc wrapping, end to end, without any HTTP.
func TestFlagErrorsAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"lists", "--bogus"},
		{"threads", "--sort", "banana"},
		{"messages", "--limit", "abc"},
	} {
		root := NewRootCmd()
		root.SetArgs(args)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SilenceErrors = true
		err := root.Execute()
		if err == nil {
			t.Errorf("args %v: no error", args)
			continue
		}
		if got := ExitCode(err); got != 2 {
			t.Errorf("args %v: exit %d, want 2 (%v)", args, got, err)
		}
	}
}

// Positional-args failures (both cobra.NoArgs and requireArg) are usage errors.
func TestArgsErrorsAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"lists", "extra"},
		{"senders", "get"},
	} {
		root := NewRootCmd()
		root.SetArgs(args)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SilenceErrors = true
		err := root.Execute()
		if err == nil {
			t.Errorf("args %v: no error", args)
			continue
		}
		if got := ExitCode(err); got != 2 {
			t.Errorf("args %v: exit %d, want 2 (%v)", args, got, err)
		}
	}
}

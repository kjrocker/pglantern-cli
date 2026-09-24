package cli

import (
	"testing"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
)

func TestLastActivity(t *testing.T) {
	at := "2026-05-31T23:38:02Z"
	cases := []struct {
		name  string
		stats *api.ListStats
		want  string
	}{
		{"no stats", nil, "-"},
		{"empty list", &api.ListStats{MessageCount: 0}, "-"},
		{"dated", &api.ListStats{MessageCount: 3, LastMessageAt: &at}, "2026-05-31"},
	}
	for _, c := range cases {
		if got := lastActivity(c.stats); got != c.want {
			t.Errorf("%s: lastActivity = %q, want %q", c.name, got, c.want)
		}
	}
}

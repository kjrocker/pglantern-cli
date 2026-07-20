package cli

import (
	"testing"

	"codeberg.org/kehvyn/pglantern-cli/internal/api"
)

func TestSenderDisplay(t *testing.T) {
	tests := []struct {
		name string
		msg  api.MessageSummary
		want string
	}{
		{
			name: "preloaded sender assembles Name <email>",
			msg: api.MessageSummary{
				FromRaw: "tgl@sss.pgh.pa.us",
				Sender:  &api.Sender{Email: "tgl@sss.pgh.pa.us", DisplayName: "Tom Lane"},
			},
			want: "Tom Lane <tgl@sss.pgh.pa.us>",
		},
		{
			name: "preloaded sender with no display name degrades to email",
			msg: api.MessageSummary{
				FromRaw: "Tom Lane <tgl@sss.pgh.pa.us>",
				Sender:  &api.Sender{Email: "tgl@sss.pgh.pa.us"},
			},
			want: "tgl@sss.pgh.pa.us",
		},
		{
			name: "no preloaded sender falls back to from_raw",
			msg: api.MessageSummary{
				FromRaw: "Tom Lane <tgl@sss.pgh.pa.us>",
			},
			want: "Tom Lane <tgl@sss.pgh.pa.us>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := senderDisplay(tt.msg); got != tt.want {
				t.Errorf("senderDisplay() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Regression for the swapped COMMITTED AT / AUTHOR headers: every commit table
// uses commitHeader, and its column order must match what commitRows emits.
func TestCommitHeaderMatchesRows(t *testing.T) {
	rows := commitRows([]api.CommitSummary{{
		SHA:         "abc123",
		AuthorName:  "Tom Lane",
		AuthorEmail: "tgl@sss.pgh.pa.us",
		CommittedAt: "2026-01-02T03:04:05Z",
		Subject:     "Fix planner",
	}})
	if len(rows) != 1 || len(rows[0]) != len(commitHeader) {
		t.Fatalf("rows %v vs header %v: length mismatch", rows, commitHeader)
	}
	want := map[string]string{
		"SHA":          "abc123",
		"AUTHOR":       "Tom Lane <tgl@sss.pgh.pa.us>",
		"COMMITTED AT": "2026-01-02T03:04:05Z",
		"SUBJECT":      "Fix planner",
	}
	for i, h := range commitHeader {
		if rows[0][i] != want[h] {
			t.Errorf("column %s = %q, want %q", h, rows[0][i], want[h])
		}
	}
}

// `commits --sort authored` swaps the date column, so its header and rows have
// to stay paired too — and must show authored_at, not committed_at.
func TestAuthoredCommitHeaderMatchesRows(t *testing.T) {
	rows := authoredCommitRows([]api.CommitSummary{{
		SHA:         "abc123",
		AuthorName:  "Tom Lane",
		AuthorEmail: "tgl@sss.pgh.pa.us",
		CommittedAt: "2026-01-02T03:04:05Z",
		AuthoredAt:  "2025-12-24T09:30:00Z",
		Subject:     "Fix planner",
	}})
	if len(rows) != 1 || len(rows[0]) != len(authoredCommitHeader) {
		t.Fatalf("rows %v vs header %v: length mismatch", rows, authoredCommitHeader)
	}
	want := map[string]string{
		"SHA":         "abc123",
		"AUTHOR":      "Tom Lane <tgl@sss.pgh.pa.us>",
		"AUTHORED AT": "2025-12-24T09:30:00Z",
		"SUBJECT":     "Fix planner",
	}
	for i, h := range authoredCommitHeader {
		if rows[0][i] != want[h] {
			t.Errorf("column %s = %q, want %q", h, rows[0][i], want[h])
		}
	}
}

func TestNormalizeMessageID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "raw message-id passes through unchanged",
			in:   "2294297.1780270682@sss.pgh.pa.us",
			want: "2294297.1780270682@sss.pgh.pa.us",
		},
		{
			name: "angle brackets from a mail header are stripped",
			in:   "<2294297.1780270682@sss.pgh.pa.us>",
			want: "2294297.1780270682@sss.pgh.pa.us",
		},
		{
			name: "gmail-style id with = and + is left intact",
			in:   "CAFiTN-sF_J8NB3xjie7g=2-R5v9aLqEE5jrtF2dMmwPngd9RBg@mail.gmail.com",
			want: "CAFiTN-sF_J8NB3xjie7g=2-R5v9aLqEE5jrtF2dMmwPngd9RBg@mail.gmail.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMessageID(tt.in); got != tt.want {
				t.Errorf("normalizeMessageID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMessagePath(t *testing.T) {
	tests := []struct {
		name   string
		arg    string
		suffix string
		want   string
	}{
		{
			name: "plain id percent-encodes into one segment",
			arg:  "<2294297.1780270682@sss.pgh.pa.us>",
			want: "/messages/2294297.1780270682@sss.pgh.pa.us",
		},
		{
			name: "a slash in the id escapes to %2F, keeping one segment",
			arg:  "20040101/some.id@example.com",
			want: "/messages/20040101%2Fsome.id@example.com",
		},
		{
			name:   "suffix is appended after the encoded segment",
			arg:    "a1@example.com",
			suffix: "/thread",
			want:   "/messages/a1@example.com/thread",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := messagePath(tt.arg, tt.suffix); got != tt.want {
				t.Errorf("messagePath(%q, %q) = %q, want %q", tt.arg, tt.suffix, got, tt.want)
			}
		})
	}
}

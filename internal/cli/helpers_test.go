package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRequireArg(t *testing.T) {
	newCmd := func() *cobra.Command {
		return &cobra.Command{Use: "patch <id>", Args: requireArg("an attachment id")}
	}

	// Missing and extra arguments both fail with a message that names the
	// argument and shows the usage line, not cobra's bare "accepts 1 arg(s)".
	for _, args := range [][]string{{}, {"1", "2"}} {
		err := newCmd().Args(newCmd(), args)
		if err == nil {
			t.Fatalf("args %v accepted, want error", args)
		}
		if !strings.Contains(err.Error(), "an attachment id") {
			t.Errorf("args %v: message %q does not name the argument", args, err)
		}
		if !strings.Contains(err.Error(), "patch <id>") {
			t.Errorf("args %v: message %q omits the usage line", args, err)
		}
	}

	if err := newCmd().Args(newCmd(), []string{"1"}); err != nil {
		t.Errorf("exactly one argument rejected: %v", err)
	}
}

func TestRequireArgs(t *testing.T) {
	cmd := &cobra.Command{Use: "search <query>...", Args: requireArgs("a search query")}

	if err := cmd.Args(cmd, nil); err == nil {
		t.Error("no arguments accepted, want error")
	} else if !strings.Contains(err.Error(), "a search query") {
		t.Errorf("message %q does not name the argument", err)
	}

	// One or more arguments are both fine for a multi-word query.
	for _, args := range [][]string{{"vacuum"}, {"index", "only", "scan"}} {
		if err := cmd.Args(cmd, args); err != nil {
			t.Errorf("args %v rejected: %v", args, err)
		}
	}
}

func TestCollectQuery(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Int("limit", 0, "")
	cmd.Flags().String("changed-since", "", "")
	cmd.Flags().Bool("committed", false, "")
	cmd.Flags().String("untouched", "", "")
	addEnumFlag(cmd, "dir", "sort direction", "asc", "desc")

	if err := cmd.Flags().Parse([]string{
		"--limit", "5", "--changed-since", "16", "--committed", "--dir", "asc",
	}); err != nil {
		t.Fatal(err)
	}

	q := collectQuery(cmd, "limit", "changed-since", "committed", "dir", "untouched")
	want := map[string]string{
		"limit":         "5",
		"changed_since": "16", // dash maps to underscore
		"committed":     "true",
		"dir":           "asc",
	}
	if len(q) != len(want) {
		t.Errorf("got %d params (%v), want %d — unset flags must not be sent", len(q), q, len(want))
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestEnumFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addEnumFlag(cmd, "dir", "sort direction", "asc", "desc")

	if err := cmd.Flags().Parse([]string{"--dir", "desc"}); err != nil {
		t.Fatalf("valid value rejected: %v", err)
	}
	if got := cmd.Flags().Lookup("dir").Value.String(); got != "desc" {
		t.Errorf("value = %q", got)
	}

	cmd2 := &cobra.Command{Use: "test"}
	addEnumFlag(cmd2, "dir", "sort direction", "asc", "desc")
	if err := cmd2.Flags().Parse([]string{"--dir", "sideways"}); err == nil {
		t.Error("invalid enum value accepted")
	}
}

func TestSearchSortEnum(t *testing.T) {
	// A known-bad --sort fails at parse time, before any HTTP call.
	if err := newSearchCmd().Flags().Parse([]string{"--sort", "banana"}); err == nil {
		t.Error("invalid --sort accepted")
	}
	// A valid value parses cleanly.
	if err := newSearchCmd().Flags().Parse([]string{"--sort", "sent_at"}); err != nil {
		t.Errorf("valid --sort rejected: %v", err)
	}
}

func TestThreadsSortEnum(t *testing.T) {
	// A known-bad --sort fails at parse time, before any HTTP call.
	if err := newThreadsCmd().Flags().Parse([]string{"--sort", "banana"}); err == nil {
		t.Error("invalid --sort accepted")
	}
	if err := newThreadsCmd().Flags().Parse([]string{"--dir", "sideways"}); err == nil {
		t.Error("invalid --dir accepted")
	}
	// Valid values parse cleanly.
	if err := newThreadsCmd().Flags().Parse([]string{"--sort", "messages", "--dir", "desc"}); err != nil {
		t.Errorf("valid sort flags rejected: %v", err)
	}
}

// idCmd is a bare command carrying the flags exact-ids mode interacts with.
func idCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Int("limit", 0, "")
	cmd.Flags().String("after", "", "")
	cmd.Flags().String("before", "", "")
	cmd.Flags().String("sort", "", "")
	cmd.Flags().String("list", "", "")
	addEnumFlag(cmd, "dir", "sort direction", "asc", "desc")
	addIDFlag(cmd, "an id")
	return cmd
}

func TestCollectIDs(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  []string
	}{
		{name: "no flag", want: nil},
		{name: "repeated", args: []string{"--id", "a", "--id", "b"}, want: []string{"a", "b"}},
		{
			name:  "stdin sentinel",
			args:  []string{"--id", "-"},
			stdin: "a\nb\n",
			want:  []string{"a", "b"},
		},
		{
			// Explicit ids and piped ones merge, in flag order.
			name:  "mixed explicit and stdin",
			args:  []string{"--id", "pinned", "--id", "-"},
			stdin: "a\nb\n",
			want:  []string{"pinned", "a", "b"},
		},
		{
			name:  "blanks trimmed and dropped",
			args:  []string{"--id", "  a  ", "--id", "   "},
			stdin: "",
			want:  []string{"a"},
		},
		{
			name:  "blank lines dropped",
			args:  []string{"--id", "-"},
			stdin: "a\n\n  \nb\n",
			want:  []string{"a", "b"},
		},
		{
			name:  "de-dupes keeping first-seen order",
			args:  []string{"--id", "b", "--id", "a", "--id", "b"},
			stdin: "a\nc\n",
			want:  []string{"b", "a"},
		},
		{
			// Stdin is consumed once, so a second `-` adds nothing.
			name:  "repeated sentinel reads stdin once",
			args:  []string{"--id", "-", "--id", "-"},
			stdin: "a\nb\n",
			want:  []string{"a", "b"},
		},
		{
			// Commas are never split — the server treats "a,b" as one literal id.
			name: "no comma splitting",
			args: []string{"--id", "a,b"},
			want: []string{"a,b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := idCmd()
			if err := cmd.Flags().Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			got, err := collectIDs(cmd, strings.NewReader(tt.stdin))
			if err != nil {
				t.Fatalf("collectIDs: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestCollectIDsMixedDedupe(t *testing.T) {
	// A piped id that duplicates an explicit one is dropped, not appended again.
	cmd := idCmd()
	if err := cmd.Flags().Parse([]string{"--id", "a", "--id", "-"}); err != nil {
		t.Fatal(err)
	}
	got, err := collectIDs(cmd, strings.NewReader("a\nb\n"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "a,b" {
		t.Errorf("got %v, want [a b]", got)
	}
}

func TestRejectPaginationWithIDs(t *testing.T) {
	// --id plus any pagination flag fails locally, before any HTTP call.
	for _, args := range [][]string{
		{"--id", "a", "--limit", "5"},
		{"--id", "a", "--after", "cursor"},
		{"--id", "a", "--before", "cursor"},
	} {
		cmd := idCmd()
		if err := cmd.Flags().Parse(args); err != nil {
			t.Fatal(err)
		}
		err := rejectPaginationWithIDs(cmd)
		if err == nil {
			t.Errorf("args %v accepted, want error", args)
			continue
		}
		// The message names the whole rule, not just the flag that tripped it.
		for _, want := range []string{"--limit", "--after", "--before"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("args %v: message %q omits %s", args, err, want)
			}
		}
	}

	// Sorts and filters stay legal in exact mode; pagination without --id is
	// just normal paged mode.
	for _, args := range [][]string{
		{"--id", "a", "--sort", "messages", "--dir", "desc"},
		{"--id", "a", "--list", "pgsql-hackers"},
		{"--limit", "5", "--after", "cursor"},
	} {
		cmd := idCmd()
		if err := cmd.Flags().Parse(args); err != nil {
			t.Fatal(err)
		}
		if err := rejectPaginationWithIDs(cmd); err != nil {
			t.Errorf("args %v rejected: %v", args, err)
		}
	}
}

func TestIDsQueryEncoding(t *testing.T) {
	cmd := idCmd()
	if err := cmd.Flags().Parse([]string{"--id", "<a@host>", "--id", "b@host"}); err != nil {
		t.Fatal(err)
	}
	ids, err := collectIDs(cmd, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	q := collectQuery(cmd)
	for _, id := range ids {
		q.Add("ids[]", normalizeMessageID(id))
	}

	// Bracketed key (bare repeated `ids=` collapses under Plug) and one entry
	// per id — not pflag's "[a b]" slice literal. Message-Id brackets stripped.
	want := "ids%5B%5D=a%40host&ids%5B%5D=b%40host"
	if got := q.Encode(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
}

func TestIntFlagRejectsGarbage(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Int("limit", 0, "")
	if err := cmd.Flags().Parse([]string{"--limit", "abc"}); err == nil {
		t.Error("non-integer --limit accepted")
	}
}

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

func TestIntFlagRejectsGarbage(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Int("limit", 0, "")
	if err := cmd.Flags().Parse([]string{"--limit", "abc"}); err == nil {
		t.Error("non-integer --limit accepted")
	}
}

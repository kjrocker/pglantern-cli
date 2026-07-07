package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

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

func TestIntFlagRejectsGarbage(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Int("limit", 0, "")
	if err := cmd.Flags().Parse([]string{"--limit", "abc"}); err == nil {
		t.Error("non-integer --limit accepted")
	}
}

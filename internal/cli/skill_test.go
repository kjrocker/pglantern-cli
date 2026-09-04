package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runSkill executes the skill command with args, returning what it wrote to
// stdout. The command resolves os.Stdout at call time (the pager relies on the
// same property), so swapping the variable captures it.
func runSkill(t *testing.T, args ...string) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	cmd := newSkillCmd()
	cmd.SetArgs(args)
	cmd.SetOut(os.Stderr)
	runErr := cmd.Execute()

	w.Close()
	os.Stdout = orig
	out, err := readAllString(r)
	if err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("skill %v: %v", args, runErr)
	}
	return out
}

func TestSkillWritesToStdout(t *testing.T) {
	got := runSkill(t)
	if got != skillMD {
		t.Errorf("stdout = %d bytes, want the embedded skill (%d bytes)", len(got), len(skillMD))
	}
	if !strings.Contains(got, "lantern") {
		t.Error("stdout does not look like the skill document")
	}
}

func TestSkillOutputToFile(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "nested", "custom.md")
	if got := runSkill(t, "-o", dest); got != "" {
		t.Errorf("stdout = %q, want empty when -o is set", got)
	}

	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != skillMD {
		t.Error("file contents do not match the embedded skill")
	}
}

func TestSkillOutputToDirectoryWritesSkillMD(t *testing.T) {
	dir := t.TempDir()
	runSkill(t, "--output", dir)

	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md inside the directory: %v", err)
	}
}

func TestSkillOutputOverwrites(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(dest, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSkill(t, "-o", dest)

	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != skillMD {
		t.Error("-o did not overwrite the existing file")
	}
}

func TestSkillRejectsPositionalArgs(t *testing.T) {
	cmd := newSkillCmd()
	cmd.SetArgs([]string{"some/path"})
	cmd.SetOut(os.Stderr)
	cmd.SetErr(os.Stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected an error for a positional argument, got nil")
	}
}

func readAllString(r *os.File) (string, error) {
	b, err := io.ReadAll(r)
	return string(b), err
}

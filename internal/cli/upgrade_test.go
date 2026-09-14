package cli

import (
	"io"
	"testing"
)

// These cases all fail before any network call, so they need no server.
func TestUpgradeUsageErrors(t *testing.T) {
	cases := []struct {
		name    string
		version string
		args    []string
	}{
		{name: "dev build without --force", version: "dev", args: nil},
		{name: "snapshot build without --force", version: "0.9.0-SNAPSHOT-abc123", args: nil},
		{name: "malformed --to", version: "0.9.0", args: []string{"--to", "latest"}},
		{name: "--check with --to", version: "0.9.0", args: []string{"--check", "--to", "v0.9.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := version
			version = tc.version
			t.Cleanup(func() { version = orig })

			cmd := newUpgradeCmd()
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()
			if code := ExitCode(err); code != 2 {
				t.Fatalf("exit code = %d (err %v), want 2", code, err)
			}
		})
	}
}

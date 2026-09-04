package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// skillMD is the agent skill that teaches an assistant to drive this CLI. It's
// embedded so a consumer of the released binary can drop it into their agent's
// skills directory without cloning the repo.
//
//go:embed skill.md
var skillMD string

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Print the query-lantern agent skill",
		Long: "Print the bundled query-lantern agent skill, which teaches an assistant to\n" +
			"drive this CLI. Writes to stdout by default, so it composes with a redirect\n" +
			"or a pipe. Pass -o to write it to a file instead; if -o names an existing\n" +
			"directory, SKILL.md is written inside it. Parent directories are created as\n" +
			"needed, and -o overwrites an existing file.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dest, _ := cmd.Flags().GetString("output")
			if dest == "" {
				_, err := fmt.Fprint(os.Stdout, skillMD)
				return err
			}

			if info, err := os.Stat(dest); err == nil && info.IsDir() {
				dest = filepath.Join(dest, "SKILL.md")
			}

			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, []byte(skillMD), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Wrote query-lantern skill to %s\n", dest)
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "write the skill to this path instead of stdout")
	return cmd
}

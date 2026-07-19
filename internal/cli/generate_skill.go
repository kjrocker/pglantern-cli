package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// skillMD is the agent skill that teaches an assistant to drive this CLI. It's
// embedded so a consumer of the released binary can drop it into their agent's
// skills directory without cloning the repo.
//
//go:embed skill.md
var skillMD string

func newGenerateSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-skill <location>",
		Short: "Write the query-lantern agent SKILL.md to a location",
		Long: "Write the bundled query-lantern agent skill to disk so an assistant can\n" +
			"learn to drive this CLI. <location> may be a directory (SKILL.md is written\n" +
			"inside it) or a path ending in .md (written verbatim). Parent directories\n" +
			"are created as needed.",
		Args: requireArg("a target location"),
		RunE: func(cmd *cobra.Command, args []string) error {
			dest := args[0]
			if !strings.HasSuffix(dest, ".md") {
				dest = filepath.Join(dest, "SKILL.md")
			}

			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}

			force, _ := cmd.Flags().GetBool("force")
			if !force {
				if _, err := os.Stat(dest); err == nil {
					return fmt.Errorf("%s already exists; pass --force to overwrite", dest)
				}
			}

			if err := os.WriteFile(dest, []byte(skillMD), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Wrote query-lantern skill to %s\n", dest)
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "overwrite the destination if it already exists")
	return cmd
}

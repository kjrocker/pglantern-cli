package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/output"
	"git.kehvyn.dev/kevin/pglantern-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade lantern to the latest release",
		Long: "Replace this lantern binary with the latest GitHub release, or the one named\n" +
			"by --to, after verifying it against the release's checksums.txt. The binary\n" +
			"is replaced in place, so an install in a root-owned directory needs sudo.\n" +
			"--check reports whether an upgrade is available without changing anything.",
		Example: "  lantern upgrade\n" +
			"  lantern upgrade --check\n" +
			"  lantern upgrade --to v0.9.0",
		Args: cobra.NoArgs,
		RunE: runUpgrade,
	}
	cmd.Flags().Bool("check", false, "report the installed and latest versions without upgrading")
	cmd.Flags().String("to", "", "install this release instead of the latest (e.g. v0.9.0)")
	cmd.Flags().Bool("force", false, "reinstall even when up to date, or over a dev build")
	return cmd
}

type upgradeCheck struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
}

type upgradeResult struct {
	Previous string `json:"previous"`
	Current  string `json:"current"`
	Path     string `json:"path"`
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	check, _ := cmd.Flags().GetBool("check")
	target, _ := cmd.Flags().GetString("to")
	force, _ := cmd.Flags().GetBool("force")
	asJSON, _ := cmd.Flags().GetBool("json")

	if check && target != "" {
		return usagef("--check and --to cannot be combined")
	}
	if target != "" {
		target = "v" + strings.TrimPrefix(target, "v")
		if _, err := selfupdate.Compare(target, target); err != nil {
			return usagef("--to: %v", err)
		}
	}
	// Source builds (and goreleaser snapshots) carry no comparable version.
	_, verErr := selfupdate.Compare(version, version)
	dev := verErr != nil
	if dev && !check && !force {
		return usagef("this is a dev build of lantern (version %s); rebuild from source, or pass --force to install a release over it", version)
	}

	u := selfupdate.New(version)
	latest := target == ""
	if latest {
		tag, err := u.Latest(cmd.Context())
		if err != nil {
			return err
		}
		target = tag
	}

	// A dev build sorts below every release.
	cmp := -1
	if !dev {
		c, err := selfupdate.Compare(version, target)
		if err != nil {
			return err
		}
		cmp = c
	}

	if check {
		if asJSON {
			return writeJSON(upgradeCheck{Current: version, Latest: target, UpdateAvailable: cmp < 0})
		}
		if cmp < 0 {
			fmt.Printf("lantern %s is installed; %s is available — run lantern upgrade\n", version, target)
		} else {
			fmt.Printf("lantern %s is up to date (latest release %s)\n", version, target)
		}
		return nil
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return fmt.Errorf("locating the lantern binary: %w", err)
	}

	if !force && (cmp == 0 || (latest && cmp > 0)) {
		if cmp == 0 {
			fmt.Fprintf(os.Stderr, "lantern %s is already installed; pass --force to reinstall\n", version)
		} else {
			fmt.Fprintf(os.Stderr, "lantern %s is newer than the latest release %s\n", version, target)
		}
		if asJSON {
			return writeJSON(upgradeResult{Previous: version, Current: version, Path: exe})
		}
		return nil
	}

	if err := u.Install(cmd.Context(), target, exe, os.Stderr); err != nil {
		return err
	}
	installed := strings.TrimPrefix(target, "v")
	fmt.Fprintf(os.Stderr, "installed lantern %s at %s (was %s)\n", installed, exe, version)
	if asJSON {
		return writeJSON(upgradeResult{Previous: version, Current: installed, Path: exe})
	}
	return nil
}

func writeJSON(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return output.JSON(os.Stdout, body)
}

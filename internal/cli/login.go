package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"codeberg.org/kehvyn/pglantern-cli/internal/api"
	"codeberg.org/kehvyn/pglantern-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save an API key (created in the web UI under /users/api-keys)",
		Long: "Save an API key for later commands. Keys are minted in the pgLantern web UI\n" +
			"under /users/api-keys. The key is validated against the server, then stored\n" +
			"with the host in ~/.config/lantern/config.json.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			host := resolveHost(cmd, cfg)

			withToken, _ := cmd.Flags().GetBool("with-token")
			var key string
			if !withToken && term.IsTerminal(int(os.Stdin.Fd())) {
				// Interactive paste: mask the key so it doesn't land in the
				// scrollback. ReadPassword swallows the newline; echo it so the
				// next line starts fresh.
				fmt.Fprint(os.Stderr, "Paste your pgLantern API key: ")
				raw, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return fmt.Errorf("reading API key: %w", err)
				}
				key = strings.TrimSpace(string(raw))
			} else {
				if !withToken {
					fmt.Fprint(os.Stderr, "Paste your pgLantern API key: ")
				}
				reader := bufio.NewReader(os.Stdin)
				line, err := reader.ReadString('\n')
				if err != nil && line == "" {
					return fmt.Errorf("reading API key: %w", err)
				}
				key = strings.TrimSpace(line)
			}
			if key == "" {
				return fmt.Errorf("no API key provided")
			}

			// Any authenticated endpoint proves the key; /versions is small.
			if _, err := api.New(host, key).Get("/versions", nil); err != nil {
				return fmt.Errorf("key rejected by %s: %w", host, err)
			}

			if err := config.Save(config.Config{Host: host, APIKey: key}); err != nil {
				return err
			}
			path, _ := config.Path()
			fmt.Fprintf(os.Stderr, "Logged in to %s (saved to %s)\n", host, path)
			return nil
		},
	}
	cmd.Flags().Bool("with-token", false, "read the API key from stdin without prompting")
	return cmd
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the saved API key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Remove(); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Logged out.")
			return nil
		},
	}
}

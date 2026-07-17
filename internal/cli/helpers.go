package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"codeberg.org/kehvyn/horton-cli/internal/api"
	"codeberg.org/kehvyn/horton-cli/internal/config"
	"codeberg.org/kehvyn/horton-cli/internal/output"
	"github.com/spf13/cobra"
)

const defaultHost = "https://pglantern.com"

// resolveHost: --host flag, then $HORTON_HOST, then config file, then the
// hosted default.
func resolveHost(cmd *cobra.Command, cfg config.Config) string {
	if host, _ := cmd.Flags().GetString("host"); host != "" {
		return host
	}
	if host := os.Getenv("HORTON_HOST"); host != "" {
		return host
	}
	if cfg.Host != "" {
		return cfg.Host
	}
	return defaultHost
}

// resolveKey: --api-key flag, then $HORTON_API_KEY, then config file.
func resolveKey(cmd *cobra.Command, cfg config.Config) string {
	if key, _ := cmd.Flags().GetString("api-key"); key != "" {
		return key
	}
	if key := os.Getenv("HORTON_API_KEY"); key != "" {
		return key
	}
	return cfg.APIKey
}

func clientFrom(cmd *cobra.Command) (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	key := resolveKey(cmd, cfg)
	if key == "" {
		return nil, fmt.Errorf("no API key: run `horton login`, set $HORTON_API_KEY, or pass --api-key")
	}
	return api.New(resolveHost(cmd, cfg), key), nil
}

// collectQuery builds query params from the named flags the user actually
// set, so defaults are never sent and the server stays authoritative. Flag
// names map to param names by dash→underscore (--changed-since →
// changed_since).
func collectQuery(cmd *cobra.Command, names ...string) url.Values {
	q := url.Values{}
	for _, name := range names {
		if cmd.Flags().Changed(name) {
			q.Set(strings.ReplaceAll(name, "-", "_"), cmd.Flags().Lookup(name).Value.String())
		}
	}
	return q
}

// enumFlag is a pflag.Value that rejects values outside its allowed set at
// parse time.
type enumFlag struct {
	allowed []string
	value   string
}

func (e *enumFlag) String() string { return e.value }
func (e *enumFlag) Type() string   { return "string" }

func (e *enumFlag) Set(s string) error {
	for _, a := range e.allowed {
		if s == a {
			e.value = s
			return nil
		}
	}
	return fmt.Errorf("must be one of: %s", strings.Join(e.allowed, ", "))
}

func addEnumFlag(cmd *cobra.Command, name, usage string, allowed ...string) {
	cmd.Flags().Var(&enumFlag{allowed: allowed}, name,
		fmt.Sprintf("%s: %s", usage, strings.Join(allowed, "|")))
}

// getRender fetches the path and either passes the raw JSON through (--json)
// or decodes into T and hands it to render.
func getRender[T any](cmd *cobra.Command, path string, q url.Values, render func(T)) error {
	client, err := clientFrom(cmd)
	if err != nil {
		return err
	}
	body, err := client.Get(path, q)
	if err != nil {
		return err
	}
	if raw, _ := cmd.Flags().GetBool("json"); raw {
		return output.JSON(os.Stdout, body)
	}
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	render(v)
	return nil
}

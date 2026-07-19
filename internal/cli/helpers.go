package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"codeberg.org/kehvyn/pglantern-cli/internal/api"
	"codeberg.org/kehvyn/pglantern-cli/internal/config"
	"codeberg.org/kehvyn/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

const defaultHost = "https://pglantern.com"

// resolveHost: --host flag, then $LANTERN_HOST, then config file, then the
// hosted default.
func resolveHost(cmd *cobra.Command, cfg config.Config) string {
	if host, _ := cmd.Flags().GetString("host"); host != "" {
		return host
	}
	if host := os.Getenv("LANTERN_HOST"); host != "" {
		return host
	}
	if cfg.Host != "" {
		return cfg.Host
	}
	return defaultHost
}

// resolveKey: --api-key flag, then $LANTERN_API_KEY, then config file.
func resolveKey(cmd *cobra.Command, cfg config.Config) string {
	if key, _ := cmd.Flags().GetString("api-key"); key != "" {
		return key
	}
	if key := os.Getenv("LANTERN_API_KEY"); key != "" {
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
		return nil, fmt.Errorf("no API key: run `lantern login`, set $LANTERN_API_KEY, or pass --api-key")
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

// addIDFlag registers the --id flag that puts a collection command into the
// server's exact-IDs mode. StringArray, not StringSlice: StringSlice splits on
// commas, and the server never splits — `a,b` is one literal id there, so
// splitting here would silently diverge.
func addIDFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().StringArray("id", nil, usage+" (repeatable, max 100; `-` reads ids from stdin)")
}

// collectIDs reads the --id values in order, splicing newline-delimited ids
// from stdin wherever the literal `-` appears, so a piped list and hand-typed
// ids can be mixed. Entries are trimmed, blanks dropped, and duplicates removed
// keeping first-seen order — mirroring what the server does to `ids[]`. Stdin
// is consumed at most once however many `-` are passed.
func collectIDs(cmd *cobra.Command, stdin io.Reader) ([]string, error) {
	raw, err := cmd.Flags().GetStringArray("id")
	if err != nil {
		return nil, err
	}

	var ids []string
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}

	readStdin := false
	for _, entry := range raw {
		if entry != "-" {
			add(entry)
			continue
		}
		if readStdin {
			continue
		}
		readStdin = true
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			add(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading ids from stdin: %w", err)
		}
	}
	return ids, nil
}

// rejectPaginationWithIDs enforces the server's rule that exact-IDs mode is not
// paginated, locally and before any HTTP round-trip. It names all three flags
// rather than the one that tripped, so the user learns the whole rule at once.
func rejectPaginationWithIDs(cmd *cobra.Command) error {
	if !cmd.Flags().Changed("id") {
		return nil
	}
	for _, name := range []string{"limit", "after", "before"} {
		if cmd.Flags().Changed(name) {
			return fmt.Errorf("--id cannot be combined with --limit, --after, or --before")
		}
	}
	return nil
}

// requireArg validates that a command received exactly one positional
// argument, naming what's missing (e.g. "an attachment id") and echoing the
// usage line, instead of cobra's bare "accepts 1 arg(s), received 0".
func requireArg(what string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return argError(cmd, what)
		}
		return nil
	}
}

// requireArgs is requireArg for commands that accept one or more arguments,
// such as a multi-word search query.
func requireArgs(what string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return argError(cmd, what)
		}
		return nil
	}
}

func argError(cmd *cobra.Command, what string) error {
	return fmt.Errorf("%s requires %s\n\nUsage:\n  %s", cmd.CommandPath(), what, cmd.UseLine())
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

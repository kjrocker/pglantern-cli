package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
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
			return usagef("--id cannot be combined with --limit, --after, or --before")
		}
	}
	return nil
}

// addPaginationFlags registers the cursor-pagination flag set shared by every
// paged collection command, plus the client-side --all/--max page follower.
func addPaginationFlags(cmd *cobra.Command) {
	cmd.Flags().IntP("limit", "n", 0, "page size (server default 25, max 100)")
	cmd.Flags().String("after", "", "page cursor")
	cmd.Flags().String("before", "", "page cursor")
	cmd.Flags().Bool("all", false, "follow next_cursor and fetch every page (bounded by --max)")
	cmd.Flags().Int("max", 5000, "with --all, stop after this many rows")
}

// positionalQuery folds positional args into the q param, matching `search`'s
// ergonomics on commands where the query is optional. The positional and --q
// forms are exclusive — silently preferring one would hide a typo'd query.
func positionalQuery(cmd *cobra.Command, args []string, q url.Values) error {
	if len(args) == 0 {
		return nil
	}
	if cmd.Flags().Changed("q") {
		return usagef("%s takes the query positionally or via --q, not both", cmd.CommandPath())
	}
	q.Set("q", strings.Join(args, " "))
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
	return usagef("%s requires %s\n\nUsage:\n  %s", cmd.CommandPath(), what, cmd.UseLine())
}

// addOpenFlags registers the flags shared by every `open` subcommand: --site
// targets our own browse page instead of the upstream archive, --print emits
// the resolved URL instead of launching a browser.
func addOpenFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("site", false, "open the pgLantern page instead of the upstream archive page")
	cmd.Flags().Bool("print", false, "print the resolved URL instead of opening a browser")
}

// openURL picks the archive or --site URL, then either prints it (--print, or
// as a copyable fallback when no opener exists) or launches the browser. On a
// successful launch it notes `# opening <url>` on stderr, matching the
// EmptyNote/CursorFooter stderr-note convention so stdout stays pipe-clean.
func openURL(cmd *cobra.Command, archiveURL, siteURL string) error {
	target := archiveURL
	if site, _ := cmd.Flags().GetBool("site"); site {
		target = siteURL
	}
	if pr, _ := cmd.Flags().GetBool("print"); pr {
		fmt.Fprintln(os.Stdout, target)
		return nil
	}
	if err := output.OpenBrowser(target); err != nil {
		// No opener (headless/CI/SSH) — print the URL so it's still copyable,
		// and surface the failure as a non-zero exit.
		fmt.Fprintln(os.Stdout, target)
		return err
	}
	fmt.Fprintf(os.Stderr, "# opening %s\n", target)
	return nil
}

// hostFor resolves the API host for building --site URLs. Offline — config is a
// local file and no key is required, since open makes no request in the
// client-side path.
func hostFor(cmd *cobra.Command) string {
	cfg, _ := config.Load()
	return resolveHost(cmd, cfg)
}

// getOpen fetches path, decodes into api.Item[T], picks the (archive, site) URL
// pair off the decoded item, and opens it. Used only by the commit-prefix
// branch of `commits open`, where a short sha must be resolved server-side.
// Ignores --json: open is an action, not a data command.
func getOpen[T any](cmd *cobra.Command, path string, pick func(T) (archiveURL, siteURL string)) error {
	client, err := clientFrom(cmd)
	if err != nil {
		return err
	}
	body, err := client.Get(path, nil)
	if err != nil {
		return err
	}
	var item api.Item[T]
	if err := json.Unmarshal(body, &item); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	archiveURL, siteURL := pick(item.Data)
	return openURL(cmd, archiveURL, siteURL)
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

// getRenderPage is getRender for paged collections: under --all it follows
// next_cursor client-side via getRenderAll, otherwise it fetches one page.
func getRenderPage[T any](cmd *cobra.Command, path string, q url.Values, render func(api.Page[T])) error {
	if all, _ := cmd.Flags().GetBool("all"); all {
		return getRenderAll(cmd, path, q, render)
	}
	if cmd.Flags().Changed("max") {
		return usagef("--max only applies with --all")
	}
	return getRender(cmd, path, q, render)
}

// getRenderAll implements --all: it follows next_cursor until the collection
// or the --max row ceiling runs out. Requests are strictly sequential — one in
// flight, the next only after the previous page is consumed — on the client's
// single http.Client; no concurrency, no retries. A failed page aborts with the
// error after printing the cursor that refetches it. Table mode accumulates
// rows (bounded by --max) and renders once with no cursor footer; --json
// streams each page's raw body as one compact line as it is fetched.
//
// The last page's limit is trimmed to the rows remaining under --max, so a
// capped run stops at exactly --max rows and the resume cursor in the stderr
// note continues from the very next row.
func getRenderAll[T any](cmd *cobra.Command, path string, q url.Values, render func(api.Page[T])) error {
	if cmd.Flags().Changed("before") {
		return usagef("--all cannot be combined with --before")
	}
	if f := cmd.Flags().Lookup("id"); f != nil && f.Changed {
		return usagef("--all cannot be combined with --id")
	}
	max, _ := cmd.Flags().GetInt("max")
	if max <= 0 {
		return usagef("--max must be a positive integer")
	}
	// Unset --limit means the server's default 25; --all wants fewer
	// round-trips for the same rows, so it asks for full pages.
	perPage := 100
	if cmd.Flags().Changed("limit") {
		perPage, _ = cmd.Flags().GetInt("limit")
	}
	client, err := clientFrom(cmd)
	if err != nil {
		return err
	}
	jsonMode, _ := cmd.Flags().GetBool("json")

	var rows []T
	count := 0
	for {
		fetch := perPage
		if remaining := max - count; fetch > remaining {
			fetch = remaining
		}
		q.Set("limit", strconv.Itoa(fetch))
		body, err := client.Get(path, q)
		if err != nil {
			if cursor := q.Get("after"); cursor != "" {
				fmt.Fprintf(os.Stderr, "# page failed; resume with --after %s\n", cursor)
			}
			return err
		}
		var page api.Page[T]
		if err := json.Unmarshal(body, &page); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
		if jsonMode {
			if err := output.JSONLine(os.Stdout, body); err != nil {
				return err
			}
		} else {
			rows = append(rows, page.Data...)
		}
		count += len(page.Data)
		next := ""
		if page.NextCursor != nil {
			next = *page.NextCursor
		}
		if next == "" || len(page.Data) == 0 {
			break
		}
		if count >= max {
			fmt.Fprintf(os.Stderr, "# stopped at %d rows; resume with --after %s or raise --max\n", count, next)
			break
		}
		q.Set("after", next)
	}
	if !jsonMode {
		render(api.Page[T]{Data: rows})
	}
	return nil
}

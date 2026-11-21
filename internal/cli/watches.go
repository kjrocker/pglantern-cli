package cli

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
	"git.kehvyn.dev/kevin/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

// watchTypes are the watch kinds the server accepts, and watchPrimaryFlag maps
// each subject-keyed type to the one flag that carries its subject. The filter
// types (query, thread_query) have no single subject: they take the search /
// threads filter flags and need at least one. Those checks — a valid --type and
// its subject — are the only things done locally; every other rule (unknown or
// invalid filters, webhook entitlement, watch limits, endpoint ownership)
// belongs to the server, whose 422/403 is printed verbatim.
var watchTypes = []string{"thread", "sender", "path", "guc", "query", "thread_query"}

var watchPrimaryFlag = map[string]string{
	"thread": "message-id",
	"sender": "sender-id",
	"path":   "path",
	"guc":    "name",
}

// watchFilterFlags are the flags a filter watch's params are built from: the
// /search and /threads filter set, registered by the same helpers `search` and
// `threads` use. A query watch sending a threads-only one (--min-matches) gets
// the server's 422 naming it.
var watchFilterFlags = append(append(append(append(append(
	[]string{"q", "sender", "committed", "path", "major", "cf-status"},
	threadFilterScalars...), threadFilterArrays...),
	messageGroupScalars...), messageGroupArrays...), threadMatchScalars...)

// watchParamsCell renders a watch's type-specific params as a compact,
// key-sorted `k=v k=v` cell so the column is stable across rows and types.
func watchParamsCell(params map[string]any) string {
	if len(params) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+watchParamValue(params[k]))
	}
	return strings.Join(parts, " ")
}

// watchParamValue stringifies one param value. JSON numbers decode as float64,
// so whole ones (sender_id) print as integers rather than 42.
func watchParamValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case nil:
		return "-"
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, watchParamValue(e))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(t)
	}
}

// watchDeliveryCell folds channel and endpoint into one column: email watches
// say `email`, webhook ones name the endpoint they post to.
func watchDeliveryCell(w api.Watch) string {
	if w.Channel == "webhook" {
		return "webhook:" + output.OrDash(w.EndpointID)
	}
	return w.Channel
}

func watchStatusCell(w api.Watch) string {
	if w.Active {
		return "active"
	}
	return "paused"
}

func watchRows(watches []api.Watch) [][]string {
	rows := make([][]string, 0, len(watches))
	for _, w := range watches {
		rows = append(rows, []string{
			w.ID,
			w.Type,
			output.Truncate(watchParamsCell(w.Params), 48),
			watchDeliveryCell(w),
			output.OrDash(w.Cadence),
			watchStatusCell(w),
		})
	}
	return rows
}

// watchDetail is the single-watch block, shared by get, create, and the
// pause/resume/cadence updates so a watch reads identically however you got it.
func watchDetail(w api.Watch) {
	output.Detail(os.Stdout, [][2]string{
		{"Id", w.ID},
		{"Type", w.Type},
		{"Params", watchParamsCell(w.Params)},
		{"Delivery", watchDeliveryCell(w)},
		{"Cadence", output.OrDash(w.Cadence)},
		{"Status", watchStatusCell(w)},
		{"Created", w.InsertedAt},
		{"Updated", w.UpdatedAt},
	})
}

func newWatchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watches",
		Short: "List your saved watches",
		Long: "List the watches on your account. Watches are the CLI's only write\n" +
			"surface: creating, pausing, retiming, or deleting one needs an API key\n" +
			"minted with manage access (/users/api-keys). A read-only key can list\n" +
			"them but gets a 403 key_read_only on every mutation.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getRender(cmd, "/watches", nil, func(page api.Page[api.Watch]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no watches")
					return
				}
				output.Table(os.Stdout,
					[]string{"ID", "TYPE", "PARAMS", "DELIVERY", "CADENCE", "STATUS"},
					watchRows(page.Data))
			})
		},
	}
	cmd.AddCommand(
		newWatchesGetCmd(),
		newWatchesCreateCmd(),
		newWatchesPauseCmd(),
		newWatchesResumeCmd(),
		newWatchesCadenceCmd(),
		newWatchesDeleteCmd(),
		newWatchesEndpointsCmd(),
	)
	return cmd
}

func newWatchesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Show one watch",
		Args:  requireArg("a watch id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return getRender(cmd, "/watches/"+url.PathEscape(args[0]), nil,
				func(item api.Item[api.Watch]) { watchDetail(item.Data) })
		},
	}
}

// watchCreateBody is the POST /watches payload. type, channel, and params are
// required — the server picks no default channel, so the CLI sends `email`
// unless --channel says otherwise. The server rejects unknown top-level keys,
// so the optional fields are omitted rather than sent empty.
type watchCreateBody struct {
	Type       string         `json:"type"`
	Channel    string         `json:"channel"`
	Params     map[string]any `json:"params"`
	EndpointID string         `json:"endpoint_id,omitempty"`
	Cadence    string         `json:"cadence,omitempty"`
}

func newWatchesCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --type <type> [flags]",
		Short: "Create a watch (needs a manage key)",
		Long: "Create a watch. --type picks what is watched and which subject flag\n" +
			"applies: thread → --message-id, sender → --sender-id, path → --path,\n" +
			"guc → --name. query and thread_query take the same filter flags as\n" +
			"`search` and `threads` (at least one): query fires on new messages\n" +
			"`search` would return, thread_query on threads that match and get a\n" +
			"new message. There is no --from/--to: a watch matches from now on.\n" +
			"Everything is passed through to the server, which validates it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("type") {
				return usagef("watches create requires --type (%s)", strings.Join(watchTypes, "|"))
			}
			watchType := cmd.Flags().Lookup("type").Value.String()
			if primary, ok := watchPrimaryFlag[watchType]; ok && !cmd.Flags().Changed(primary) {
				return usagef("watches create --type %s requires --%s", watchType, primary)
			}

			params, err := watchCreateParams(cmd, watchType)
			if err != nil {
				return err
			}
			// channel is required server-side with no default, so an unset
			// --channel means email, the common case.
			channel := cmd.Flags().Lookup("channel").Value.String()
			if channel == "" {
				channel = "email"
			}
			body := watchCreateBody{Type: watchType, Channel: channel, Params: params}
			if cmd.Flags().Changed("cadence") {
				body.Cadence = cmd.Flags().Lookup("cadence").Value.String()
			}
			if cmd.Flags().Changed("endpoint") {
				body.EndpointID, _ = cmd.Flags().GetString("endpoint")
			}
			return mutateRender(cmd, http.MethodPost, "/watches", body,
				func(item api.Item[api.Watch]) { watchDetail(item.Data) })
		},
	}
	addEnumFlag(cmd, "type", "what to watch", watchTypes...)
	addEnumFlag(cmd, "channel", "how to deliver (default email)", "email", "webhook")
	addEnumFlag(cmd, "cadence", "email cadence", "daily", "weekly")
	cmd.Flags().String("endpoint", "", "webhook endpoint id (--channel webhook)")
	cmd.Flags().String("message-id", "", "thread watch: a Message-Id in the thread")
	cmd.Flags().String("name", "", "guc watch: a GUC name")
	cmd.Flags().String("path", "", "path watch: a source path prefix; filter watches: threads touching it")
	cmd.Flags().String("major", "", "path watch: restrict commits to this major; filter watches: threads landed in it")
	// The filter set, as on `search` / `threads`. --sender-id is the message
	// group's repeatable one; a sender watch takes exactly one.
	cmd.Flags().String("q", "", "filter watches: full-text query (optional)")
	cmd.Flags().String("sender", "", "filter watches: a message's sender email or name contains this")
	cmd.Flags().Bool("committed", false, "filter watches: only threads with a landed commit (--committed=false for none)")
	addEnumFlag(cmd, "cf-status", "filter watches: threads with a commitfest patch in this status", cfSearchStatuses...)
	addThreadFilterFlags(cmd,
		"query: messages on any of these lists; thread_query: threads with a message on any",
		"drop messages (query) or threads (thread_query) on this list")
	addMessageGroupFlags(cmd,
		"query: the message subject; thread_query: the thread subject",
		"sender watch: the sender id; filter watches: a message is from any of these")
	addThreadMatchFlags(cmd)
	return cmd
}

// collectParams builds a JSON params object from the named flags the user set,
// typed by flag kind: ints and bools native, repeatable flags as string arrays
// (the server trims, de-dupes and canonicalizes), the rest as strings. Flag
// names map to param names by dash→underscore, as in collectQuery.
func collectParams(cmd *cobra.Command, names ...string) map[string]any {
	params := map[string]any{}
	for _, name := range names {
		f := cmd.Flags().Lookup(name)
		if f == nil || !f.Changed {
			continue
		}
		key := strings.ReplaceAll(name, "-", "_")
		switch f.Value.Type() {
		case "int":
			v, _ := cmd.Flags().GetInt(name)
			params[key] = v
		case "bool":
			v, _ := cmd.Flags().GetBool(name)
			params[key] = v
		case "stringArray":
			v, _ := cmd.Flags().GetStringArray(name)
			params[key] = v
		default:
			params[key] = f.Value.String()
		}
	}
	return params
}

// watchCreateParams builds the type-specific params object, folding in the
// optional narrowing flags only when they were set.
func watchCreateParams(cmd *cobra.Command, watchType string) (map[string]any, error) {
	params := map[string]any{}
	add := func(name, key string) {
		if cmd.Flags().Changed(name) {
			params[key] = cmd.Flags().Lookup(name).Value.String()
		}
	}
	switch watchType {
	case "thread":
		id, _ := cmd.Flags().GetString("message-id")
		params["message_id"] = normalizeMessageID(id)
	case "sender":
		ids, _ := cmd.Flags().GetStringArray("sender-id")
		if len(ids) != 1 {
			return nil, usagef("a sender watch takes exactly one --sender-id")
		}
		id, err := strconv.Atoi(strings.TrimSpace(ids[0]))
		if err != nil {
			return nil, usagef("--sender-id must be an integer")
		}
		params["sender_id"] = id
	case "path":
		add("path", "path")
		add("major", "major")
	case "guc":
		add("name", "name")
	case "query", "thread_query":
		params = collectParams(cmd, watchFilterFlags...)
		if len(params) == 0 {
			return nil, usagef("watches create --type %s needs at least one filter flag "+
				"(the `search` / `threads` filters, e.g. --q, --sender-role, --list)", watchType)
		}
	}
	return params, nil
}

func newWatchesPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <id>",
		Short: "Stop delivering a watch (needs a manage key)",
		Args:  requireArg("a watch id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setWatchActive(cmd, args, false)
		},
	}
}

func newWatchesResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <id>",
		Short: "Start delivering a paused watch (needs a manage key)",
		Args:  requireArg("a watch id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setWatchActive(cmd, args, true)
		},
	}
}

// setWatchActive is the one PATCH behind pause and resume.
func setWatchActive(cmd *cobra.Command, args []string, active bool) error {
	body := struct {
		Active bool `json:"active"`
	}{Active: active}
	return mutateRender(cmd, http.MethodPatch, "/watches/"+url.PathEscape(args[0]), body,
		func(item api.Item[api.Watch]) { watchDetail(item.Data) })
}

func newWatchesCadenceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cadence <id> <daily|weekly>",
		Short: "Change an email watch's cadence (needs a manage key)",
		Long: "Retime an email watch. daily sends one email a day;\n" +
			"weekly sends one email every Monday covering everything since the last.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return usagef("%s requires a watch id and a cadence\n\nUsage:\n  %s",
					cmd.CommandPath(), cmd.UseLine())
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[1] != "daily" && args[1] != "weekly" {
				return usagef("cadence must be one of: daily, weekly (got %q)", args[1])
			}
			body := struct {
				Cadence string `json:"cadence"`
			}{Cadence: args[1]}
			return mutateRender(cmd, http.MethodPatch, "/watches/"+url.PathEscape(args[0]), body,
				func(item api.Item[api.Watch]) { watchDetail(item.Data) })
		},
	}
}

func newWatchesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a watch (needs a manage key)",
		Long: "Delete a watch. There is no confirmation prompt — this is a dumb client,\n" +
			"and the server's 204 is the receipt. stdout stays empty; the removed id\n" +
			"is noted on stderr.",
		Args: requireArg("a watch id"),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := mutateRender(cmd, http.MethodDelete, "/watches/"+url.PathEscape(args[0]), nil,
				func(item api.Item[api.Watch]) {})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "# deleted %s\n", args[0])
			return nil
		},
	}
}

func newWatchesEndpointsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "endpoints",
		Short: "List your webhook endpoints",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getRender(cmd, "/webhook-endpoints", nil, func(page api.Page[api.WebhookEndpoint]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no webhook endpoints")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, e := range page.Data {
					status := "active"
					if !e.Active {
						status = "disabled " + output.OrDash(e.DisabledAt)
					}
					rows = append(rows, []string{e.ID, e.URL, status})
				}
				output.Table(os.Stdout, []string{"ID", "URL", "STATUS"}, rows)
			})
		},
	}
}

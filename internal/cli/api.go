package cli

import (
	"net/url"
	"os"
	"strings"

	"codeberg.org/kehvyn/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "GET any /api/v1 path and print the raw JSON",
		Long: "Escape hatch for endpoints or params this CLI doesn't wrap. The path is\n" +
			"relative to /api/v1 (e.g. `lantern api /messages --param limit=3`).",
		Args: requireArg("an API path"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			params, _ := cmd.Flags().GetStringArray("param")
			q := url.Values{}
			for _, p := range params {
				key, value, ok := strings.Cut(p, "=")
				if !ok {
					return usagef("--param must be key=value, got %q", p)
				}
				q.Add(key, value)
			}
			client, err := clientFrom(cmd)
			if err != nil {
				return err
			}
			body, err := client.Get(path, q)
			if err != nil {
				return err
			}
			return output.JSON(os.Stdout, body)
		},
	}
	cmd.Flags().StringArray("param", nil, "query parameter as key=value (repeatable)")
	return cmd
}

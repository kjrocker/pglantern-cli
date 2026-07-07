package cli

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/kjrocker/horton/internal/output"
	"github.com/spf13/cobra"
)

func newAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "GET any /api/v1 path and print the raw JSON",
		Long: "Escape hatch for endpoints or params this CLI doesn't wrap. The path is\n" +
			"relative to /api/v1 (e.g. `horton api /messages --param limit=3`).",
		Args: cobra.ExactArgs(1),
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
					return fmt.Errorf("--param must be key=value, got %q", p)
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

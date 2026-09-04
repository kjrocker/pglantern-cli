package cli

import (
	"fmt"
	"os"
	"strconv"

	"git.kehvyn.dev/kevin/pglantern-cli/internal/api"
	"git.kehvyn.dev/kevin/pglantern-cli/internal/output"
	"github.com/spf13/cobra"
)

// The /analytics endpoints return bounded aggregates — one row per bucket, or
// `limit` rows — as a bare {data: [...]} with no cursors, so none of these
// subcommands carry pagination flags or print a cursor footer.

func newAnalyticsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Archive-wide aggregates: message volume, sender growth, rankings, thread sizes",
	}
	cmd.AddCommand(
		newAnalyticsMessagesCmd(),
		newAnalyticsSendersCmd(),
		newAnalyticsTopSendersCmd(),
		newAnalyticsThreadSizesCmd(),
	)
	return cmd
}

// addSeriesFlags registers the flag set the two time-series commands share;
// column names which timestamp the from/to window bounds.
func addSeriesFlags(cmd *cobra.Command, column string) {
	addEnumFlag(cmd, "interval", "bucket granularity (server default year)", "year", "quarter", "month")
	cmd.Flags().String("from", "", fmt.Sprintf("ISO-8601 lower bound on %s", column))
	cmd.Flags().String("to", "", fmt.Sprintf("ISO-8601 upper bound on %s", column))
	cmd.Flags().Bool("cumulative", false,
		"count is a running total through each bucket, not the bucket's own rate")
}

func renderSeries(page api.Page[api.SeriesPoint]) {
	if len(page.Data) == 0 {
		output.EmptyNote("no results")
		return
	}
	rows := make([][]string, 0, len(page.Data))
	for _, p := range page.Data {
		rows = append(rows, []string{p.Bucket, p.BucketStart, strconv.Itoa(p.Count)})
	}
	output.Table(os.Stdout, []string{"BUCKET", "START", "COUNT"}, rows)
}

// The two series commands keep their paths literal in their own RunE (rather
// than sharing a parameterized constructor) so the API⇄CLI drift gate in the
// server repo can extract endpoint and params from one function scope.

func newAnalyticsMessagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Messages per time bucket",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "interval", "from", "to", "cumulative")
			return getRender(cmd, "/analytics/messages", q, renderSeries)
		},
	}
	addSeriesFlags(cmd, "sent_at")
	return cmd
}

func newAnalyticsSendersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "senders",
		Short: "New senders per time bucket of their first post",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "interval", "from", "to", "cumulative")
			return getRender(cmd, "/analytics/senders", q, renderSeries)
		},
	}
	addSeriesFlags(cmd, "the sender's first post")
	return cmd
}

func newAnalyticsTopSendersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top-senders",
		Short: "Most prolific senders, highest first",
		Long: "The most prolific senders, highest first. Without --from/--to the count is the\n" +
			"sender's all-time total; with a window it is their messages inside that window.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "limit", "from", "to")
			return getRender(cmd, "/analytics/top-senders", q, func(page api.Page[api.TopSender]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no results")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, s := range page.Data {
					name := "-"
					if s.DisplayName != "" {
						name = output.Truncate(s.DisplayName, 40)
					}
					rows = append(rows, []string{name, s.Email, strconv.Itoa(s.MessageCount)})
				}
				output.Table(os.Stdout, []string{"NAME", "EMAIL", "MESSAGES"}, rows)
			})
		},
	}
	cmd.Flags().IntP("limit", "n", 0, "senders to return, 1-100 (server default 25)")
	cmd.Flags().String("from", "", "ISO-8601 lower bound on sent_at")
	cmd.Flags().String("to", "", "ISO-8601 upper bound on sent_at")
	return cmd
}

func newAnalyticsThreadSizesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread-sizes",
		Short: "Thread size distribution in fixed bands (1, 2, 3–5, …, 50+)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := collectQuery(cmd, "from", "to")
			return getRender(cmd, "/analytics/thread-sizes", q, func(page api.Page[api.SizeBucket]) {
				if len(page.Data) == 0 {
					output.EmptyNote("no results")
					return
				}
				rows := make([][]string, 0, len(page.Data))
				for _, b := range page.Data {
					rows = append(rows, []string{b.Bucket, strconv.Itoa(b.Count)})
				}
				output.Table(os.Stdout, []string{"SIZE", "THREADS"}, rows)
			})
		},
	}
	cmd.Flags().String("from", "", "ISO-8601 lower bound on the thread's started_at")
	cmd.Flags().String("to", "", "ISO-8601 upper bound on the thread's started_at")
	return cmd
}

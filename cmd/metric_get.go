package cmd

import (
	"fmt"
	"time"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/glassnode/glassnode-cli/internal/output"
	"github.com/glassnode/glassnode-cli/internal/timeparse"
	"github.com/spf13/cobra"
)

var metricGetCmd = &cobra.Command{
	Use:   "get <path>",
	Short: "Get metric data",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := api.NormalizePath(args[0])
		useBulk := api.IsBulkPath(path)
		if useBulk {
			path = api.TrimBulkSuffix(path)
		}

		apiKeyFlag, _ := cmd.Flags().GetString("api-key")
		apiKey, bearer, err := api.RequireAuth(cmd.Context(), apiKeyFlag)
		if err != nil {
			return err
		}
		client := api.NewClient(apiKey, bearer)

		assets, _ := cmd.Flags().GetStringArray("asset")
		exchanges, _ := cmd.Flags().GetStringArray("exchange")
		since, _ := cmd.Flags().GetString("since")
		until, _ := cmd.Flags().GetString("until")
		interval, _ := cmd.Flags().GetString("interval")
		currency, _ := cmd.Flags().GetString("currency")
		network, _ := cmd.Flags().GetString("network")

		params := map[string]string{}

		if since != "" {
			ts, err := timeparse.Parse(since)
			if err != nil {
				return fmt.Errorf("parsing --since: %w", err)
			}
			params["s"] = fmt.Sprintf("%d", ts)
		}
		if until != "" {
			ts, err := timeparse.Parse(until)
			if err != nil {
				return fmt.Errorf("parsing --until: %w", err)
			}
			params["u"] = fmt.Sprintf("%d", ts)
		}
		if interval != "" {
			params["i"] = interval
		}
		if currency != "" {
			params["c"] = currency
		}
		if network != "" {
			params["network"] = network
		}

		if useBulk {
			repeatedParams := map[string][]string{}
			if len(assets) > 0 {
				repeatedParams["a"] = assets
			}
			if len(exchanges) > 0 {
				repeatedParams["e"] = exchanges
			}

			// The API requires s for bulk requests and limits the range per
			// request; longer ranges are fetched in several requests with --split.
			plan, err := api.PlanBulk(params, time.Now())
			if err != nil {
				return err
			}
			split, _ := cmd.Flags().GetBool("split")
			windows := plan.Windows()
			if len(windows) > 1 && !split {
				return plan.RangeError()
			}
			if len(windows) > 1 {
				fmt.Fprintln(cmd.ErrOrStderr(), plan.Describe())
			}

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if dryRun {
				for _, w := range windows {
					u, err := client.BuildURL("/v1/metrics"+path+"/bulk", api.WindowParams(params, w), repeatedParams)
					if err != nil {
						return err
					}
					printDryRun(cmd, client, u)
				}
				return nil
			}

			resp, err := client.GetMetricBulkSplit(cmd.Context(), path, params, repeatedParams, plan)
			if err != nil {
				return err
			}
			format, _ := cmd.Flags().GetString("output")
			tsFmt, _ := cmd.Flags().GetString("timestamp-format")
			return output.Print(output.Options{Format: format, Data: resp, TimestampFormat: tsFmt})
		}

		if len(assets) > 0 {
			params["a"] = assets[0]
		}
		if len(exchanges) > 0 {
			params["e"] = exchanges[0]
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			u, err := client.BuildURL("/v1/metrics"+path, params, nil)
			if err != nil {
				return err
			}
			printDryRun(cmd, client, u)
			return nil
		}

		data, err := client.GetMetric(cmd.Context(), path, params)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("output")
		tsFmt, _ := cmd.Flags().GetString("timestamp-format")
		return output.Print(output.Options{Format: format, Data: data, TimestampFormat: tsFmt})
	},
}

func init() {
	metricGetCmd.Flags().StringArrayP("asset", "a", nil, "asset ID (repeatable for bulk)")
	metricGetCmd.Flags().StringP("since", "s", "", "start time")
	metricGetCmd.Flags().StringP("until", "u", "", "end time")
	metricGetCmd.Flags().StringP("interval", "i", "", "resolution: 10m, 1h, 24h, 1w, 1month")
	metricGetCmd.Flags().StringP("currency", "c", "", "native or usd")
	metricGetCmd.Flags().StringArrayP("exchange", "e", nil, "exchange filter (repeatable for bulk)")
	metricGetCmd.Flags().StringP("network", "n", "", "network filter")
	metricGetCmd.Flags().Bool("split", false, "bulk only: fetch a range longer than the API allows per request in several requests")
}

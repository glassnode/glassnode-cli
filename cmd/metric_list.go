package cmd

import (
	"fmt"
	"strings"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/glassnode/glassnode-cli/internal/output"
	"github.com/spf13/cobra"
)

var metricListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKeyFlag, _ := cmd.Flags().GetString("api-key")
		apiKey, bearer, err := api.RequireAuth(cmd.Context(), apiKeyFlag)
		if err != nil {
			return err
		}
		client := api.NewClient(apiKey, bearer)

		params, repeatedParams := metricListParamsFromFlags(cmd)
		format, _ := cmd.Flags().GetString("format")
		format = strings.ToLower(format)
		if format != "" && format != "csv" && format != "json" {
			return fmt.Errorf("--format must be csv or json, got %q", format)
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			u, err := client.BuildURL("/v1/metadata/metrics", params, repeatedParams)
			if err != nil {
				return err
			}
			printDryRun(cmd, client, u)
			if format != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "# --format then reads each listed metric's metadata (/v1/metadata/metric?path=...)")
			}
			return nil
		}

		metrics, err := client.ListMetrics(cmd.Context(), params, repeatedParams)
		if err != nil {
			return err
		}
		if format != "" {
			// The API's f filter returns nothing; check each metric's metadata
			// instead. Metadata requests cost no credits.
			fmt.Fprintf(cmd.ErrOrStderr(), "# checking the metadata of %d metrics for %s support\n", len(metrics), format)
			kept, skipped, err := client.FilterMetricsByFormat(cmd.Context(), metrics, format)
			if err != nil {
				return err
			}
			if len(skipped) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "# %d metrics have no metadata and were left out\n", len(skipped))
			}
			metrics = kept
		}

		outputFormat, _ := cmd.Flags().GetString("output")
		return output.Print(output.Options{Format: outputFormat, Data: metrics})
	},
}

func metricListParamsFromFlags(cmd *cobra.Command) (map[string]string, map[string][]string) {
	params := map[string]string{}
	repeatedParams := map[string][]string{}

	var assets []string
	if v, _ := cmd.Flags().GetString("asset"); v != "" {
		assets = append(assets, v)
	}
	if extra, _ := cmd.Flags().GetStringArray("assets"); len(extra) > 0 {
		assets = append(assets, extra...)
	}
	if len(assets) == 1 {
		params["a"] = assets[0]
	} else if len(assets) > 1 {
		repeatedParams["a"] = assets
	}
	if v, _ := cmd.Flags().GetString("currency"); v != "" {
		params["c"] = v
	}
	if v, _ := cmd.Flags().GetString("exchange"); v != "" {
		params["e"] = v
	}
	if v, _ := cmd.Flags().GetString("interval"); v != "" {
		params["i"] = v
	}
	if v, _ := cmd.Flags().GetString("from-exchange"); v != "" {
		params["from_exchange"] = v
	}
	if v, _ := cmd.Flags().GetString("to-exchange"); v != "" {
		params["to_exchange"] = v
	}
	if v, _ := cmd.Flags().GetString("miner"); v != "" {
		params["miner"] = v
	}
	if v, _ := cmd.Flags().GetString("maturity"); v != "" {
		params["maturity"] = v
	}
	if v, _ := cmd.Flags().GetString("network"); v != "" {
		params["network"] = v
	}
	if v, _ := cmd.Flags().GetString("period"); v != "" {
		params["period"] = v
	}
	if v, _ := cmd.Flags().GetString("quote-symbol"); v != "" {
		params["quote_symbol"] = v
	}

	return params, repeatedParams
}

func init() {
	metricListCmd.Flags().StringP("asset", "a", "", "filter by asset (single)")
	metricListCmd.Flags().StringArray("assets", nil, "filter by assets (multiple, e.g. --assets BTC --assets ETH)")
	metricListCmd.Flags().StringP("currency", "c", "", "filter by currency (e.g. native, usd)")
	metricListCmd.Flags().StringP("exchange", "e", "", "filter by exchange (e.g. binance, coinbase)")
	metricListCmd.Flags().StringP("format", "f", "", "keep only metrics available in this response format (csv or json); reads each metric's metadata")
	metricListCmd.Flags().StringP("interval", "i", "", "filter by time interval (e.g. 1h, 24h)")
	metricListCmd.Flags().String("from-exchange", "", "source exchange for inter-exchange metrics")
	metricListCmd.Flags().String("to-exchange", "", "destination exchange for inter-exchange metrics")
	metricListCmd.Flags().String("miner", "", "miner identifier for mining-related metrics")
	metricListCmd.Flags().String("maturity", "", "maturity period for derivatives metrics")
	metricListCmd.Flags().String("network", "", "network/blockchain for cross-chain metrics")
	metricListCmd.Flags().String("period", "", "time period for aggregation")
	metricListCmd.Flags().String("quote-symbol", "", "quote currency symbol for trading pairs")
}

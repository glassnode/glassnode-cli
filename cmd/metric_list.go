package cmd

import (
	"fmt"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/glassnode/glassnode-cli/internal/output"
	"github.com/spf13/cobra"
)

var metricListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKeyFlag, _ := cmd.Flags().GetString("api-key")
		apiKey, err := api.RequireAPIKey(apiKeyFlag)
		if err != nil {
			return err
		}
		client := api.NewClient(apiKey)

		asset, _ := cmd.Flags().GetString("asset")

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			params := map[string]string{}
			if asset != "" {
				params["a"] = asset
			}
			u, err := client.BuildURL("/v1/metadata/metrics", params, nil)
			if err != nil {
				return err
			}
			redacted, _ := api.RedactAPIKeyFromURL(u)
			fmt.Println(redacted)
			return nil
		}

		metrics, err := client.ListMetrics(cmd.Context(), asset)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("output")
		return output.Print(output.Options{Format: format, Data: metrics})
	},
}

func init() {
	metricListCmd.Flags().StringP("asset", "a", "", "filter by asset")
}

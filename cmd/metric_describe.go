package cmd

import (
	"fmt"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/glassnode/glassnode-cli/internal/output"
	"github.com/spf13/cobra"
)

var metricDescribeCmd = &cobra.Command{
	Use:   "describe <path>",
	Short: "Describe a metric by metric path",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := api.NormalizePath(args[0])

		apiKeyFlag, _ := cmd.Flags().GetString("api-key")
		apiKey, err := api.RequireAPIKey(apiKeyFlag)
		if err != nil {
			return err
		}
		client := api.NewClient(apiKey)

		asset, _ := cmd.Flags().GetString("asset")

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			params := map[string]string{"path": path}
			if asset != "" {
				params["a"] = asset
			}
			u, err := client.BuildURL("/v1/metadata/metric", params, nil)
			if err != nil {
				return err
			}
			redacted, _ := api.RedactAPIKeyFromURL(u)
			fmt.Println(redacted)
			return nil
		}

		meta, err := client.DescribeMetric(cmd.Context(), path, asset)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("output")
		return output.Print(output.Options{Format: format, Data: meta})
	},
}

func init() {
	metricDescribeCmd.Flags().StringP("asset", "a", "", "narrow down valid parameter values")
}

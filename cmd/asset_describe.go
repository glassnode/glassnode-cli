package cmd

import (
	"fmt"
	"strings"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/glassnode/glassnode-cli/internal/output"
	"github.com/spf13/cobra"
)

var assetDescribeCmd = &cobra.Command{
	Use:   "describe <id>",
	Short: "Describe an asset by asset ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		assetID := args[0]
		escaped := strings.ReplaceAll(assetID, "'", "''")

		apiKeyFlag, _ := cmd.Flags().GetString("api-key")
		apiKey, err := api.RequireAPIKey(apiKeyFlag)
		if err != nil {
			return err
		}
		client := api.NewClient(apiKey)

		filter := fmt.Sprintf("asset.id=='%s'", escaped)

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			params := map[string]string{"filter": filter}
			u, err := client.BuildURL("/v1/metadata/assets", params, nil)
			if err != nil {
				return err
			}
			redacted, _ := api.RedactAPIKeyFromURL(u)
			_, _ = fmt.Println(redacted)
			return nil
		}

		assets, err := client.ListAssets(cmd.Context(), filter)
		if err != nil {
			return err
		}
		if len(assets) == 0 {
			return fmt.Errorf("asset not found: %s", assetID)
		}

		format, _ := cmd.Flags().GetString("output")
		return output.Print(output.Options{Format: format, Data: assets[0]})
	},
}

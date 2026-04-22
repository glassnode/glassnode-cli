package cmd

import (
	"fmt"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/glassnode/glassnode-cli/internal/output"
	"github.com/spf13/cobra"
)

var creditsCmd = &cobra.Command{
	Use:   "credits",
	Short: "Show the API credits summary for your account",
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKeyFlag, _ := cmd.Flags().GetString("api-key")
		apiKey, err := api.RequireAPIKey(apiKeyFlag)
		if err != nil {
			return err
		}

		client := api.NewClient(apiKey)

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			u, err := client.BuildURL("/v1/user/api_usage", nil, nil)
			if err != nil {
				return err
			}
			redacted, _ := api.RedactAPIKeyFromURL(u)
			fmt.Println(redacted)
			return nil
		}

		resp, err := client.GetAPIUsage(cmd.Context())
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("output")
		tsFmt, _ := cmd.Flags().GetString("timestamp-format")
		return output.Print(output.Options{Format: format, Data: resp.Summary(), TimestampFormat: tsFmt})
	},
}

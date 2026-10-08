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
		apiKey, bearer, err := api.RequireAuth(cmd.Context(), apiKeyFlag)
		if err != nil {
			return err
		}

		client := api.NewClient(apiKey, bearer)

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			for _, path := range []string{"/v1/user/info", "/v1/user/api_usage"} {
				u, err := client.BuildURL(path, nil, nil)
				if err != nil {
					return err
				}
				redacted, _ := api.RedactAPIKeyFromURL(u)
				fmt.Println(redacted)
			}
			return nil
		}

		// The credit allowance resets daily on the advanced product and monthly on
		// every other one, so the period has to come from the account's products.
		info, err := client.GetUserInfo(cmd.Context())
		if err != nil {
			return err
		}

		resp, err := client.GetAPIUsage(cmd.Context())
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("output")
		tsFmt, _ := cmd.Flags().GetString("timestamp-format")
		return output.Print(output.Options{Format: format, Data: resp.Summary(info.CreditsPeriod()), TimestampFormat: tsFmt})
	},
}

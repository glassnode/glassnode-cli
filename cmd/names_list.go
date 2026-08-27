package cmd

import (
	"fmt"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/glassnode/glassnode-cli/internal/output"
	"github.com/spf13/cobra"
)

// runNamesListCommand executes a metadata list endpoint (e.g. /v1/metadata/tags)
// that returns {"data":[{"name":"..."},...]} and prints the plain list of names.
func runNamesListCommand(cmd *cobra.Command, endpoint string, withFilter bool) error {
	apiKeyFlag, _ := cmd.Flags().GetString("api-key")
	apiKey, bearer, err := api.RequireAuth(cmd.Context(), apiKeyFlag)
	if err != nil {
		return err
	}
	client := api.NewClient(apiKey, bearer)

	filter := ""
	if withFilter {
		filter, _ = cmd.Flags().GetString("filter")
		filter = normalizeFilter(filter)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		params := map[string]string{}
		if filter != "" {
			params["filter"] = filter
		}
		u, err := client.BuildURL(endpoint, params, nil)
		if err != nil {
			return err
		}
		redacted, _ := api.RedactAPIKeyFromURL(u)
		fmt.Println(redacted)
		return nil
	}

	names, err := client.ListNames(cmd.Context(), endpoint, filter)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString("output")
	return output.Print(output.Options{Format: format, Data: names})
}

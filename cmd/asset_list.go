package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/glassnode/gn/internal/api"
	"github.com/glassnode/gn/internal/output"
	"github.com/spf13/cobra"
)

// normalizeFilter quotes a bare RHS after == so CEL receives a string literal.
// Shells often strip double quotes, so asset.id==BTC becomes asset.id=="BTC".
var filterBareRHS = regexp.MustCompile(`^(.+)==([a-zA-Z0-9_\-]+)$`)

func normalizeFilter(filter string) string {
	if filter == "" {
		return filter
	}
	m := filterBareRHS.FindStringSubmatch(strings.TrimSpace(filter))
	if m == nil {
		return filter
	}
	// RHS is a bare word (no quotes); wrap in double quotes for CEL string literal
	return m[1] + `=="` + m[2] + `"`
}

var assetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available assets",
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKeyFlag, _ := cmd.Flags().GetString("api-key")
		apiKey, err := api.RequireAPIKey(apiKeyFlag)
		if err != nil {
			return err
		}
		client := api.NewClient(apiKey)

		filter, _ := cmd.Flags().GetString("filter")
		filter = normalizeFilter(filter)
		pruneFlag, _ := cmd.Flags().GetString("prune")

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			params := map[string]string{}
			if filter != "" {
				params["filter"] = filter
			}
			u, err := client.BuildURL("/v1/metadata/assets", params, nil)
			if err != nil {
				return err
			}
			redacted, _ := api.RedactAPIKeyFromURL(u)
			fmt.Println(redacted)
			return nil
		}

		assets, err := client.ListAssets(cmd.Context(), filter)
		if err != nil {
			return err
		}

		var data interface{} = assets
		if pruneFlag != "" {
			fields := strings.Split(pruneFlag, ",")
			data = api.PruneAssets(assets, fields)
		}

		format, _ := cmd.Flags().GetString("output")
		return output.Print(format, data)
	},
}

func init() {
	assetListCmd.Flags().String("filter", "", "CEL filter expression")
	assetListCmd.Flags().StringP("prune", "p", "", "Comma-separated list of fields to keep; output is an array of objects with only those fields (e.g. id,symbol)")
}

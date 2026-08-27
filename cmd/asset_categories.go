package cmd

import (
	"github.com/spf13/cobra"
)

var assetCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "List all asset categories",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNamesListCommand(cmd, "/v1/metadata/assets/categories", true)
	},
}

func init() {
	assetCategoriesCmd.Flags().String("filter", "", "CEL filter expression")
}

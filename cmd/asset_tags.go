package cmd

import (
	"github.com/spf13/cobra"
)

var assetTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "List all asset semantic tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNamesListCommand(cmd, "/v1/metadata/assets/tags", true)
	},
}

func init() {
	assetTagsCmd.Flags().String("filter", "", "CEL filter expression")
}

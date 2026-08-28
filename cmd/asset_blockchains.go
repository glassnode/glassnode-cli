package cmd

import (
	"github.com/spf13/cobra"
)

var assetBlockchainsCmd = &cobra.Command{
	Use:   "blockchains",
	Short: "List all blockchains on which at least one asset is supported",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNamesListCommand(cmd, "/v1/metadata/assets/blockchains", true)
	},
}

func init() {
	assetBlockchainsCmd.Flags().String("filter", "", "CEL filter expression")
}

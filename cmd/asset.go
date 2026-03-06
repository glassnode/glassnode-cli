package cmd

import "github.com/spf13/cobra"

var assetCmd = &cobra.Command{
	Use:   "asset",
	Short: "Query assets",
}

func init() {
	assetCmd.AddCommand(assetListCmd)
	assetCmd.AddCommand(assetDescribeCmd)
}

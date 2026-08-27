package cmd

import (
	"github.com/spf13/cobra"
)

var metricTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "List all tags used across metric metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNamesListCommand(cmd, "/v1/metadata/tags", false)
	},
}

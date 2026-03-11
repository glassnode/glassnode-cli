package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gn",
	Short: "Glassnode API command-line interface",
	SilenceUsage: true, // don't print help when a subcommand returns an error (e.g. API errors)
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// No setup required; API key is resolved via flag/env/config in each command.
		return nil
	},
}

func init() {
	rootCmd.Version = "dev"
	rootCmd.PersistentFlags().String("api-key", "", "API key override")
	rootCmd.PersistentFlags().StringP("output", "o", "json", "output format: json, csv, table")
	rootCmd.PersistentFlags().Bool("dry-run", false, "print the request URL without executing")
	rootCmd.PersistentFlags().String("timestamp-format", "humanized", "timestamp format: unix, humanized, or Go layout (e.g. 2006-01-02 15:04:05); applies to table/csv output")
	rootCmd.SetVersionTemplate("gn version {{.Version}}\n")

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(metricCmd)
	rootCmd.AddCommand(assetCmd)
}

func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

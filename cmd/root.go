package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var versionStr = "dev"

func SetVersion(s string) {
	versionStr = s
}

var rootCmd = &cobra.Command{
	Use:   "gn",
	Short: "Glassnode API command-line interface",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// No setup required; API key is resolved via flag/env/config in each command.
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("api-key", "", "API key override")
	rootCmd.PersistentFlags().StringP("output", "o", "json", "output format: json, csv, table")
	rootCmd.PersistentFlags().Bool("dry-run", false, "print the request URL without executing")
	rootCmd.PersistentFlags().String("timestamp-format", "humanized", "timestamp format: unix, humanized, or Go layout (e.g. 2006-01-02 15:04:05); applies to table/csv output")
	rootCmd.Version = versionStr
	rootCmd.SetVersionTemplate("gn version {{.Version}}\n")

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(metricCmd)
	rootCmd.AddCommand(assetCmd)
}

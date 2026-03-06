package cmd

import "github.com/spf13/cobra"

var metricCmd = &cobra.Command{
	Use:   "metric",
	Short: "Query metrics data",
}

func init() {
	metricCmd.AddCommand(metricListCmd)
	metricCmd.AddCommand(metricDescribeCmd)
	metricCmd.AddCommand(metricGetCmd)
}

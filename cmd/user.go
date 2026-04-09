package cmd

import "github.com/spf13/cobra"

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User endpoints",
}

func init() {
	userCmd.AddCommand(creditsCmd)
}

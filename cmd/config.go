package cmd

import (
	"fmt"

	"github.com/glassnode/gn/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long:  configHelpLong(),
}

func configHelpLong() string {
	const header = "Manage CLI configuration.\n\nValid keys:\n"
	s := header
	for _, h := range config.KeyHelp() {
		s += fmt.Sprintf("  %-12s %s\n", h.Key, h.Description)
	}
	s += "\nTo see all current values, run: gn config get all\n"
	return s
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}

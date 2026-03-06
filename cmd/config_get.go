package cmd

import (
	"fmt"

	"github.com/glassnode/gn/internal/config"
	"github.com/spf13/cobra"
)

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value (use 'all' to show all)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == "all" {
			values, err := config.GetAll()
			if err != nil {
				return err
			}
			for k, v := range values {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, v)
				if err != nil {
					return err
				}
			}
			return nil
		}

		value, err := config.Get(args[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), value)
		if err != nil {
			return err
		}
		return nil
	},
}

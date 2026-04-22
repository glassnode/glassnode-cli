package cmd

import (
	"fmt"

	"github.com/glassnode/glassnode-cli/internal/config"
	"github.com/spf13/cobra"
)

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value (use 'all' to show all). Sensitive values are masked.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == "all" {
			values, err := config.GetAll()
			if err != nil {
				return err
			}
			for k, v := range values {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, config.Mask(k, v)); err != nil {
					return err
				}
			}
			return nil
		}

		value, err := config.Get(args[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), config.Mask(args[0], value))
		return err
	},
}

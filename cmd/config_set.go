package cmd

import (
	"fmt"
	"strings"

	"github.com/glassnode/gn/internal/config"
	"github.com/spf13/cobra"
)

var configSetCmd = &cobra.Command{
	Use:   "set key=value",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parts := strings.SplitN(args[0], "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("expected key=value format, got %q", args[0])
		}
		key, value := parts[0], parts[1]
		if err := config.Set(key, value); err != nil {
			return err
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Set %s=%s\n", key, value)
		return err
	},
}

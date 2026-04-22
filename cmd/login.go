package cmd

import (
	"fmt"

	"github.com/glassnode/glassnode-cli/internal/oauth"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in with your Glassnode account in the browser (OAuth 2.0 with PKCE)",
	Long:  `Opens your browser to complete sign-in. Tokens are stored in ~/.gn/config.yaml.`,
	RunE:  runLogin,
}

func runLogin(cmd *cobra.Command, _ []string) error {
	if err := oauth.RunLogin(cmd.Context()); err != nil {
		return err
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), "Signed in successfully.")
	return err
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

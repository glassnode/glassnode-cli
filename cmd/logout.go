package cmd

import (
	"errors"
	"fmt"

	"github.com/glassnode/glassnode-cli/internal/oauth"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke the stored OAuth tokens and clear them from ~/.gn/config.yaml",
	Long:  `Revokes the OAuth refresh and access tokens. Local tokens are always cleared even if the remote revocation failed.`,
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, _ []string) error {
	err := oauth.Logout(cmd.Context())
	var remoteErr *oauth.RemoteRevokeError
	if errors.As(err, &remoteErr) {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"warning: remote token revocation failed but local tokens were removed: %v\n",
			remoteErr)
	} else if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Signed out.")
	return err
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

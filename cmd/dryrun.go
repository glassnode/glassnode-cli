package cmd

import (
	"fmt"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/spf13/cobra"
)

// printDryRun prints the request URL that --dry-run would send. The credential
// is sent in a header and is not part of the URL, which the note on stderr
// says, so the URL on stdout stays usable in scripts.
func printDryRun(cmd *cobra.Command, client *api.Client, url string) {
	fmt.Fprintln(cmd.OutOrStdout(), url)
	fmt.Fprintf(cmd.ErrOrStderr(), "# credentials are sent in the %s header, not in the URL\n", client.AuthHeader())
}

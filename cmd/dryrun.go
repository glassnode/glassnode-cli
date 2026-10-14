package cmd

import (
	"fmt"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/spf13/cobra"
)

// printDryRun prints the endpoint and parameters --dry-run would send. The URL
// is assembled from the flags rather than by the SDK, so it is a preview, not
// the request byte for byte: the SDK appends f=json to metric requests and
// validates parameters only when sending. The credential is sent in a header
// and is not part of the URL, which the note on stderr says, so the URL on
// stdout stays usable in scripts. A failed write is returned: the URLs are the
// command's whole output.
func printDryRun(cmd *cobra.Command, client *api.Client, urls ...string) error {
	for _, url := range urls {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), url); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(cmd.ErrOrStderr(), "# credentials are sent in the %s, not in the URL\n", client.AuthHeader())
	return err
}

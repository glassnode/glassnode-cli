package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/spf13/cobra"
)

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

// The URLs are a dry run's whole output: a failed write (closed pipe, full
// disk) must fail the command rather than exit 0 with nothing printed.
func TestPrintDryRun_ReturnsWriteError(t *testing.T) {
	writeErr := errors.New("broken pipe")
	cmd := &cobra.Command{}
	cmd.SetOut(failingWriter{writeErr})
	cmd.SetErr(&bytes.Buffer{})
	if err := printDryRun(cmd, api.NewClient("k", ""), "https://api.glassnode.com/v1/x"); !errors.Is(err, writeErr) {
		t.Fatalf("err = %v, want the write error", err)
	}
}

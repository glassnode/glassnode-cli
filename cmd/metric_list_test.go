package cmd

import (
	"strings"
	"testing"
)

// The removed --format filter fails with the reason and the replacement for
// one release, instead of cobra's "unknown shorthand flag".
func TestMetricList_RemovedFormatFlagExplains(t *testing.T) {
	for _, args := range [][]string{{"-f", "csv"}, {"--format", "json"}} {
		_, stderr, err := runCLI(t, []string{"HOME=" + t.TempDir()}, append([]string{"metric", "list", "--api-key", "k"}, args...)...)
		if err == nil {
			t.Fatalf("%v: expected a failure", args)
		}
		if !strings.Contains(stderr, "--format was removed") || !strings.Contains(stderr, "-o csv") || strings.Contains(stderr, "unknown shorthand") {
			t.Errorf("%v: stderr = %s", args, stderr)
		}
	}
}

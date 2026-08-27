package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// runCLINames executes the CLI binary via `go run .` against the given base URL,
// appending an api-key flag so no config/env credentials are required.
func runCLINames(t *testing.T, baseURL string, args ...string) (string, string, error) {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(f), "..")
	cmdArgs := append([]string{"run", "."}, append(args, "--api-key", "test-key")...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GLASSNODE_BASE_URL="+baseURL)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	return outBuf.String(), errBuf.String(), runErr
}

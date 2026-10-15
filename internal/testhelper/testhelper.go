package testhelper

import (
	"testing"
)

// WithTempHome runs fn with HOME and USERPROFILE set to a temporary directory,
// then restores the original values. Use this to isolate config or home-based
// paths in tests. On Unix, UserHomeDir uses HOME; on Windows it uses USERPROFILE.
func WithTempHome(t *testing.T, fn func()) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	fn()
}

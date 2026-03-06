package testhelper

import (
	"os"
	"testing"
)

// WithTempHome runs fn with HOME and USERPROFILE set to a temporary directory,
// then restores the original values. Use this to isolate config or home-based
// paths in tests. On Unix, UserHomeDir uses HOME; on Windows it uses USERPROFILE.
func WithTempHome(t *testing.T, fn func()) {
	t.Helper()
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", origHome)
		os.Setenv("USERPROFILE", origUserProfile)
	})
	fn()
}

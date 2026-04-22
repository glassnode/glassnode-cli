package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/glassnode/glassnode-cli/internal/testhelper"
)

func withTempHome(t *testing.T, fn func()) {
	testhelper.WithTempHome(t, fn)
}

func TestSaveOAuthSession(t *testing.T) {
	withTempHome(t, func() {
		exp := time.Date(2030, 1, 2, 15, 4, 5, 0, time.UTC)
		if err := SaveOAuthSession(OAuthSession{
			AccessToken:  "at",
			RefreshToken: "rt",
			ExpiresAt:    exp,
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.OAuthAccessToken != "at" || cfg.OAuthRefreshToken != "rt" {
			t.Errorf("tokens not saved: %+v", cfg)
		}
		if cfg.OAuthExpiresAt != "2030-01-02T15:04:05Z" {
			t.Errorf("expires at %q", cfg.OAuthExpiresAt)
		}
	})
}

func TestClearOAuthSession(t *testing.T) {
	withTempHome(t, func() {
		if err := SaveOAuthSession(OAuthSession{
			AccessToken:  "at",
			RefreshToken: "rt",
			ExpiresAt:    time.Now().UTC().Add(time.Hour),
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}
		if err := ClearOAuthSession(); err != nil {
			t.Fatalf("ClearOAuthSession: %v", err)
		}
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.OAuthAccessToken != "" || cfg.OAuthRefreshToken != "" || cfg.OAuthExpiresAt != "" {
			t.Errorf("OAuth fields not cleared: %+v", cfg)
		}
	})
}

func TestSet_RejectsOAuthKeys(t *testing.T) {
	withTempHome(t, func() {
		for _, k := range []string{"oauth-access-token", "oauth-refresh-token", "oauth-expires-at"} {
			if err := Set(k, "x"); err == nil {
				t.Errorf("Set(%q) should be rejected as read-only", k)
			}
		}
	})
}

func TestMask(t *testing.T) {
	cases := []struct {
		key, in, want string
	}{
		{"api-key", "", ""},
		{"api-key", "abcd", "*****"},
		{"api-key", "abcdefghij", "*****-ghij"},
		{"oauth-refresh-token", "abcdefghij", "*****-ghij"},
		{"oauth-expires-at", "2030-01-02T15:04:05Z", "2030-01-02T15:04:05Z"},
		{"api-key", "1234567890abcdef", "*****-cdef"},
	}
	for _, c := range cases {
		got := Mask(c.key, c.in)
		if got != c.want {
			t.Errorf("Mask(%q,%q) = %q, want %q", c.key, c.in, got, c.want)
		}
	}
}

func TestSetAndGet(t *testing.T) {
	withTempHome(t, func() {
		err := Set("api-key", "test-key-123")
		if err != nil {
			t.Fatalf("Set: %v", err)
		}
		got, err := Get("api-key")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got != "test-key-123" {
			t.Errorf("got %q, want test-key-123", got)
		}
	})
}

func TestGetUnknownKey(t *testing.T) {
	withTempHome(t, func() {
		_, err := Get("unknown-key")
		if err == nil {
			t.Error("expected error for unknown key")
		}
	})
}

func TestSetUnknownKey(t *testing.T) {
	withTempHome(t, func() {
		err := Set("unknown-key", "value")
		if err == nil {
			t.Error("expected error for unknown key")
		}
		if err != nil && !strings.Contains(err.Error(), "unknown") {
			t.Errorf("expected 'unknown' in error, got %v", err)
		}
	})
}

func TestGetAll(t *testing.T) {
	withTempHome(t, func() {
		err := Set("api-key", "all-test-key")
		if err != nil {
			t.Fatalf("Set: %v", err)
		}
		got, err := GetAll()
		if err != nil {
			t.Fatalf("GetAll: %v", err)
		}
		if got["api-key"] != "all-test-key" {
			t.Errorf("got %q, want all-test-key", got["api-key"])
		}
	})
}

func TestLoadEmptyWhenFileNotExists(t *testing.T) {
	withTempHome(t, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.ApiKey != "" {
			t.Errorf("got ApiKey %q, want empty", cfg.ApiKey)
		}
	})
}

func TestLoadAndSaveRoundtrip(t *testing.T) {
	withTempHome(t, func() {
		cfg := &Config{ApiKey: "roundtrip-key"}
		err := Save(cfg)
		if err != nil {
			t.Fatalf("Save: %v", err)
		}
		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if loaded.ApiKey != "roundtrip-key" {
			t.Errorf("got ApiKey %q, want roundtrip-key", loaded.ApiKey)
		}
	})
}

func TestLoadInvalidYAML(t *testing.T) {
	withTempHome(t, func() {
		home, _ := os.UserHomeDir()
		dir := filepath.Join(home, ".gn")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte("invalid: yaml: [unclosed"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		_, err := Load()
		if err == nil {
			t.Error("expected error for invalid YAML")
		}
	})
}

func TestLoad_UnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 not reliable on Windows")
	}
	withTempHome(t, func() {
		home, _ := os.UserHomeDir()
		dir := filepath.Join(home, ".gn")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte("api-key: x"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
		_, err := Load()
		if err == nil {
			t.Error("expected error when config file is not readable")
		}
	})
}

func TestSave_WhenConfigDirIsAFile(t *testing.T) {
	withTempHome(t, func() {
		home, _ := os.UserHomeDir()
		gnPath := filepath.Join(home, ".gn")
		if err := os.WriteFile(gnPath, []byte(""), 0o644); err != nil {
			t.Fatalf("create .gn as file: %v", err)
		}
		err := Save(&Config{ApiKey: "key"})
		if err == nil {
			t.Error("expected error when .gn is a file (cannot create config.yaml inside it)")
		}
	})
}

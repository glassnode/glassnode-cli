package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ApiKey            string `yaml:"api-key"`
	OAuthAccessToken  string `yaml:"oauth-access-token,omitempty"`
	OAuthRefreshToken string `yaml:"oauth-refresh-token,omitempty"`
	OAuthExpiresAt    string `yaml:"oauth-expires-at,omitempty"`
}

type OAuthSession struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// sensitiveKeys are config values that must never be printed in cleartext via Get/GetAll display.
var sensitiveKeys = map[string]struct{}{
	"api-key":             {},
	"oauth-access-token":  {},
	"oauth-refresh-token": {},
}

// mutableKeys are config keys a user may change via `gn config set`. OAuth fields are set only
// by `gn login` / token refresh and must not be settable by hand to avoid exfiltration vectors
var mutableKeys = map[string]struct{}{
	"api-key": {},
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	return filepath.Join(home, ".gn"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, "config.yaml")
	return os.WriteFile(path, data, 0o600)
}

func SaveOAuthSession(s OAuthSession) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	cfg.OAuthAccessToken = s.AccessToken
	cfg.OAuthRefreshToken = s.RefreshToken
	if s.ExpiresAt.IsZero() {
		cfg.OAuthExpiresAt = ""
	} else {
		cfg.OAuthExpiresAt = s.ExpiresAt.UTC().Format(time.RFC3339)
	}

	return Save(cfg)
}

func ClearOAuthSession() error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	cfg.OAuthAccessToken = ""
	cfg.OAuthRefreshToken = ""
	cfg.OAuthExpiresAt = ""
	return Save(cfg)
}

func Get(key string) (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}

	val, ok := fieldByYAMLTag(cfg, key)
	if !ok {
		return "", fmt.Errorf("unknown config key: %s", key)
	}

	return val, nil
}

func Set(key, value string) error {
	if _, ok := mutableKeys[key]; !ok {
		// Distinguish "unknown" from "read-only" so users don't get a misleading error.
		if _, known := fieldByYAMLTag(&Config{}, key); known {
			return fmt.Errorf("config key %q is read-only (managed by gn login/logout)", key)
		}
		return fmt.Errorf("unknown config key: %s", key)
	}

	cfg, err := Load()
	if err != nil {
		return err
	}
	if !setFieldByYAMLTag(cfg, key, value) {
		return fmt.Errorf("unknown config key: %s", key)
	}
	return Save(cfg)
}

func GetAll() (map[string]string, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("yaml")
		if tag != "" {
			name, _, _ := strings.Cut(tag, ",")
			result[name] = v.Field(i).String()
		}
	}
	return result, nil
}

// Mask returns a display-safe representation for the given key's value. Sensitive values are
// shown as "*****-<last4>" so the user can still verify which credential is stored without
// exposing it in the terminal.
func Mask(key, value string) string {
	if _, ok := sensitiveKeys[key]; !ok {
		return value
	}
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "*****"
	}
	return "*****-" + value[len(value)-4:]
}

func fieldByYAMLTag(cfg *Config, key string) (string, bool) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		if name == key {
			return v.Field(i).String(), true
		}
	}
	return "", false
}

func setFieldByYAMLTag(cfg *Config, key, value string) bool {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		if name == key {
			v.Field(i).SetString(value)
			return true
		}
	}
	return false
}

// KeyHelp returns valid config keys and their descriptions for help output.
func KeyHelp() []struct{ Key, Description string } {
	return []struct{ Key, Description string }{
		{"api-key", "Glassnode API key (alternative to OAuth; used when no valid OAuth token)"},
		{"oauth-access-token", "OAuth2 access token (read-only, set by gn login/refresh)"},
		{"oauth-refresh-token", "OAuth2 refresh token (read-only, set by gn login/refresh)"},
		{"oauth-expires-at", "OAuth2 access token expiry in RFC3339 (read-only, set by gn login/refresh)"},
	}
}

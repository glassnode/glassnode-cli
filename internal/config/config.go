package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ApiKey string `yaml:"api-key"`
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, "config.yaml")
	// 0o600: owner read/write only. On Windows, permission bits are not applied
	// the same way, but the file is still created with default user-only access.
	return os.WriteFile(path, data, 0o600)
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
			result[tag] = v.Field(i).String()
		}
	}
	return result, nil
}

func fieldByYAMLTag(cfg *Config, key string) (string, bool) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("yaml") == key {
			return v.Field(i).String(), true
		}
	}
	return "", false
}

func setFieldByYAMLTag(cfg *Config, key, value string) bool {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("yaml") == key {
			v.Field(i).SetString(value)
			return true
		}
	}
	return false
}

// KeyHelp returns valid config keys and their descriptions for help output.
func KeyHelp() []struct{ Key, Description string } {
	return []struct{ Key, Description string }{
		{"api-key", "Glassnode API key (required for API access)"},
	}
}

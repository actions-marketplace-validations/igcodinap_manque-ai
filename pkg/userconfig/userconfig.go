// Package userconfig manages persistent user configuration stored in ~/.manque-ai/config.yaml
package userconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	configDir  = ".manque-ai"
	configFile = "config.yaml"
)

// UserConfig holds user-specific configuration
type UserConfig struct {
	Provider string `yaml:"provider,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

// ConfigPath returns the full path to the config file
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, configDir, configFile), nil
}

// Load reads the config from ~/.manque-ai/config.yaml
func Load() (*UserConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UserConfig{}, nil // Return empty config if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config UserConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// Save writes the config to ~/.manque-ai/config.yaml
func (c *UserConfig) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Clear removes the config file
func Clear() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already cleared
		}
		return fmt.Errorf("failed to remove config: %w", err)
	}

	return nil
}

// Set updates a single config value
func (c *UserConfig) Set(key, value string) error {
	switch key {
	case "LLM_PROVIDER", "provider":
		c.Provider = value
	case "LLM_API_KEY", "api_key":
		c.APIKey = value
	case "LLM_MODEL", "model":
		c.Model = value
	default:
		return fmt.Errorf("unknown config key: %s (valid keys: provider, api_key, model)", key)
	}
	return nil
}

// MaskedAPIKey returns the API key with most characters masked
func (c *UserConfig) MaskedAPIKey() string {
	if c.APIKey == "" {
		return "(not set)"
	}
	if len(c.APIKey) <= 8 {
		return "****"
	}
	return c.APIKey[:4] + "****" + c.APIKey[len(c.APIKey)-4:]
}

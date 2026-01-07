package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FileConfig represents the .manque.yml configuration file
type FileConfig struct {
	Version int `yaml:"version"`

	Review ReviewConfig `yaml:"review"`
	Ignore []string     `yaml:"ignore"`
	Rules  []PathRule   `yaml:"rules"`
}

// ReviewConfig contains review-specific settings
type ReviewConfig struct {
	AutoApproveThreshold int  `yaml:"auto_approve_threshold"`
	BlockOnCritical      bool `yaml:"block_on_critical"`
}

// PathRule defines rules for specific file paths
type PathRule struct {
	Path             string `yaml:"path"`
	SeverityOverride string `yaml:"severity_override,omitempty"` // "suggestion", "warning", "critical"
	ExtraRules       string `yaml:"extra_rules,omitempty"`       // Additional rules as text
	Ignore           bool   `yaml:"ignore,omitempty"`            // Ignore files matching this path
}

// DefaultConfig returns the default configuration
func DefaultConfig() *FileConfig {
	return &FileConfig{
		Version: 1,
		Review: ReviewConfig{
			AutoApproveThreshold: 90,
			BlockOnCritical:      true,
		},
		Ignore: []string{
			"**/*.lock",
			"**/vendor/**",
			"**/node_modules/**",
			"**/*.generated.go",
			"**/dist/**",
			"**/*.min.js",
			"**/*.min.css",
		},
		Rules: []PathRule{},
	}
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// FindConfigFile searches for a .manque.yml file in the current directory and parent directories
func FindConfigFile(startDir string) (string, error) {
	configNames := []string{".manque.yml", ".manque.yaml", "manque.yml", "manque.yaml"}

	dir := startDir
	for {
		for _, name := range configNames {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

// LoadFromDirectory finds and loads the config file from a directory
func LoadFromDirectory(dir string) (*FileConfig, error) {
	path, err := FindConfigFile(dir)
	if err != nil {
		// No config file found, return defaults
		return DefaultConfig(), nil
	}

	return LoadFromFile(path)
}

// ShouldIgnoreFile checks if a file should be ignored based on ignore patterns
func (c *FileConfig) ShouldIgnoreFile(filename string) bool {
	for _, pattern := range c.Ignore {
		matched, err := filepath.Match(pattern, filename)
		if err == nil && matched {
			return true
		}
		// Also try matching with the base name
		matched, err = filepath.Match(pattern, filepath.Base(filename))
		if err == nil && matched {
			return true
		}
	}

	// Check path-specific ignore rules
	for _, rule := range c.Rules {
		if rule.Ignore {
			matched, err := filepath.Match(rule.Path, filename)
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}

// GetRulesForFile returns the extra rules that apply to a specific file
func (c *FileConfig) GetRulesForFile(filename string) string {
	for _, rule := range c.Rules {
		matched, err := filepath.Match(rule.Path, filename)
		if err == nil && matched && rule.ExtraRules != "" {
			return rule.ExtraRules
		}
	}
	return ""
}

// GetSeverityOverrideForFile returns the severity override for a specific file
func (c *FileConfig) GetSeverityOverrideForFile(filename string) string {
	for _, rule := range c.Rules {
		matched, err := filepath.Match(rule.Path, filename)
		if err == nil && matched && rule.SeverityOverride != "" {
			return rule.SeverityOverride
		}
	}
	return ""
}

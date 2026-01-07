package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// GitHub settings
	GitHubToken  string // Optional for local
	GitHubAPIURL string

	// LLM settings
	LLMAPIKey   string `validate:"required"`
	LLMModel    string
	LLMProvider string
	LLMBaseURL  string

	// Review settings
	StyleGuideRules string

	// CLI/Action context
	PRNumber        int
	Repository      string
	GitHubEventPath string

	// Output settings
	UpdatePRTitle bool
	UpdatePRBody  bool

	// Review action settings
	AutoApproveThreshold int  // Score threshold for auto-approve (default: 90)
	BlockOnCritical      bool // Request changes when critical issues found (default: true)
	
	// CLI settings
	Debug bool
	SkipGitHubValidation bool
	
	// Discovery settings
	AutoDiscoverPractices bool   // Enable auto-discovery of repo practices (default: true)
	DiscoveredPractices   string // Content discovered from repo practice files

	// File-based config
	IgnorePatterns []string            // Patterns to ignore during review
	PathRules      map[string]PathRule // Path-specific rules
}

// PathRule defines rules for specific file paths (mirrored from pkg/config)
type PathRule struct {
	SeverityOverride string
	ExtraRules       string
	Ignore           bool
}

func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Load user config file (~/.manque-ai/config.yaml) for defaults
	userCfg, _ := loadUserConfig()

	config := &Config{
		GitHubToken:           getEnvWithFallbacks("GH_TOKEN", "GITHUB_TOKEN"),
		GitHubAPIURL:          getEnvWithDefault("GITHUB_API_URL", "https://api.github.com"),
		LLMAPIKey:             getEnvOrUserConfig("LLM_API_KEY", userCfg.APIKey, getEnvWithFallbacks("OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY", "OPENROUTER_API_KEY")),
		LLMModel:              getEnvOrUserConfig("LLM_MODEL", userCfg.Model, "mistralai/mistral-7b-instruct:free"),
		LLMProvider:           getEnvOrUserConfig("LLM_PROVIDER", userCfg.Provider, "openrouter"),
		LLMBaseURL:            getEnvWithDefault("LLM_BASE_URL", ""),
		StyleGuideRules:       getEnvWithDefault("STYLE_GUIDE_RULES", ""),
		GitHubEventPath:       getEnvWithDefault("GITHUB_EVENT_PATH", ""),
		UpdatePRTitle:         getEnvWithDefault("UPDATE_PR_TITLE", "true") == "true",
		UpdatePRBody:          getEnvWithDefault("UPDATE_PR_BODY", "true") == "true",
		AutoApproveThreshold:  getEnvAsInt("AUTO_APPROVE_THRESHOLD", 90),
		BlockOnCritical:       getEnvWithDefault("BLOCK_ON_CRITICAL", "true") == "true",
		AutoDiscoverPractices: getEnvWithDefault("AUTO_DISCOVER_PRACTICES", "true") == "true",
	}

	return config, nil
}

func (c *Config) Validate() error {
	if !c.SkipGitHubValidation && c.GitHubToken == "" {
		return fmt.Errorf("GitHub token is required (set GH_TOKEN or GITHUB_TOKEN)")
	}
	if c.LLMAPIKey == "" {
		return fmt.Errorf("LLM API key is required (set LLM_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY, GOOGLE_API_KEY, or OPENROUTER_API_KEY)")
	}

	validProviders := map[string]bool{
		"openai":     true,
		"anthropic":  true,
		"google":     true,
		"openrouter": true,
	}
	if !validProviders[c.LLMProvider] {
		return fmt.Errorf("invalid LLM_PROVIDER: %s. Must be one of: openai, anthropic, google, openrouter", c.LLMProvider)
	}

	return nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvWithFallbacks checks multiple environment variable names in order
// and returns the first non-empty value found
func getEnvWithFallbacks(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

// getEnvAsInt returns an environment variable as an integer, or the default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// userConfigData holds values loaded from ~/.manque-ai/config.yaml
type userConfigData struct {
	Provider string
	APIKey   string
	Model    string
}

// loadUserConfig reads user config from ~/.manque-ai/config.yaml
func loadUserConfig() (userConfigData, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return userConfigData{}, err
	}

	configPath := filepath.Join(home, ".manque-ai", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return userConfigData{}, err
	}

	// Simple YAML parsing for our specific fields
	cfg := userConfigData{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "provider:") {
			cfg.Provider = strings.TrimSpace(strings.TrimPrefix(line, "provider:"))
		} else if strings.HasPrefix(line, "api_key:") {
			cfg.APIKey = strings.TrimSpace(strings.TrimPrefix(line, "api_key:"))
		} else if strings.HasPrefix(line, "model:") {
			cfg.Model = strings.TrimSpace(strings.TrimPrefix(line, "model:"))
		}
	}
	return cfg, nil
}

// getEnvOrUserConfig returns env var value, then user config value, then fallback
func getEnvOrUserConfig(envKey, userConfigValue, fallback string) string {
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	if userConfigValue != "" {
		return userConfigValue
	}
	return fallback
}

// ShouldIgnoreFile checks if a file should be ignored based on ignore patterns
func (c *Config) ShouldIgnoreFile(filename string) bool {
	for _, pattern := range c.IgnorePatterns {
		matched, err := matchPattern(pattern, filename)
		if err == nil && matched {
			return true
		}
	}

	// Check path-specific ignore rules
	for path, rule := range c.PathRules {
		if rule.Ignore {
			matched, err := matchPattern(path, filename)
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}

// GetExtraRulesForFile returns extra rules that apply to a specific file
func (c *Config) GetExtraRulesForFile(filename string) string {
	for path, rule := range c.PathRules {
		matched, err := matchPattern(path, filename)
		if err == nil && matched && rule.ExtraRules != "" {
			return rule.ExtraRules
		}
	}
	return ""
}

// matchPattern is a helper to match glob-like patterns against filenames
func matchPattern(pattern, filename string) (bool, error) {
	// Try direct match
	if matched, err := filepath.Match(pattern, filename); err == nil && matched {
		return true, nil
	}
	// Try matching against base name
	if matched, err := filepath.Match(pattern, filepath.Base(filename)); err == nil && matched {
		return true, nil
	}
	return false, nil
}

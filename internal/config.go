package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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

	config := &Config{
		GitHubToken:           getEnvWithFallbacks("GH_TOKEN", "GITHUB_TOKEN"),
		GitHubAPIURL:          getEnvWithDefault("GITHUB_API_URL", "https://api.github.com"),
		LLMAPIKey:             getEnvWithFallbacks("LLM_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY", "OPENROUTER_API_KEY"),
		LLMModel:              getEnvWithDefault("LLM_MODEL", "gpt-4o"),
		LLMProvider:           getEnvWithDefault("LLM_PROVIDER", "openai"),
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

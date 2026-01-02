package internal

import (
	"fmt"
	"os"
)

type Config struct {
	// GitHub settings
	GitHubToken  string `validate:"required"`
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
}

func LoadConfig() (*Config, error) {
	config := &Config{
		GitHubToken:     getEnvWithFallbacks("GH_TOKEN", "GITHUB_TOKEN"),
		GitHubAPIURL:    getEnvWithDefault("GITHUB_API_URL", "https://api.github.com"),
		LLMAPIKey:       getEnvWithFallbacks("LLM_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY", "OPENROUTER_API_KEY"),
		LLMModel:        getEnvWithDefault("LLM_MODEL", "gpt-4o"),
		LLMProvider:     getEnvWithDefault("LLM_PROVIDER", "openai"),
		LLMBaseURL:      getEnvWithDefault("LLM_BASE_URL", ""),
		StyleGuideRules: getEnvWithDefault("STYLE_GUIDE_RULES", ""),
		GitHubEventPath: getEnvWithDefault("GITHUB_EVENT_PATH", ""),
		UpdatePRTitle:   getEnvWithDefault("UPDATE_PR_TITLE", "true") == "true",
		UpdatePRBody:    getEnvWithDefault("UPDATE_PR_BODY", "true") == "true",
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

func (c *Config) validate() error {
	if c.GitHubToken == "" {
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

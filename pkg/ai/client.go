package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
}

func NewClient(config Config) (Client, error) {
	switch strings.ToLower(config.Provider) {
	case "openai":
		return NewOpenAIClient(config), nil
	case "anthropic":
		return NewAnthropicClient(config), nil
	case "google":
		return NewGoogleClient(config), nil
	case "openrouter":
		return NewOpenRouterClient(config), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}
}

// Base HTTP client for LLM providers
type BaseClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
	baseURL    string
	headers    map[string]string
}

func NewBaseClient(apiKey, model, baseURL string, headers map[string]string) *BaseClient {
	return &BaseClient{
		httpClient: &http.Client{Timeout: 600 * time.Second},
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		headers:    headers,
	}
}

func (c *BaseClient) makeRequest(endpoint string, payload interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")

	// Set custom headers
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func extractJSONFromResponse(content string) string {
	// Try to find JSON content between ```json and ``` markers
	if start := strings.Index(content, "```json"); start != -1 {
		start += 7
		if end := strings.Index(content[start:], "```"); end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}

	// Try to find JSON content between { and } markers
	start := strings.Index(content, "{")
	if start == -1 {
		return content
	}

	braceCount := 0
	for i, char := range content[start:] {
		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 {
				return content[start : start+i+1]
			}
		}
	}

	return content
}

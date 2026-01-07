package ai

import (
	"encoding/json"
	"fmt"
)

type AnthropicClient struct {
	*BaseClient
}

type AnthropicRequest struct {
	Model     string                 `json:"model"`
	MaxTokens int                    `json:"max_tokens"`
	Messages  []AnthropicMessage     `json:"messages"`
	System    string                 `json:"system,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewAnthropicClient(config Config) *AnthropicClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	
	headers := map[string]string{
		"x-api-key":         config.APIKey,
		"anthropic-version": "2023-06-01",
	}
	
	return &AnthropicClient{
		BaseClient: NewBaseClient(config.APIKey, config.Model, baseURL, headers),
	}
}

func (c *AnthropicClient) GeneratePRSummary(prTitle, prDescription, diff string) (*PRSummary, error) {
	systemPrompt := GetPRSummaryPrompt()
	userPrompt := fmt.Sprintf("PR Title: %s\n\nPR Description: %s\n\nGit Diff:\n%s", prTitle, prDescription, diff)
	
	request := AnthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []AnthropicMessage{
			{Role: "user", Content: userPrompt},
		},
	}
	
	respBytes, err := c.makeRequest("/v1/messages", request)
	if err != nil {
		return nil, err
	}
	
	var response AnthropicResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}
	
	if len(response.Content) == 0 {
		return nil, fmt.Errorf("no response content returned")
	}
	
	content := extractJSONFromResponse(response.Content[0].Text)
	
	var summary PRSummary
	if err := json.Unmarshal([]byte(content), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse PR summary JSON: %w", err)
	}
	
	return &summary, nil
}

func (c *AnthropicClient) GenerateCodeReview(prTitle, prDescription, diff string) (*ReviewResult, error) {
	return c.GenerateCodeReviewWithStyleGuide(prTitle, prDescription, diff, "")
}

func (c *AnthropicClient) GenerateCodeReviewWithStyleGuide(prTitle, prDescription, diff, styleGuide string) (*ReviewResult, error) {
	var systemPrompt string
	if styleGuide != "" {
		systemPrompt = GetCodeReviewPromptWithStyleGuide(styleGuide)
	} else {
		systemPrompt = GetCodeReviewPrompt()
	}
	
	userPrompt := fmt.Sprintf("PR Title: %s\n\nPR Description: %s\n\nGit Diff:\n%s", prTitle, prDescription, diff)
	
	request := AnthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []AnthropicMessage{
			{Role: "user", Content: userPrompt},
		},
	}
	
	respBytes, err := c.makeRequest("/v1/messages", request)
	if err != nil {
		return nil, err
	}
	
	var response AnthropicResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}
	
	if len(response.Content) == 0 {
		return nil, fmt.Errorf("no response content returned")
	}
	
	content := extractJSONFromResponse(response.Content[0].Text)

	var review ReviewResult
	if err := json.Unmarshal([]byte(content), &review); err != nil {
		return nil, fmt.Errorf("failed to parse review JSON: %w", err)
	}

	return &review, nil
}

func (c *AnthropicClient) GenerateResponse(prompt string) (string, error) {
	request := AnthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		Messages: []AnthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	respBytes, err := c.makeRequest("/v1/messages", request)
	if err != nil {
		return "", err
	}

	var response AnthropicResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("API error: %s", response.Error.Message)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no response content returned")
	}

	return response.Content[0].Text, nil
}
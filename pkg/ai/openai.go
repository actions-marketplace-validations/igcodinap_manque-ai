package ai

import (
	"encoding/json"
	"fmt"
)

type OpenAIClient struct {
	*BaseClient
}

func NewOpenAIClient(config Config) *OpenAIClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	
	headers := map[string]string{
		"Authorization": "Bearer " + config.APIKey,
	}
	
	return &OpenAIClient{
		BaseClient: NewBaseClient(config.APIKey, config.Model, baseURL, headers),
	}
}

func (c *OpenAIClient) GeneratePRSummary(prTitle, prDescription, diff string) (*PRSummary, error) {
	systemPrompt := GetPRSummaryPrompt()
	userPrompt := fmt.Sprintf("PR Title: %s\n\nPR Description: %s\n\nGit Diff:\n%s", prTitle, prDescription, diff)
	
	request := ChatCompletionRequest{
		Model: c.model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: &[]float64{0.1}[0],
	}
	
	respBytes, err := c.makeRequest("/chat/completions", request)
	if err != nil {
		return nil, err
	}
	
	var response ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}
	
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}
	
	content := extractJSONFromResponse(response.Choices[0].Message.Content)
	
	var summary PRSummary
	if err := json.Unmarshal([]byte(content), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse PR summary JSON: %w", err)
	}
	
	return &summary, nil
}

func (c *OpenAIClient) GenerateCodeReview(prTitle, prDescription, diff string) (*ReviewResult, error) {
	return c.GenerateCodeReviewWithStyleGuide(prTitle, prDescription, diff, "")
}

func (c *OpenAIClient) GenerateCodeReviewWithStyleGuide(prTitle, prDescription, diff, styleGuide string) (*ReviewResult, error) {
	var systemPrompt string
	if styleGuide != "" {
		systemPrompt = GetCodeReviewPromptWithStyleGuide(styleGuide)
	} else {
		systemPrompt = GetCodeReviewPrompt()
	}
	
	userPrompt := fmt.Sprintf("PR Title: %s\n\nPR Description: %s\n\nGit Diff:\n%s", prTitle, prDescription, diff)
	
	request := ChatCompletionRequest{
		Model: c.model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: &[]float64{0.1}[0],
	}
	
	respBytes, err := c.makeRequest("/chat/completions", request)
	if err != nil {
		return nil, err
	}
	
	var response ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}
	
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}
	
	content := extractJSONFromResponse(response.Choices[0].Message.Content)

	var review ReviewResult
	if err := json.Unmarshal([]byte(content), &review); err != nil {
		return nil, fmt.Errorf("failed to parse review JSON: %w", err)
	}

	return &review, nil
}

func (c *OpenAIClient) GenerateResponse(prompt string) (string, error) {
	request := ChatCompletionRequest{
		Model: c.model,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: &[]float64{0.7}[0],
	}

	respBytes, err := c.makeRequest("/chat/completions", request)
	if err != nil {
		return "", err
	}

	var response ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("API error: %s", response.Error.Message)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return response.Choices[0].Message.Content, nil
}
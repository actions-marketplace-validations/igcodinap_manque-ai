package ai

import (
	"encoding/json"
	"fmt"
)

type GoogleClient struct {
	*BaseClient
}

type GoogleRequest struct {
	Contents         []GoogleContent `json:"contents"`
	SystemInstruction *GoogleContent  `json:"systemInstruction,omitempty"`
	GenerationConfig *GoogleGenConfig `json:"generationConfig,omitempty"`
}

type GoogleContent struct {
	Role  string        `json:"role"`
	Parts []GooglePart  `json:"parts"`
}

type GooglePart struct {
	Text string `json:"text"`
}

type GoogleGenConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
}

type GoogleResponse struct {
	Candidates []struct {
		Content struct {
			Parts []GooglePart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func NewGoogleClient(config Config) *GoogleClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	
	headers := map[string]string{}
	
	return &GoogleClient{
		BaseClient: NewBaseClient(config.APIKey, config.Model, baseURL, headers),
	}
}

func (c *GoogleClient) GeneratePRSummary(prTitle, prDescription, diff string) (*PRSummary, error) {
	systemPrompt := GetPRSummaryPrompt()
	userPrompt := fmt.Sprintf("PR Title: %s\n\nPR Description: %s\n\nGit Diff:\n%s", prTitle, prDescription, diff)
	
	request := GoogleRequest{
		SystemInstruction: &GoogleContent{
			Parts: []GooglePart{{Text: systemPrompt}},
		},
		Contents: []GoogleContent{
			{
				Role:  "user",
				Parts: []GooglePart{{Text: userPrompt}},
			},
		},
		GenerationConfig: &GoogleGenConfig{
			Temperature:     &[]float64{0.1}[0],
			MaxOutputTokens: &[]int{4096}[0],
		},
	}
	
	endpoint := fmt.Sprintf("/models/%s:generateContent?key=%s", c.model, c.apiKey)
	respBytes, err := c.makeRequest(endpoint, request)
	if err != nil {
		return nil, err
	}
	
	var response GoogleResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}
	
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response candidates returned")
	}
	
	content := extractJSONFromResponse(response.Candidates[0].Content.Parts[0].Text)
	
	var summary PRSummary
	if err := json.Unmarshal([]byte(content), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse PR summary JSON: %w", err)
	}
	
	return &summary, nil
}

func (c *GoogleClient) GenerateCodeReview(prTitle, prDescription, diff string) (*ReviewResult, error) {
	return c.GenerateCodeReviewWithStyleGuide(prTitle, prDescription, diff, "")
}

func (c *GoogleClient) GenerateCodeReviewWithStyleGuide(prTitle, prDescription, diff, styleGuide string) (*ReviewResult, error) {
	var systemPrompt string
	if styleGuide != "" {
		systemPrompt = GetCodeReviewPromptWithStyleGuide(styleGuide)
	} else {
		systemPrompt = GetCodeReviewPrompt()
	}
	
	userPrompt := fmt.Sprintf("PR Title: %s\n\nPR Description: %s\n\nGit Diff:\n%s", prTitle, prDescription, diff)
	
	request := GoogleRequest{
		SystemInstruction: &GoogleContent{
			Parts: []GooglePart{{Text: systemPrompt}},
		},
		Contents: []GoogleContent{
			{
				Role:  "user",
				Parts: []GooglePart{{Text: userPrompt}},
			},
		},
		GenerationConfig: &GoogleGenConfig{
			Temperature:     &[]float64{0.1}[0],
			MaxOutputTokens: &[]int{4096}[0],
		},
	}
	
	endpoint := fmt.Sprintf("/models/%s:generateContent?key=%s", c.model, c.apiKey)
	respBytes, err := c.makeRequest(endpoint, request)
	if err != nil {
		return nil, err
	}
	
	var response GoogleResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}
	
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response candidates returned")
	}
	
	content := extractJSONFromResponse(response.Candidates[0].Content.Parts[0].Text)

	var review ReviewResult
	if err := json.Unmarshal([]byte(content), &review); err != nil {
		return nil, fmt.Errorf("failed to parse review JSON: %w", err)
	}

	return &review, nil
}

func (c *GoogleClient) GenerateResponse(prompt string) (string, error) {
	request := GoogleRequest{
		Contents: []GoogleContent{
			{
				Role:  "user",
				Parts: []GooglePart{{Text: prompt}},
			},
		},
		GenerationConfig: &GoogleGenConfig{
			Temperature:     &[]float64{0.7}[0],
			MaxOutputTokens: &[]int{4096}[0],
		},
	}

	endpoint := fmt.Sprintf("/models/%s:generateContent?key=%s", c.model, c.apiKey)
	respBytes, err := c.makeRequest(endpoint, request)
	if err != nil {
		return "", err
	}

	var response GoogleResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("API error: %s", response.Error.Message)
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response candidates returned")
	}

	return response.Candidates[0].Content.Parts[0].Text, nil
}
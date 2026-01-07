package review

import (
	"strings"
	"testing"

	"github.com/igcodinap/manque-ai/internal"
	"github.com/igcodinap/manque-ai/pkg/ai"
)

// MockAIClient implements ai.Client interface
type MockAIClient struct {
	Summary *ai.PRSummary
	Review  *ai.ReviewResult
}

func (m *MockAIClient) GeneratePRSummary(title, description, diff string) (*ai.PRSummary, error) {
	return m.Summary, nil
}

func (m *MockAIClient) GenerateCodeReview(title, description, diff string) (*ai.ReviewResult, error) {
	return m.Review, nil
}

func (m *MockAIClient) GenerateCodeReviewWithStyleGuide(title, description, diff, rules string) (*ai.ReviewResult, error) {
	return m.Review, nil
}

func (m *MockAIClient) GenerateResponse(prompt string) (string, error) {
	return "Mock response", nil
}

func TestFormatOutput(t *testing.T) {
	summary := &ai.PRSummary{
		Title:       "Test PR",
		Description: "This is a test description",
		// Use anonymous struct literal matches ai.PRSummary definition
		Files: []struct {
			Filename string `json:"filename"`
			Summary  string `json:"summary"`
			Title    string `json:"title"`
		}{
			{Filename: "file1.go", Summary: "Added logic"},
		},
	}

	reviewResult := &ai.ReviewResult{
		Review: ai.ReviewSummary{
			Score:           90,
			EstimatedEffort: 2,
			SecurityConcerns: "None",
		},
		Comments: []ai.Comment{
			{
				File:      "file1.go",
				StartLine: 10,
				EndLine:   12,
				Content:   "Fix this bug",
				Label:     "bug",
				Header:    "🔴 Bug detected",
			},
		},
	}

	output := FormatOutput(summary, reviewResult)

	expectedStrings := []string{
		"🐰 **Executive Summary**",
		"This is a test description",
		"File: file1.go",
		"Line: 10 to 12",
		"Type: bug",
		"Comment:",
		"🔴 Bug detected",
		"Fix this bug",
		"Prompt for AI Agent:",
	}

	for _, exp := range expectedStrings {
		if !strings.Contains(output, exp) {
			t.Errorf("Expected output to contain '%s', but it didn't", exp)
		}
	}
}

func TestEngine_Review(t *testing.T) {
	// Initialize logger for test
	internal.InitLogger(true)

	mockClient := &MockAIClient{
		Summary: &ai.PRSummary{Description: "Mock summary"},
		Review:  &ai.ReviewResult{Comments: []ai.Comment{}},
	}

	config := &internal.Config{
		LLMProvider: "openai", // Dummy
		LLMAPIKey: "dummy",
	}
	
	engine := &Engine{
		AIClient: mockClient,
		Config: config,
	}

	// Simple Diff
	diff := `diff --git a/test.txt b/test.txt
index 123..456 100644
--- a/test.txt
+++ b/test.txt
@@ -1 +1 @@
-old
+new
`
	
	sum, rev, err := engine.Review(diff)
	if err != nil {
		t.Fatalf("Review returned error: %v", err)
	}

	if sum.Description != "Mock summary" {
		t.Errorf("Expected mock summary, got %s", sum.Description)
	}
	if rev == nil {
		t.Error("Expected review result, got nil")
	}
}

package ai

type PRSummary struct {
	Title       string `json:"title"`       // Max 10 words
	Description string `json:"description"`
	Type        []string `json:"type"`      // e.g., BUG, ENHANCEMENT
	Files       []struct {
		Filename string `json:"filename"`
		Summary  string `json:"summary"`    // Max 70 words
		Title    string `json:"title"`      // 5-10 words
	} `json:"files"`
}

type ReviewResult struct {
	Review   ReviewSummary `json:"review"`
	Comments []Comment     `json:"comments"`
}

type ReviewSummary struct {
	EstimatedEffort  int    `json:"estimated_effort_to_review"` // 1-5
	Score            int    `json:"score"`                      // 0-100
	HasRelevantTests bool   `json:"has_relevant_tests"`
	SecurityConcerns string `json:"security_concerns"`
}

// ReviewAction represents the GitHub review action type
type ReviewAction string

const (
	ReviewActionComment        ReviewAction = "COMMENT"
	ReviewActionApprove        ReviewAction = "APPROVE"
	ReviewActionRequestChanges ReviewAction = "REQUEST_CHANGES"
)

// GetReviewAction determines the appropriate GitHub review action based on the review result
func (r *ReviewResult) GetReviewAction(autoApproveThreshold int, blockOnCritical bool) ReviewAction {
	// Check for critical issues
	hasCritical := false
	for _, comment := range r.Comments {
		if comment.Critical {
			hasCritical = true
			break
		}
	}

	// If we have critical issues and blocking is enabled, request changes
	if hasCritical && blockOnCritical {
		return ReviewActionRequestChanges
	}

	// If score is above threshold and no critical issues, approve
	if r.Review.Score >= autoApproveThreshold && !hasCritical && len(r.Comments) == 0 {
		return ReviewActionApprove
	}

	// Default to comment
	return ReviewActionComment
}

type Comment struct {
	File            string `json:"file"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	HighlightedCode string `json:"highlighted_code"`
	Header          string `json:"header"`
	Content         string `json:"content"`
	Label           string `json:"label"`    // e.g. "bug", "security"
	Critical        bool   `json:"critical"`
	SuggestedCode   string `json:"suggested_code,omitempty"` // GitHub suggestion block content
}

type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

type Client interface {
	GeneratePRSummary(prTitle, prDescription, diff string) (*PRSummary, error)
	GenerateCodeReview(prTitle, prDescription, diff string) (*ReviewResult, error)
	GenerateCodeReviewWithStyleGuide(prTitle, prDescription, diff, styleGuide string) (*ReviewResult, error)
	GenerateResponse(prompt string) (string, error) // For conversational responses
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}
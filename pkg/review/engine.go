package review

import (
	"fmt"
	"strings"

	"github.com/manque-ai/internal"
	"github.com/manque-ai/pkg/ai"
	"github.com/manque-ai/pkg/diff"
)

type Engine struct {
	AIClient ai.Client
	Config   *internal.Config
}

func NewEngine(config *internal.Config) (*Engine, error) {
	aiClient, err := ai.NewClient(ai.Config{
		Provider: config.LLMProvider,
		APIKey:   config.LLMAPIKey,
		Model:    config.LLMModel,
		BaseURL:  config.LLMBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AI client: %w", err)
	}

	return &Engine{
		AIClient: aiClient,
		Config:   config,
	}, nil
}

func (e *Engine) Review(diffContent string) (*ai.PRSummary, *ai.ReviewResult, error) {
	// Parse the diff
	files, err := diff.ParseGitDiff(diffContent)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	formattedDiff := diff.FormatForLLM(files)

	// Check for large diffs and truncate if necessary
	const maxDiffSize = 100000
	if len(formattedDiff) > maxDiffSize {
		internal.Logger.Warn(fmt.Sprintf("Diff is too large (%d chars), truncating to %d chars...", len(formattedDiff), maxDiffSize))
		runes := []rune(formattedDiff)
		if len(runes) > maxDiffSize {
			formattedDiff = string(runes[:maxDiffSize]) + "\n... (truncated due to size limit)"
		}
	}

	// Generate Summary
	internal.Logger.Info("Generating PR summary...")
	// For local runs, we might not have title/desc, so we can use placeholders or pass them in if available.
	// In the future, we could prompt for them or parse from first commit message.
	title := "Local Changes"
	description := "Review of local changes"
	
	if e.Config.PRNumber != 0 {
		// If we have context (e.g. from GH), use it. But for pure local, defaults are fine.
		// Wait, cmd/root.go passes real title/desc. We should accept them as args.
		// Let's modify Review to take optional context.
	}

	summary, err := e.AIClient.GeneratePRSummary(title, description, formattedDiff)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate PR summary: %w", err)
	}

	// Generate Code Review
	internal.Logger.Info("Generating code review...")
	var review *ai.ReviewResult
	
	// Combine discovered practices with style guide rules
	combinedRules := e.getCombinedRules()
	
	if combinedRules != "" {
		review, err = e.AIClient.GenerateCodeReviewWithStyleGuide(title, description, formattedDiff, combinedRules)
	} else {
		review, err = e.AIClient.GenerateCodeReview(title, description, formattedDiff)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate code review: %w", err)
	}

	return summary, review, nil
}

// ReviewWithContext allows passing specific title/description (used by GitHub action)
func (e *Engine) ReviewWithContext(title, description, diffContent string) (*ai.PRSummary, *ai.ReviewResult, error) {
	files, err := diff.ParseGitDiff(diffContent)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	formattedDiff := diff.FormatForLLM(files)

	const maxDiffSize = 100000
	if len(formattedDiff) > maxDiffSize {
		internal.Logger.Warn(fmt.Sprintf("Diff is too large (%d chars), truncating to %d chars...", len(formattedDiff), maxDiffSize))
		runes := []rune(formattedDiff)
		if len(runes) > maxDiffSize {
			formattedDiff = string(runes[:maxDiffSize]) + "\n... (truncated due to size limit)"
		}
	}

	internal.Logger.Info("Generating PR summary...")
	summary, err := e.AIClient.GeneratePRSummary(title, description, formattedDiff)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate PR summary: %w", err)
	}

	internal.Logger.Info("Generating code review...")
	var review *ai.ReviewResult
	
	// Combine discovered practices with style guide rules
	combinedRules := e.getCombinedRules()
	
	if combinedRules != "" {
		review, err = e.AIClient.GenerateCodeReviewWithStyleGuide(title, description, formattedDiff, combinedRules)
	} else {
		review, err = e.AIClient.GenerateCodeReview(title, description, formattedDiff)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate code review: %w", err)
	}

	return summary, review, nil
}

// getCombinedRules combines discovered practices with user-provided style guide rules
func (e *Engine) getCombinedRules() string {
	var parts []string
	
	if e.Config.DiscoveredPractices != "" {
		parts = append(parts, "## Repository Practices (Auto-Discovered)\n\n"+e.Config.DiscoveredPractices)
	}
	
	if e.Config.StyleGuideRules != "" {
		parts = append(parts, "## Custom Style Guide Rules\n\n"+e.Config.StyleGuideRules)
	}
	
	if len(parts) == 0 {
		return ""
	}
	
	return strings.Join(parts, "\n\n---\n\n")
}

// FormatOutput generates the standard markdown report
func FormatOutput(summary *ai.PRSummary, review *ai.ReviewResult) string {
	var builder strings.Builder

	// We can still print the summary at the top if desired, or skip it to match the requested "structure" exactly.
	// The user request shows file-based comments.
	// However, usually a summary is nice. I will keep the summary but format the comments as requested.
	
	builder.WriteString("🐰 **Executive Summary**\n")
	builder.WriteString(summary.Description + "\n\n")

	if len(review.Comments) == 0 {
		builder.WriteString("No issues found! 🎉\n")
		return builder.String()
	}

	for _, comment := range review.Comments {
		// Determine Type
		issueType := comment.Label
		if issueType == "" {
			issueType = "potential_issue"
		}
		if comment.Critical {
			issueType = "critical_issue"
		}

		builder.WriteString("============================================================================\n")
		builder.WriteString(fmt.Sprintf("File: %s\n", comment.File))
		if comment.EndLine > 0 && comment.EndLine > comment.StartLine {
			builder.WriteString(fmt.Sprintf("Line: %d to %d\n", comment.StartLine, comment.EndLine))
		} else if comment.StartLine > 0 {
			builder.WriteString(fmt.Sprintf("Line: %d\n", comment.StartLine))
		} else {
			builder.WriteString("Line: (unknown)\n")
		}
		builder.WriteString(fmt.Sprintf("Type: %s\n\n", issueType))

		// Clean up Header (remove emoji if present for the "Comment" section?)
		// The user example had "Comment:\nRemove duplicate line.\n\nLine 105..."
		// Our Header is usually short. Content is longer.
		// Let's combine them or just use Header as title.
		
		builder.WriteString("Comment:\n")
		if comment.Header != "" {
			builder.WriteString(comment.Header + "\n\n")
		}
		builder.WriteString(comment.Content + "\n\n")

		if comment.HighlightedCode != "" {
			builder.WriteString(comment.HighlightedCode + "\n\n")
		}

		builder.WriteString("Prompt for AI Agent:\n")
		// Construct the agent prompt
		// "In @<file> around lines <start> - <end>, <content/instruction>"
		agentPrompt := fmt.Sprintf("In @%s around lines %d - %d, %s", 
			comment.File, 
			comment.StartLine, 
			comment.EndLine, 
			strings.ReplaceAll(comment.Content, "\n", " "))
		
		builder.WriteString(agentPrompt + "\n\n")
	}

	return builder.String()
}

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
	if e.Config.StyleGuideRules != "" {
		review, err = e.AIClient.GenerateCodeReviewWithStyleGuide(title, description, formattedDiff, e.Config.StyleGuideRules)
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
	if e.Config.StyleGuideRules != "" {
		review, err = e.AIClient.GenerateCodeReviewWithStyleGuide(title, description, formattedDiff, e.Config.StyleGuideRules)
	} else {
		review, err = e.AIClient.GenerateCodeReview(title, description, formattedDiff)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate code review: %w", err)
	}

	return summary, review, nil
}

// FormatOutput generates the standard markdown report
func FormatOutput(summary *ai.PRSummary, review *ai.ReviewResult) string {
	var builder strings.Builder
	
	builder.WriteString("🐰 **Executive Summary**\n")
	builder.WriteString(summary.Description + "\n\n")
	
	builder.WriteString("🔍 **Walkthrough**\n")
	builder.WriteString("| File | Summary |\n")
	builder.WriteString("|------|----------|\n")
	for _, file := range summary.Files {
		builder.WriteString(fmt.Sprintf("| `%s` | %s |\n", file.Filename, file.Summary))
	}
	builder.WriteString("\n")
	
	// Group comments
	var critical, warnings, suggestions []ai.Comment
	for _, comment := range review.Comments {
		switch {
		case comment.Critical || comment.Label == "security" || strings.Contains(comment.Header, "🔴"):
			critical = append(critical, comment)
		case comment.Label == "bug" || strings.Contains(comment.Header, "🟡"):
			warnings = append(warnings, comment)
		default:
			suggestions = append(suggestions, comment)
		}
	}
	
	if len(critical) > 0 {
		builder.WriteString("🔴 **Critical Issues**\n")
		for _, comment := range critical {
			builder.WriteString(fmt.Sprintf("- **%s:%d** - %s\n", comment.File, comment.StartLine, comment.Header))
		}
		builder.WriteString("\n")
	}
	
	if len(warnings) > 0 {
		builder.WriteString("🟡 **Warnings**\n")
		for _, comment := range warnings {
			builder.WriteString(fmt.Sprintf("- **%s:%d** - %s\n", comment.File, comment.StartLine, comment.Header))
		}
		builder.WriteString("\n")
	}
	
	if len(suggestions) > 0 {
		builder.WriteString("💡 **Suggestions**\n")
		for _, comment := range suggestions {
			builder.WriteString(fmt.Sprintf("- **%s:%d** - %s\n", comment.File, comment.StartLine, comment.Header))
		}
		builder.WriteString("\n")
	}
	
	builder.WriteString(fmt.Sprintf("**Quality Score**: %d/100 | **Review Effort**: %d/5 | **Security**: %s",
		review.Review.Score, 
		review.Review.EstimatedEffort,
		review.Review.SecurityConcerns))
	
	return builder.String()
}

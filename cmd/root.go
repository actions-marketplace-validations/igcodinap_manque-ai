package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/manque-ai/internal"
	"github.com/manque-ai/pkg/ai"
	"github.com/manque-ai/pkg/github"
	"github.com/manque-ai/pkg/review"
	"github.com/spf13/cobra"
	gh "github.com/google/go-github/v60/github"
)

var (
	prNumber   int
	prURL      string
	repository string
)

var rootCmd = &cobra.Command{
	Use:   "manque-ai",
	Short: "AI-powered Pull Request reviewer",
	Long:  `A robust Golang binary that reviews Pull Requests using LLMs (OpenAI, Anthropic, Google, OpenRouter).`,
	Run:   runReview,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.Flags().IntVar(&prNumber, "pr", 0, "PR number to review")
	rootCmd.Flags().StringVar(&prURL, "url", "", "GitHub PR URL to review")
	rootCmd.Flags().StringVar(&repository, "repo", "", "Repository in format 'owner/repo'")
}

func runReview(cmd *cobra.Command, args []string) {
	// Initialize logging
	debug, _ := cmd.Flags().GetBool("debug")
	internal.InitLogger(debug)
	
	config, err := internal.LoadConfig()
	if err != nil {
		internal.Logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}
	// Action/Remote CLI always requires GitHub Token
	if err := config.Validate(); err != nil {
		internal.Logger.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	// Initialize clients
	githubClient := github.NewClient(config.GitHubToken, config.GitHubAPIURL)
	engine, err := review.NewEngine(config)
	if err != nil {
		internal.Logger.Error("Failed to initialize review engine", "error", err)
		os.Exit(1)
	}

	// Get PR information
	var prInfo *github.PRInfo
	if config.GitHubEventPath != "" {
		// Running as GitHub Action
		prInfo, err = githubClient.GetPRFromEvent(config.GitHubEventPath)
		if err != nil {
			internal.Logger.Error("Failed to get PR from GitHub event", "error", err)
			os.Exit(1)
		}
	} else if prURL != "" {
		// CLI mode with URL
		prInfo, err = githubClient.GetPRFromURL(prURL)
		if err != nil {
			internal.Logger.Error("Failed to get PR from URL", "error", err)
			os.Exit(1)
		}
	} else if repository != "" && prNumber > 0 {
		// CLI mode with repo and PR number
		parts := strings.Split(repository, "/")
		if len(parts) != 2 {
			internal.Logger.Error("Invalid repository format. Use 'owner/repo'")
			os.Exit(1)
		}
		prInfo, err = githubClient.GetPR(parts[0], parts[1], prNumber)
		if err != nil {
			internal.Logger.Error("Failed to get PR", "error", err)
			os.Exit(1)
		}
	} else {
		internal.Logger.Error("Must provide either GITHUB_EVENT_PATH (for Actions) or --url/--repo+--pr (for CLI)")
		os.Exit(1)
	}

	internal.Logger.Info("Reviewing PR", "number", prInfo.Number, "title", prInfo.Title)

	// Run Review
	// Note: We use ReviewWithContext since we have the full PR details
	summary, result, err := engine.ReviewWithContext(prInfo.Title, prInfo.Description, prInfo.Diff)
	if err != nil {
		internal.Logger.Error("Review failed", "error", err)
		os.Exit(1)
	}

	// Post results to GitHub
	err = postResultsToGitHub(githubClient, prInfo, summary, result, config)
	if err != nil {
		internal.Logger.Error("Failed to post results to GitHub", "error", err)
		os.Exit(1)
	}

	internal.Logger.Info("✅ Review completed successfully!")
}

func postResultsToGitHub(githubClient *github.Client, prInfo *github.PRInfo, summary *ai.PRSummary, review *ai.ReviewResult, config *internal.Config) error {
	parts := strings.Split(prInfo.Repository, "/")
	owner, repo := parts[0], parts[1]

	// Update PR title if configured
	if config.UpdatePRTitle {
		if err := githubClient.UpdatePR(owner, repo, prInfo.Number, &summary.Title, nil); err != nil {
			return fmt.Errorf("failed to update PR title: %w", err)
		}
	}

	// Update PR body with full report if configured
	if config.UpdatePRBody {
		// Build the AI summary section
		walkthrough := formatWalkthrough(summary, review)
		
		var aiSection strings.Builder
		aiSection.WriteString("\n\n<!-- ai-review-start -->\n")
		aiSection.WriteString("# 🤖 AI Code Review\n\n")
		aiSection.WriteString(walkthrough)
		aiSection.WriteString("\n<!-- ai-review-end -->")
		
		// Strip any existing AI summary from the description and add new one
		enhanced := stripAISummary(prInfo.Description) + aiSection.String()
		
		if err := githubClient.UpdatePR(owner, repo, prInfo.Number, nil, &enhanced); err != nil {
			return fmt.Errorf("failed to update PR body: %w", err)
		}
	}

	// Create review with inline comments
	if len(review.Comments) > 0 {
		var reviewComments []*gh.DraftReviewComment
		seenComments := make(map[string]bool) // Deduplicate before sending
		
		for _, comment := range review.Comments {
			// Combine header and content for a complete, unique comment
			body := fmt.Sprintf("**%s**\n\n%s", comment.Header, comment.Content)
			
			// Create a fingerprint to detect duplicates within this batch
			fingerprint := fmt.Sprintf("%s:%d:%d:%s", comment.File, comment.StartLine, comment.EndLine, body)
			if seenComments[fingerprint] {
				continue // Skip duplicate
			}
			seenComments[fingerprint] = true
			
			reviewComments = append(reviewComments, &gh.DraftReviewComment{
				Path:      &comment.File,
				Line:      &comment.EndLine,
				StartLine: &comment.StartLine,
				Body:      &body,
			})
		}
		
		reviewBody := fmt.Sprintf("## Code Review Summary\n\n" +
			"**Estimated Review Effort**: %d/5\n" +
			"**Quality Score**: %d/100\n" +
			"**Has Relevant Tests**: %t\n" +
			"**Security Concerns**: %s\n\n" +
			"Found %d issues requiring attention.",
			review.Review.EstimatedEffort,
			review.Review.Score,
			review.Review.HasRelevantTests,
			review.Review.SecurityConcerns,
			len(review.Comments))
		
		if err := githubClient.CreateReview(owner, repo, prInfo.Number, reviewComments, &reviewBody); err != nil {
			return fmt.Errorf("failed to create review: %w", err)
		}
	}

	return nil
}

// stripAISummary removes any existing AI Summary section from the PR description
func stripAISummary(description string) string {
	// 1. Try to find the new robust HTML markers
	startMarker := "<!-- ai-review-start -->"
	idx := strings.Index(description, startMarker)
	if idx != -1 {
		return strings.TrimSpace(description[:idx])
	}

	// 2. Fallback: Find the old markdown header marker
	aiSummaryMarker := "## AI Summary"
	idx = strings.Index(description, aiSummaryMarker)
	if idx != -1 {
		return strings.TrimSpace(description[:idx])
	}

	return strings.TrimSpace(description)
}

func formatWalkthrough(summary *ai.PRSummary, review *ai.ReviewResult) string {
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
	
	// Group comments by severity
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
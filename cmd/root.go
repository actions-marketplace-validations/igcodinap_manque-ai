package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/manque-ai/internal"
	"github.com/manque-ai/pkg/ai"
	"github.com/manque-ai/pkg/diff"
	"github.com/manque-ai/pkg/github"
	"github.com/spf13/cobra"
	gh "github.com/google/go-github/v60/github"
)

var (
	prNumber   int
	prURL      string
	repository string
)

var rootCmd = &cobra.Command{
	Use:   "ai-reviewer",
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
	rootCmd.Flags().IntVar(&prNumber, "pr", 0, "PR number to review")
	rootCmd.Flags().StringVar(&prURL, "url", "", "GitHub PR URL to review")
	rootCmd.Flags().StringVar(&repository, "repo", "", "Repository in format 'owner/repo'")
}

func runReview(cmd *cobra.Command, args []string) {
	// Initialize logging
	internal.InitLogger()
	
	config, err := internal.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize clients
	githubClient := github.NewClient(config.GitHubToken, config.GitHubAPIURL)
	aiClient, err := ai.NewClient(ai.Config{
		Provider: config.LLMProvider,
		APIKey:   config.LLMAPIKey,
		Model:    config.LLMModel,
		BaseURL:  config.LLMBaseURL,
	})
	if err != nil {
		log.Fatalf("Failed to initialize AI client: %v", err)
	}

	// Get PR information
	var prInfo *github.PRInfo
	if config.GitHubEventPath != "" {
		// Running as GitHub Action
		prInfo, err = githubClient.GetPRFromEvent(config.GitHubEventPath)
		if err != nil {
			log.Fatalf("Failed to get PR from GitHub event: %v", err)
		}
	} else if prURL != "" {
		// CLI mode with URL
		prInfo, err = githubClient.GetPRFromURL(prURL)
		if err != nil {
			log.Fatalf("Failed to get PR from URL: %v", err)
		}
	} else if repository != "" && prNumber > 0 {
		// CLI mode with repo and PR number
		parts := strings.Split(repository, "/")
		if len(parts) != 2 {
			log.Fatalf("Invalid repository format. Use 'owner/repo'")
		}
		prInfo, err = githubClient.GetPR(parts[0], parts[1], prNumber)
		if err != nil {
			log.Fatalf("Failed to get PR: %v", err)
		}
	} else {
		log.Fatalf("Must provide either GITHUB_EVENT_PATH (for Actions) or --url/--repo+--pr (for CLI)")
	}

	fmt.Printf("Reviewing PR #%d: %s\n", prInfo.Number, prInfo.Title)

	// Parse the diff
	files, err := diff.ParseGitDiff(prInfo.Diff)
	if err != nil {
		log.Fatalf("Failed to parse diff: %v", err)
	}

	formattedDiff := diff.FormatForLLM(files)

	// Check for large diffs and truncate if necessary (limit to ~100k characters for safety)
	// Most LLMs have around 128k context, we leave room for system prompt and output
	const maxDiffSize = 100000
	if len(formattedDiff) > maxDiffSize {
		fmt.Printf("⚠️ Diff is too large (%d chars), truncating to %d chars...\n", len(formattedDiff), maxDiffSize)
		// Handle UTF-8 safe truncation
		runes := []rune(formattedDiff)
		if len(runes) > maxDiffSize {
			formattedDiff = string(runes[:maxDiffSize]) + "\n... (truncated due to size limit)"
		}
	}

	// Generate PR summary
	fmt.Println("Generating PR summary...")
	summary, err := aiClient.GeneratePRSummary(prInfo.Title, prInfo.Description, formattedDiff)
	if err != nil {
		log.Fatalf("Failed to generate PR summary: %v", err)
	}

	// Generate code review
	fmt.Println("Generating code review...")
	var review *ai.ReviewResult
	if config.StyleGuideRules != "" {
		review, err = aiClient.GenerateCodeReviewWithStyleGuide(prInfo.Title, prInfo.Description, formattedDiff, config.StyleGuideRules)
	} else {
		review, err = aiClient.GenerateCodeReview(prInfo.Title, prInfo.Description, formattedDiff)
	}
	if err != nil {
		log.Fatalf("Failed to generate code review: %v", err)
	}

	// Post results to GitHub
	err = postResultsToGitHub(githubClient, prInfo, summary, review, config)
	if err != nil {
		log.Fatalf("Failed to post results to GitHub: %v", err)
	}

	fmt.Println("✅ Review completed successfully!")
}

func postResultsToGitHub(githubClient *github.Client, prInfo *github.PRInfo, summary *ai.PRSummary, review *ai.ReviewResult, config *internal.Config) error {
	parts := strings.Split(prInfo.Repository, "/")
	owner, repo := parts[0], parts[1]

	// Update PR title and body if configured
	if config.UpdatePRTitle || config.UpdatePRBody {
		var newTitle, newBody *string
		if config.UpdatePRTitle {
			newTitle = &summary.Title
		}
		if config.UpdatePRBody {
			// Create enhanced description
			enhanced := fmt.Sprintf("%s\n\n## AI Summary\n%s\n\n### Files Changed\n", 
				prInfo.Description, summary.Description)
			for _, file := range summary.Files {
				enhanced += fmt.Sprintf("- **%s**: %s\n", file.Filename, file.Summary)
			}
			newBody = &enhanced
		}
		
		if err := githubClient.UpdatePR(owner, repo, prInfo.Number, newTitle, newBody); err != nil {
			return fmt.Errorf("failed to update PR: %w", err)
		}
	}

	// Create walkthrough comment
	walkthroughBody := formatWalkthrough(summary, review)
	if err := githubClient.CreateComment(owner, repo, prInfo.Number, walkthroughBody); err != nil {
		return fmt.Errorf("failed to create walkthrough comment: %w", err)
	}

	// Create review with inline comments
	if len(review.Comments) > 0 {
		var reviewComments []*gh.DraftReviewComment
		for _, comment := range review.Comments {
			reviewComments = append(reviewComments, &gh.DraftReviewComment{
				Path:      &comment.File,
				Line:      &comment.EndLine,
				StartLine: &comment.StartLine,
				Body:      &comment.Content,
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
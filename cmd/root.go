package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/igcodinap/manque-ai/internal"
	"github.com/igcodinap/manque-ai/pkg/ai"
	"github.com/igcodinap/manque-ai/pkg/github"
	"github.com/igcodinap/manque-ai/pkg/review"
	"github.com/igcodinap/manque-ai/pkg/state"
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

	// Check for incremental review
	tracker := state.NewTracker(prInfo.Repository, prInfo.Number)
	isIncremental, previousState := tracker.IsIncrementalReview(prInfo.Description, prInfo.HeadSHA)

	// Load or create session for memory across reviews
	sessionManager := state.NewSessionManager(prInfo.Repository, prInfo.Number)
	session := sessionManager.GetOrCreateSession(prInfo.Description)
	if len(session.Reviews) > 0 {
		internal.Logger.Info("Session loaded", "previous_reviews", len(session.Reviews), "dismissed_issues", len(session.Dismissed))
	}

	var diffToReview string
	if isIncremental && previousState != nil {
		// Get incremental diff
		internal.Logger.Info("Incremental review detected", "previous_sha", previousState.LastReviewedSHA[:7], "current_sha", prInfo.HeadSHA[:7])
		incrementalDiff, err := state.GetIncrementalDiff(previousState.LastReviewedSHA, prInfo.HeadSHA)
		if err != nil {
			internal.Logger.Warn("Failed to get incremental diff, falling back to full review", "error", err)
			diffToReview = prInfo.Diff
		} else if incrementalDiff == "" {
			internal.Logger.Info("No new changes to review")
			return
		} else {
			diffToReview = incrementalDiff
		}
	} else {
		diffToReview = prInfo.Diff
	}

	// Run Review
	// Note: We use ReviewWithContext since we have the full PR details
	summary, result, err := engine.ReviewWithContext(prInfo.Title, prInfo.Description, diffToReview)
	if err != nil {
		internal.Logger.Error("Review failed", "error", err)
		os.Exit(1)
	}

	// Filter out dismissed issues from session memory
	filteredComments := filterDismissedComments(result.Comments, session)
	result.Comments = filteredComments

	// Compute comment hashes for session tracking
	var commentHashes []string
	for _, comment := range result.Comments {
		hash := state.ComputeCommentHash(comment.File, comment.StartLine, comment.EndLine, comment.Content)
		commentHashes = append(commentHashes, hash)
	}

	// Update session with this review
	session.AddReviewRecord(prInfo.HeadSHA, commentHashes, result.Review.Score, len(result.Comments))
	session.TrimSession(10) // Keep last 10 reviews
	sessionMarker := state.CreateSessionMarker(session)

	// Store review state for future incremental reviews
	newState := tracker.CreateNewState(prInfo.HeadSHA, len(result.Comments))
	stateMarker := state.CreateStateMarker(newState)

	// Post results to GitHub
	err = postResultsToGitHub(githubClient, prInfo, summary, result, config, stateMarker, sessionMarker, isIncremental)
	if err != nil {
		internal.Logger.Error("Failed to post results to GitHub", "error", err)
		os.Exit(1)
	}

	if isIncremental {
		internal.Logger.Info("✅ Incremental review completed successfully!")
	} else {
		internal.Logger.Info("✅ Review completed successfully!")
	}
}

// filterDismissedComments removes comments that were previously dismissed by users
func filterDismissedComments(comments []ai.Comment, session *state.Session) []ai.Comment {
	if session == nil || len(session.Dismissed) == 0 {
		return comments
	}

	var filtered []ai.Comment
	dismissedCount := 0
	for _, comment := range comments {
		hash := state.ComputeCommentHash(comment.File, comment.StartLine, comment.EndLine, comment.Content)
		if session.IsDismissed(hash) {
			dismissedCount++
			internal.Logger.Debug("Skipping dismissed issue", "file", comment.File, "line", comment.StartLine)
			continue
		}
		filtered = append(filtered, comment)
	}

	if dismissedCount > 0 {
		internal.Logger.Info("Filtered dismissed issues", "count", dismissedCount)
	}

	return filtered
}

func postResultsToGitHub(githubClient *github.Client, prInfo *github.PRInfo, summary *ai.PRSummary, review *ai.ReviewResult, config *internal.Config, stateMarker, sessionMarker string, isIncremental bool) error {
	parts := strings.Split(prInfo.Repository, "/")
	owner, repo := parts[0], parts[1]

	// Update PR title if configured (only on first review, not incremental)
	if config.UpdatePRTitle && !isIncremental {
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
		if isIncremental {
			aiSection.WriteString("# 🤖 AI Code Review (Incremental)\n\n")
		} else {
			aiSection.WriteString("# 🤖 AI Code Review\n\n")
		}
		aiSection.WriteString(walkthrough)
		aiSection.WriteString("\n")
		// Add state marker for future incremental reviews
		if stateMarker != "" {
			aiSection.WriteString(stateMarker)
			aiSection.WriteString("\n")
		}
		// Add session marker for memory across reviews
		if sessionMarker != "" {
			aiSection.WriteString(sessionMarker)
			aiSection.WriteString("\n")
		}
		aiSection.WriteString("<!-- ai-review-end -->")

		// Strip any existing AI summary, state marker, and session marker from the description
		cleanDescription := state.StripSessionMarker(state.StripStateMarker(stripAISummary(prInfo.Description)))
		enhanced := cleanDescription + aiSection.String()

		if err := githubClient.UpdatePR(owner, repo, prInfo.Number, nil, &enhanced); err != nil {
			return fmt.Errorf("failed to update PR body: %w", err)
		}
	}

	// Create review with inline comments
	if len(review.Comments) > 0 {
		internal.Logger.Debug("AI returned comments", "count", len(review.Comments))
		
		var reviewComments []*gh.DraftReviewComment
		seenComments := make(map[string]bool) // Deduplicate before sending
		batchDuplicates := 0
		
		for _, comment := range review.Comments {
			// Combine header and content for a complete, unique comment
			var body strings.Builder
			body.WriteString(fmt.Sprintf("**%s**\n\n%s", comment.Header, comment.Content))

			// Add GitHub suggestion block if we have suggested code
			if comment.SuggestedCode != "" {
				body.WriteString("\n\n```suggestion\n")
				body.WriteString(comment.SuggestedCode)
				// Ensure newline at end of suggestion
				if !strings.HasSuffix(comment.SuggestedCode, "\n") {
					body.WriteString("\n")
				}
				body.WriteString("```")
			}

			bodyStr := body.String()

			// Create a fingerprint to detect duplicates within this batch
			fingerprint := fmt.Sprintf("%s:%d:%d:%s", comment.File, comment.StartLine, comment.EndLine, bodyStr)
			if seenComments[fingerprint] {
				batchDuplicates++
				internal.Logger.Debug("Batch duplicate found", "file", comment.File, "startLine", comment.StartLine, "endLine", comment.EndLine)
				continue // Skip duplicate
			}
			seenComments[fingerprint] = true

			reviewComments = append(reviewComments, &gh.DraftReviewComment{
				Path:      &comment.File,
				Line:      &comment.EndLine,
				StartLine: &comment.StartLine,
				Body:      &bodyStr,
			})
		}
		internal.Logger.Debug("Batch deduplication complete", "unique_comments", len(reviewComments), "batch_duplicates", batchDuplicates)
		
		// Determine review action based on score and critical issues
		reviewAction := review.GetReviewAction(config.AutoApproveThreshold, config.BlockOnCritical)
		internal.Logger.Debug("Review action determined", "action", reviewAction, "score", review.Review.Score, "threshold", config.AutoApproveThreshold)

		actionEmoji := "💬"
		actionText := "Comment"
		switch reviewAction {
		case ai.ReviewActionApprove:
			actionEmoji = "✅"
			actionText = "Approved"
		case ai.ReviewActionRequestChanges:
			actionEmoji = "🚫"
			actionText = "Changes Requested"
		}

		reviewBody := fmt.Sprintf("## %s Code Review Summary\n\n"+
			"**Estimated Review Effort**: %d/5\n"+
			"**Quality Score**: %d/100\n"+
			"**Has Relevant Tests**: %t\n"+
			"**Security Concerns**: %s\n\n"+
			"Found %d issues requiring attention.\n\n"+
			"**Review Action**: %s %s",
			actionEmoji,
			review.Review.EstimatedEffort,
			review.Review.Score,
			review.Review.HasRelevantTests,
			review.Review.SecurityConcerns,
			len(review.Comments),
			actionEmoji,
			actionText)

		opts := github.CreateReviewOptions{IsIncremental: isIncremental}
		if err := githubClient.CreateReviewWithOptions(owner, repo, prInfo.Number, reviewComments, &reviewBody, string(reviewAction), opts); err != nil {
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
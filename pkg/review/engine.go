package review

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/igcodinap/manque-ai/internal"
	"github.com/igcodinap/manque-ai/pkg/ai"
	"github.com/igcodinap/manque-ai/pkg/context"
	"github.com/igcodinap/manque-ai/pkg/diff"
)

const (
	// MaxChunkSize is the maximum size of a diff chunk in characters
	MaxChunkSize = 80000
	// MinChunkSize is the minimum useful chunk size
	MinChunkSize = 10000
)

type Engine struct {
	AIClient       ai.Client
	Config         *internal.Config
	ContextFetcher *context.Fetcher
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

	// Initialize context fetcher with current working directory
	var ctxFetcher *context.Fetcher
	if cwd, err := os.Getwd(); err == nil {
		ctxFetcher = context.NewFetcher(cwd)
	}

	return &Engine{
		AIClient:       aiClient,
		Config:         config,
		ContextFetcher: ctxFetcher,
	}, nil
}

func (e *Engine) Review(diffContent string) (*ai.PRSummary, *ai.ReviewResult, error) {
	return e.ReviewWithContext("Local Changes", "Review of local changes", diffContent)
}

// ReviewWithContext allows passing specific title/description (used by GitHub action)
func (e *Engine) ReviewWithContext(title, description, diffContent string) (*ai.PRSummary, *ai.ReviewResult, error) {
	files, err := diff.ParseGitDiff(diffContent)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	// Filter out ignored files
	filteredFiles := e.filterIgnoredFiles(files)
	if len(filteredFiles) == 0 {
		internal.Logger.Info("No files to review after filtering")
		return &ai.PRSummary{Description: "No reviewable files"}, &ai.ReviewResult{}, nil
	}

	// Create chunks based on file sizes
	chunks := e.createFileChunks(filteredFiles)
	internal.Logger.Info(fmt.Sprintf("Processing %d files in %d chunk(s)", len(filteredFiles), len(chunks)))

	// Generate summary using the first chunk (or full diff if small enough)
	summaryDiff := diff.FormatForLLM(chunks[0])
	if len(chunks) > 1 {
		// For summary, use a condensed version of all files
		summaryDiff = e.createSummaryDiff(filteredFiles)
	}

	internal.Logger.Info("Generating PR summary...")
	summary, err := e.AIClient.GeneratePRSummary(title, description, summaryDiff)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate PR summary: %w", err)
	}

	// Generate code review for each chunk and aggregate comments
	combinedRules := e.getCombinedRules()
	var allComments []ai.Comment
	var totalScore, totalEffort int

	for i, chunk := range chunks {
		chunkDiff := diff.FormatForLLM(chunk)

		// Fetch referenced files for context expansion
		var contextSection string
		if e.ContextFetcher != nil {
			referencedFiles := e.ContextFetcher.FetchReferencedFiles(chunk)
			if len(referencedFiles) > 0 {
				contextSection = context.FormatForLLM(referencedFiles)
				internal.Logger.Debug(fmt.Sprintf("Added %d referenced files to context", len(referencedFiles)))
			}
		}

		// Add git blame context for code history
		blameContext := e.getBlameContext(chunk)
		if blameContext != "" {
			contextSection += blameContext
		}

		// Combine diff with context
		fullContext := chunkDiff
		if contextSection != "" {
			fullContext = chunkDiff + "\n" + contextSection
		}

		internal.Logger.Info(fmt.Sprintf("Generating code review for chunk %d/%d (%d files, %d chars)...",
			i+1, len(chunks), len(chunk), len(fullContext)))

		var review *ai.ReviewResult
		if combinedRules != "" {
			review, err = e.AIClient.GenerateCodeReviewWithStyleGuide(title, description, fullContext, combinedRules)
		} else {
			review, err = e.AIClient.GenerateCodeReview(title, description, fullContext)
		}
		if err != nil {
			internal.Logger.Warn(fmt.Sprintf("Failed to review chunk %d: %v", i+1, err))
			continue
		}

		allComments = append(allComments, review.Comments...)
		totalScore += review.Review.Score
		totalEffort += review.Review.EstimatedEffort
	}

	// Aggregate results
	avgScore := totalScore
	avgEffort := totalEffort
	if len(chunks) > 0 {
		avgScore = totalScore / len(chunks)
		avgEffort = totalEffort / len(chunks)
	}

	aggregatedReview := &ai.ReviewResult{
		Review: ai.ReviewSummary{
			Score:            avgScore,
			EstimatedEffort:  avgEffort,
			HasRelevantTests: e.hasTestFiles(filteredFiles),
			SecurityConcerns: e.aggregateSecurityConcerns(allComments),
		},
		Comments: allComments,
	}

	return summary, aggregatedReview, nil
}

// filterIgnoredFiles removes files that match ignore patterns
func (e *Engine) filterIgnoredFiles(files []diff.FileDiff) []diff.FileDiff {
	if e.Config == nil {
		return files
	}

	var filtered []diff.FileDiff
	for _, file := range files {
		if !e.Config.ShouldIgnoreFile(file.Filename) {
			filtered = append(filtered, file)
		} else {
			internal.Logger.Debug("Ignoring file", "file", file.Filename)
		}
	}
	return filtered
}

// createFileChunks groups files into chunks that fit within the size limit
func (e *Engine) createFileChunks(files []diff.FileDiff) [][]diff.FileDiff {
	if len(files) == 0 {
		return nil
	}

	// Calculate size for each file
	type fileWithSize struct {
		file diff.FileDiff
		size int
	}
	filesWithSizes := make([]fileWithSize, len(files))
	for i, file := range files {
		formatted := diff.FormatForLLM([]diff.FileDiff{file})
		filesWithSizes[i] = fileWithSize{file: file, size: len(formatted)}
	}

	// Sort by size (largest first) for better packing
	sort.Slice(filesWithSizes, func(i, j int) bool {
		return filesWithSizes[i].size > filesWithSizes[j].size
	})

	// Greedy bin packing
	var chunks [][]diff.FileDiff
	var currentChunk []diff.FileDiff
	currentSize := 0

	for _, fws := range filesWithSizes {
		// If this single file is too large, it gets its own chunk
		if fws.size > MaxChunkSize {
			if len(currentChunk) > 0 {
				chunks = append(chunks, currentChunk)
				currentChunk = nil
				currentSize = 0
			}
			chunks = append(chunks, []diff.FileDiff{fws.file})
			internal.Logger.Warn(fmt.Sprintf("File %s is very large (%d chars), reviewing separately", fws.file.Filename, fws.size))
			continue
		}

		// If adding this file exceeds limit, start new chunk
		if currentSize+fws.size > MaxChunkSize && len(currentChunk) > 0 {
			chunks = append(chunks, currentChunk)
			currentChunk = nil
			currentSize = 0
		}

		currentChunk = append(currentChunk, fws.file)
		currentSize += fws.size
	}

	// Add remaining files
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// createSummaryDiff creates a condensed diff for summary generation
func (e *Engine) createSummaryDiff(files []diff.FileDiff) string {
	var builder strings.Builder
	builder.WriteString("# Files Changed Summary\n\n")

	for _, file := range files {
		addedLines := 0
		removedLines := 0
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				switch line.Type {
				case diff.LineAdded:
					addedLines++
				case diff.LineRemoved:
					removedLines++
				}
			}
		}
		builder.WriteString(fmt.Sprintf("- %s (+%d/-%d)\n", file.Filename, addedLines, removedLines))
	}

	// Add first file's diff as example if space permits
	if len(files) > 0 {
		firstDiff := diff.FormatForLLM([]diff.FileDiff{files[0]})
		if len(firstDiff) < MaxChunkSize/2 {
			builder.WriteString("\n# First File Details\n")
			builder.WriteString(firstDiff)
		}
	}

	return builder.String()
}

// hasTestFiles checks if any of the files are test files
func (e *Engine) hasTestFiles(files []diff.FileDiff) bool {
	for _, file := range files {
		if strings.Contains(file.Filename, "_test.go") ||
			strings.Contains(file.Filename, ".test.") ||
			strings.Contains(file.Filename, ".spec.") ||
			strings.Contains(file.Filename, "__tests__") {
			return true
		}
	}
	return false
}

// aggregateSecurityConcerns combines security-related comments
func (e *Engine) aggregateSecurityConcerns(comments []ai.Comment) string {
	var concerns []string
	for _, comment := range comments {
		if comment.Label == "security" || comment.Critical {
			concerns = append(concerns, comment.Header)
		}
	}
	if len(concerns) == 0 {
		return "No significant security issues detected"
	}
	return fmt.Sprintf("%d security concern(s): %s", len(concerns), strings.Join(concerns, "; "))
}

// getBlameContext gets git blame context for files in a chunk
func (e *Engine) getBlameContext(files []diff.FileDiff) string {
	blameContexts := make(map[string]string)

	for _, file := range files {
		// Get the line numbers that were changed
		var changedLines []int
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Type == diff.LineAdded {
					changedLines = append(changedLines, line.NewNum)
				}
			}
		}

		if len(changedLines) > 0 {
			blameCtx := context.GetFileBlameContext(file.Filename, changedLines)
			if blameCtx != "" {
				blameContexts[file.Filename] = blameCtx
			}
		}
	}

	return context.FormatBlameContext(blameContexts)
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

	builder.WriteString("🪶 **Executive Summary**\n")
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
			builder.WriteString("Current Code:\n```\n")
			builder.WriteString(comment.HighlightedCode)
			builder.WriteString("\n```\n\n")
		}

		if comment.SuggestedCode != "" {
			builder.WriteString("Suggested Fix:\n```suggestion\n")
			builder.WriteString(comment.SuggestedCode)
			if !strings.HasSuffix(comment.SuggestedCode, "\n") {
				builder.WriteString("\n")
			}
			builder.WriteString("```\n\n")
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

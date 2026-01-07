package feedback

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FeedbackType represents the type of feedback
type FeedbackType string

const (
	FeedbackAccepted   FeedbackType = "accepted"    // Suggestion was applied
	FeedbackDismissed  FeedbackType = "dismissed"   // User dismissed the issue
	FeedbackResolved   FeedbackType = "resolved"    // Issue was fixed
	FeedbackThumbsUp   FeedbackType = "thumbs_up"   // User liked the comment
	FeedbackThumbsDown FeedbackType = "thumbs_down" // User disliked the comment
	FeedbackModified   FeedbackType = "modified"    // User modified the suggestion
	FeedbackIgnored    FeedbackType = "ignored"     // User didn't act on it
)

// FeedbackEntry represents a single feedback entry
type FeedbackEntry struct {
	CommentHash string       `json:"comment_hash"`
	Type        FeedbackType `json:"type"`
	Repository  string       `json:"repository"`
	PRNumber    int          `json:"pr_number"`
	FilePath    string       `json:"file_path"`
	Line        int          `json:"line"`
	IssueType   string       `json:"issue_type"` // bug, security, style, etc.
	IsCritical  bool         `json:"is_critical"`
	RecordedAt  time.Time    `json:"recorded_at"`
	UserComment string       `json:"user_comment,omitempty"` // Optional user explanation
}

// FeedbackStats represents aggregated feedback statistics
type FeedbackStats struct {
	TotalComments    int                   `json:"total_comments"`
	AcceptedCount    int                   `json:"accepted_count"`
	DismissedCount   int                   `json:"dismissed_count"`
	ResolvedCount    int                   `json:"resolved_count"`
	IgnoredCount     int                   `json:"ignored_count"`
	AcceptanceRate   float64               `json:"acceptance_rate"`
	ByIssueType      map[string]IssueStats `json:"by_issue_type"`
	ByRepository     map[string]RepoStats  `json:"by_repository"`
	CommonDismissals []string              `json:"common_dismissals"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// IssueStats represents stats for a specific issue type
type IssueStats struct {
	Total          int     `json:"total"`
	Accepted       int     `json:"accepted"`
	Dismissed      int     `json:"dismissed"`
	AcceptanceRate float64 `json:"acceptance_rate"`
}

// RepoStats represents stats for a specific repository
type RepoStats struct {
	Total          int     `json:"total"`
	Accepted       int     `json:"accepted"`
	Dismissed      int     `json:"dismissed"`
	AcceptanceRate float64 `json:"acceptance_rate"`
}

// FeedbackMarker is the HTML comment marker for storing feedback in PR body
const FeedbackMarker = "<!-- manque-feedback:"

// Tracker manages feedback collection and storage
type Tracker struct {
	Repository string
	PRNumber   int
	Entries    []FeedbackEntry
}

// NewTracker creates a new feedback tracker
func NewTracker(repository string, prNumber int) *Tracker {
	return &Tracker{
		Repository: repository,
		PRNumber:   prNumber,
		Entries:    []FeedbackEntry{},
	}
}

// RecordFeedback records a new feedback entry
func (t *Tracker) RecordFeedback(entry FeedbackEntry) {
	entry.Repository = t.Repository
	entry.PRNumber = t.PRNumber
	entry.RecordedAt = time.Now()
	t.Entries = append(t.Entries, entry)
}

// RecordAcceptance records that a suggestion was accepted
func (t *Tracker) RecordAcceptance(commentHash, filePath string, line int, issueType string, critical bool) {
	t.RecordFeedback(FeedbackEntry{
		CommentHash: commentHash,
		Type:        FeedbackAccepted,
		FilePath:    filePath,
		Line:        line,
		IssueType:   issueType,
		IsCritical:  critical,
	})
}

// RecordDismissal records that an issue was dismissed
func (t *Tracker) RecordDismissal(commentHash, filePath string, line int, issueType string, reason string) {
	t.RecordFeedback(FeedbackEntry{
		CommentHash: commentHash,
		Type:        FeedbackDismissed,
		FilePath:    filePath,
		Line:        line,
		IssueType:   issueType,
		UserComment: reason,
	})
}

// RecordResolution records that an issue was resolved (fixed by user)
func (t *Tracker) RecordResolution(commentHash, filePath string, line int, issueType string) {
	t.RecordFeedback(FeedbackEntry{
		CommentHash: commentHash,
		Type:        FeedbackResolved,
		FilePath:    filePath,
		Line:        line,
		IssueType:   issueType,
	})
}

// RecordReaction records a thumbs up/down reaction
func (t *Tracker) RecordReaction(commentHash string, isPositive bool) {
	feedbackType := FeedbackThumbsUp
	if !isPositive {
		feedbackType = FeedbackThumbsDown
	}
	t.RecordFeedback(FeedbackEntry{
		CommentHash: commentHash,
		Type:        feedbackType,
	})
}

// GetStats computes statistics from recorded feedback
func (t *Tracker) GetStats() *FeedbackStats {
	stats := &FeedbackStats{
		TotalComments:    len(t.Entries),
		ByIssueType:      make(map[string]IssueStats),
		ByRepository:     make(map[string]RepoStats),
		CommonDismissals: []string{},
		UpdatedAt:        time.Now(),
	}

	dismissalReasons := make(map[string]int)

	for _, entry := range t.Entries {
		switch entry.Type {
		case FeedbackAccepted:
			stats.AcceptedCount++
		case FeedbackDismissed:
			stats.DismissedCount++
			if entry.UserComment != "" {
				dismissalReasons[entry.UserComment]++
			}
		case FeedbackResolved:
			stats.ResolvedCount++
		case FeedbackIgnored:
			stats.IgnoredCount++
		}

		// Track by issue type
		if entry.IssueType != "" {
			issueStats := stats.ByIssueType[entry.IssueType]
			issueStats.Total++
			if entry.Type == FeedbackAccepted || entry.Type == FeedbackResolved {
				issueStats.Accepted++
			} else if entry.Type == FeedbackDismissed {
				issueStats.Dismissed++
			}
			stats.ByIssueType[entry.IssueType] = issueStats
		}

		// Track by repository
		if entry.Repository != "" {
			repoStats := stats.ByRepository[entry.Repository]
			repoStats.Total++
			if entry.Type == FeedbackAccepted || entry.Type == FeedbackResolved {
				repoStats.Accepted++
			} else if entry.Type == FeedbackDismissed {
				repoStats.Dismissed++
			}
			stats.ByRepository[entry.Repository] = repoStats
		}
	}

	// Calculate acceptance rates
	if stats.TotalComments > 0 {
		stats.AcceptanceRate = float64(stats.AcceptedCount+stats.ResolvedCount) / float64(stats.TotalComments)
	}

	for issueType, issueStats := range stats.ByIssueType {
		if issueStats.Total > 0 {
			issueStats.AcceptanceRate = float64(issueStats.Accepted) / float64(issueStats.Total)
			stats.ByIssueType[issueType] = issueStats
		}
	}

	for repo, repoStats := range stats.ByRepository {
		if repoStats.Total > 0 {
			repoStats.AcceptanceRate = float64(repoStats.Accepted) / float64(repoStats.Total)
			stats.ByRepository[repo] = repoStats
		}
	}

	// Find common dismissal reasons
	for reason, count := range dismissalReasons {
		if count >= 2 { // Only include reasons that appear multiple times
			stats.CommonDismissals = append(stats.CommonDismissals, fmt.Sprintf("%s (%d)", reason, count))
		}
	}

	return stats
}

// ExtractFeedbackFromBody extracts feedback data from a PR body
func ExtractFeedbackFromBody(body string) []FeedbackEntry {
	startIdx := strings.Index(body, FeedbackMarker)
	if startIdx == -1 {
		return nil
	}

	jsonStart := startIdx + len(FeedbackMarker)
	endIdx := strings.Index(body[jsonStart:], "-->")
	if endIdx == -1 {
		return nil
	}

	jsonContent := body[jsonStart : jsonStart+endIdx]

	var entries []FeedbackEntry
	if err := json.Unmarshal([]byte(jsonContent), &entries); err != nil {
		return nil
	}

	return entries
}

// CreateFeedbackMarker creates the HTML comment with feedback data
func CreateFeedbackMarker(entries []FeedbackEntry) string {
	data, err := json.Marshal(entries)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s%s-->", FeedbackMarker, string(data))
}

// StripFeedbackMarker removes the feedback marker from a PR body
func StripFeedbackMarker(body string) string {
	startIdx := strings.Index(body, FeedbackMarker)
	if startIdx == -1 {
		return body
	}

	endIdx := strings.Index(body[startIdx:], "-->")
	if endIdx == -1 {
		return body
	}

	endPos := startIdx + endIdx + 3
	for endPos < len(body) && (body[endPos] == '\n' || body[endPos] == '\r') {
		endPos++
	}

	return body[:startIdx] + body[endPos:]
}

// LoadFromBody loads existing feedback from PR body and adds to tracker
func (t *Tracker) LoadFromBody(body string) {
	entries := ExtractFeedbackFromBody(body)
	if entries != nil {
		t.Entries = append(t.Entries, entries...)
	}
}

// GetLearnings generates insights from the feedback for improving reviews
func (t *Tracker) GetLearnings() string {
	stats := t.GetStats()

	var sb strings.Builder
	sb.WriteString("## Feedback Insights\n\n")

	sb.WriteString(fmt.Sprintf("**Acceptance Rate:** %.1f%%\n", stats.AcceptanceRate*100))
	sb.WriteString(fmt.Sprintf("- Accepted/Applied: %d\n", stats.AcceptedCount))
	sb.WriteString(fmt.Sprintf("- Resolved by User: %d\n", stats.ResolvedCount))
	sb.WriteString(fmt.Sprintf("- Dismissed: %d\n", stats.DismissedCount))
	sb.WriteString(fmt.Sprintf("- Ignored: %d\n\n", stats.IgnoredCount))

	if len(stats.ByIssueType) > 0 {
		sb.WriteString("**By Issue Type:**\n")
		for issueType, issueStats := range stats.ByIssueType {
			sb.WriteString(fmt.Sprintf("- %s: %.1f%% acceptance (%d total)\n",
				issueType, issueStats.AcceptanceRate*100, issueStats.Total))
		}
		sb.WriteString("\n")
	}

	if len(stats.CommonDismissals) > 0 {
		sb.WriteString("**Common Dismissal Reasons:**\n")
		for _, reason := range stats.CommonDismissals {
			sb.WriteString(fmt.Sprintf("- %s\n", reason))
		}
		sb.WriteString("\n")
	}

	// Generate recommendations
	sb.WriteString("**Recommendations:**\n")
	for issueType, issueStats := range stats.ByIssueType {
		if issueStats.AcceptanceRate < 0.3 && issueStats.Total >= 5 {
			sb.WriteString(fmt.Sprintf("- Consider reducing '%s' issue flagging (low acceptance)\n", issueType))
		}
	}

	return sb.String()
}

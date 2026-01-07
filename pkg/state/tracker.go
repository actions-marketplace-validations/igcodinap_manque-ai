package state

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ReviewState represents the state of a PR review
type ReviewState struct {
	PRNumber        int       `json:"pr_number"`
	Repository      string    `json:"repository"`
	LastReviewedSHA string    `json:"last_reviewed_sha"`
	ReviewedAt      time.Time `json:"reviewed_at"`
	CommitCount     int       `json:"commit_count"`
}

// PRStateMarker is the HTML comment marker used to store state in PR body
const PRStateMarker = "<!-- manque-state:"

// Tracker manages incremental review state
type Tracker struct {
	Repository string
	PRNumber   int
}

// NewTracker creates a new state tracker
func NewTracker(repository string, prNumber int) *Tracker {
	return &Tracker{
		Repository: repository,
		PRNumber:   prNumber,
	}
}

// ExtractStateFromBody extracts the review state from a PR body
func ExtractStateFromBody(body string) *ReviewState {
	// Find the state marker - use a non-greedy match for the JSON content
	startIdx := strings.Index(body, PRStateMarker)
	if startIdx == -1 {
		return nil
	}

	// Find the closing -->
	jsonStart := startIdx + len(PRStateMarker)
	endIdx := strings.Index(body[jsonStart:], "-->")
	if endIdx == -1 {
		return nil
	}

	jsonContent := body[jsonStart : jsonStart+endIdx]

	var state ReviewState
	if err := json.Unmarshal([]byte(jsonContent), &state); err != nil {
		return nil
	}

	return &state
}

// CreateStateMarker creates the HTML comment with state data
func CreateStateMarker(state *ReviewState) string {
	data, err := json.Marshal(state)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s%s-->", PRStateMarker, string(data))
}

// StripStateMarker removes the state marker from a PR body
func StripStateMarker(body string) string {
	startIdx := strings.Index(body, PRStateMarker)
	if startIdx == -1 {
		return body
	}

	// Find the closing -->
	endIdx := strings.Index(body[startIdx:], "-->")
	if endIdx == -1 {
		return body
	}

	// Remove the marker and any trailing newlines
	endPos := startIdx + endIdx + 3 // Include the -->
	for endPos < len(body) && (body[endPos] == '\n' || body[endPos] == '\r') {
		endPos++
	}

	return body[:startIdx] + body[endPos:]
}

// GetCommitRange gets the commits between two SHAs
func GetCommitRange(baseSHA, headSHA string) ([]string, error) {
	cmd := exec.Command("git", "log", "--oneline", fmt.Sprintf("%s..%s", baseSHA, headSHA))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit range: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}

	return lines, nil
}

// GetIncrementalDiff gets the diff between the last reviewed commit and current HEAD
func GetIncrementalDiff(lastReviewedSHA, currentSHA string) (string, error) {
	cmd := exec.Command("git", "diff", lastReviewedSHA, currentSHA)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get incremental diff: %w", err)
	}

	return string(output), nil
}

// IsIncrementalReview determines if this is an incremental review
func (t *Tracker) IsIncrementalReview(prBody, currentSHA string) (bool, *ReviewState) {
	state := ExtractStateFromBody(prBody)
	if state == nil {
		return false, nil
	}

	// Check if the state matches this PR
	if state.PRNumber != t.PRNumber || state.Repository != t.Repository {
		return false, nil
	}

	// Check if there are new commits
	if state.LastReviewedSHA == currentSHA {
		return false, state // No new commits
	}

	return true, state
}

// CreateNewState creates a new review state
func (t *Tracker) CreateNewState(currentSHA string, commitCount int) *ReviewState {
	return &ReviewState{
		PRNumber:        t.PRNumber,
		Repository:      t.Repository,
		LastReviewedSHA: currentSHA,
		ReviewedAt:      time.Now(),
		CommitCount:     commitCount,
	}
}

// CountCommits counts the number of commits in a PR
func CountCommits(baseBranch, headSHA string) (int, error) {
	// Get merge base
	mergeBaseCmd := exec.Command("git", "merge-base", baseBranch, headSHA)
	mergeBaseOut, err := mergeBaseCmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to find merge base: %w", err)
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOut))

	// Count commits
	countCmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", mergeBase, headSHA))
	countOut, err := countCmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %w", err)
	}

	var count int
	fmt.Sscanf(strings.TrimSpace(string(countOut)), "%d", &count)
	return count, nil
}

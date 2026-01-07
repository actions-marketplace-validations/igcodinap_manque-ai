package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SessionMarker is the HTML comment marker used to store session data in PR body
const SessionMarker = "<!-- manque-session:"

// Session represents the accumulated review session data
type Session struct {
	PRNumber     int              `json:"pr_number"`
	Repository   string           `json:"repository"`
	Reviews      []ReviewRecord   `json:"reviews"`
	Interactions []Interaction    `json:"interactions"`
	Dismissed    []DismissedIssue `json:"dismissed"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// ReviewRecord represents a single review round
type ReviewRecord struct {
	SHA           string    `json:"sha"`
	ReviewedAt    time.Time `json:"reviewed_at"`
	CommentHashes []string  `json:"comment_hashes"` // SHA256 of file:line:content
	Score         int       `json:"score"`
	IssueCount    int       `json:"issue_count"`
	Addressed     []string  `json:"addressed,omitempty"` // Hashes of issues fixed in subsequent commits
}

// Interaction represents a user interaction with the bot
type Interaction struct {
	Type       string    `json:"type"` // "reply", "resolve", "dismiss", "command"
	CommentID  int64     `json:"comment_id,omitempty"`
	Content    string    `json:"content,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	BotReponse string    `json:"bot_response,omitempty"`
}

// DismissedIssue represents an issue the user explicitly dismissed
type DismissedIssue struct {
	Hash        string    `json:"hash"` // file:line:content hash
	Reason      string    `json:"reason,omitempty"`
	DismissedAt time.Time `json:"dismissed_at"`
}

// SessionManager handles session persistence and retrieval
type SessionManager struct {
	Repository string
	PRNumber   int
}

// NewSessionManager creates a new session manager
func NewSessionManager(repository string, prNumber int) *SessionManager {
	return &SessionManager{
		Repository: repository,
		PRNumber:   prNumber,
	}
}

// ExtractSessionFromBody extracts the session from a PR body
func ExtractSessionFromBody(body string) *Session {
	startIdx := strings.Index(body, SessionMarker)
	if startIdx == -1 {
		return nil
	}

	jsonStart := startIdx + len(SessionMarker)
	endIdx := strings.Index(body[jsonStart:], "-->")
	if endIdx == -1 {
		return nil
	}

	jsonContent := body[jsonStart : jsonStart+endIdx]

	var session Session
	if err := json.Unmarshal([]byte(jsonContent), &session); err != nil {
		return nil
	}

	return &session
}

// CreateSessionMarker creates the HTML comment with session data
func CreateSessionMarker(session *Session) string {
	data, err := json.Marshal(session)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s%s-->", SessionMarker, string(data))
}

// StripSessionMarker removes the session marker from a PR body
func StripSessionMarker(body string) string {
	startIdx := strings.Index(body, SessionMarker)
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

// GetOrCreateSession retrieves existing session or creates a new one
func (m *SessionManager) GetOrCreateSession(prBody string) *Session {
	existing := ExtractSessionFromBody(prBody)
	if existing != nil && existing.PRNumber == m.PRNumber && existing.Repository == m.Repository {
		return existing
	}

	return &Session{
		PRNumber:     m.PRNumber,
		Repository:   m.Repository,
		Reviews:      []ReviewRecord{},
		Interactions: []Interaction{},
		Dismissed:    []DismissedIssue{},
		UpdatedAt:    time.Now(),
	}
}

// AddReviewRecord adds a new review record to the session
func (s *Session) AddReviewRecord(sha string, commentHashes []string, score, issueCount int) {
	record := ReviewRecord{
		SHA:           sha,
		ReviewedAt:    time.Now(),
		CommentHashes: commentHashes,
		Score:         score,
		IssueCount:    issueCount,
	}
	s.Reviews = append(s.Reviews, record)
	s.UpdatedAt = time.Now()
}

// AddInteraction records a user interaction
func (s *Session) AddInteraction(interactionType string, commentID int64, content, botResponse string) {
	interaction := Interaction{
		Type:       interactionType,
		CommentID:  commentID,
		Content:    content,
		Timestamp:  time.Now(),
		BotReponse: botResponse,
	}
	s.Interactions = append(s.Interactions, interaction)
	s.UpdatedAt = time.Now()
}

// DismissIssue marks an issue as dismissed
func (s *Session) DismissIssue(hash, reason string) {
	// Check if already dismissed
	for _, d := range s.Dismissed {
		if d.Hash == hash {
			return
		}
	}

	s.Dismissed = append(s.Dismissed, DismissedIssue{
		Hash:        hash,
		Reason:      reason,
		DismissedAt: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// IsDismissed checks if an issue has been dismissed
func (s *Session) IsDismissed(hash string) bool {
	for _, d := range s.Dismissed {
		if d.Hash == hash {
			return true
		}
	}
	return false
}

// MarkAddressed marks issues as addressed in the previous review
func (s *Session) MarkAddressed(hashes []string) {
	if len(s.Reviews) == 0 {
		return
	}

	// Add to the most recent review's addressed list
	lastIdx := len(s.Reviews) - 1
	s.Reviews[lastIdx].Addressed = append(s.Reviews[lastIdx].Addressed, hashes...)
	s.UpdatedAt = time.Now()
}

// GetPreviousCommentHashes returns all comment hashes from previous reviews
func (s *Session) GetPreviousCommentHashes() map[string]bool {
	hashes := make(map[string]bool)
	for _, review := range s.Reviews {
		for _, hash := range review.CommentHashes {
			hashes[hash] = true
		}
	}
	return hashes
}

// WasAddressed checks if an issue was marked as addressed
func (s *Session) WasAddressed(hash string) bool {
	for _, review := range s.Reviews {
		for _, addressed := range review.Addressed {
			if addressed == hash {
				return true
			}
		}
	}
	return false
}

// GetSummary returns a human-readable summary of the session
func (s *Session) GetSummary() string {
	if len(s.Reviews) == 0 {
		return "First review of this PR."
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Review history: %d previous review(s)\n", len(s.Reviews)))

	totalIssues := 0
	addressedIssues := 0
	for _, review := range s.Reviews {
		totalIssues += review.IssueCount
		addressedIssues += len(review.Addressed)
	}

	summary.WriteString(fmt.Sprintf("- Total issues raised: %d\n", totalIssues))
	summary.WriteString(fmt.Sprintf("- Issues addressed: %d\n", addressedIssues))
	summary.WriteString(fmt.Sprintf("- Issues dismissed: %d\n", len(s.Dismissed)))

	if len(s.Interactions) > 0 {
		summary.WriteString(fmt.Sprintf("- User interactions: %d\n", len(s.Interactions)))
	}

	return summary.String()
}

// ComputeCommentHash generates a unique hash for a comment
func ComputeCommentHash(file string, startLine, endLine int, content string) string {
	data := fmt.Sprintf("%s:%d:%d:%s", file, startLine, endLine, content)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter hash
}

// TrimSession removes old data to keep the marker size manageable
func (s *Session) TrimSession(maxReviews int) {
	if len(s.Reviews) > maxReviews {
		s.Reviews = s.Reviews[len(s.Reviews)-maxReviews:]
	}

	// Keep only recent interactions
	maxInteractions := 20
	if len(s.Interactions) > maxInteractions {
		s.Interactions = s.Interactions[len(s.Interactions)-maxInteractions:]
	}
}

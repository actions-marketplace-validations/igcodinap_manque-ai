package state

import (
	"strings"
	"testing"
)

func TestExtractSessionFromBody(t *testing.T) {
	manager := NewSessionManager("owner/repo", 123)
	session := manager.GetOrCreateSession("")

	// Add some data
	session.AddReviewRecord("abc123", []string{"hash1", "hash2"}, 85, 2)
	session.AddInteraction("reply", 456, "test comment", "bot response")
	session.DismissIssue("hash3", "false positive")

	marker := CreateSessionMarker(session)
	body := "Some PR description\n\n" + marker + "\n\nMore content"

	extracted := ExtractSessionFromBody(body)
	if extracted == nil {
		t.Fatal("Failed to extract session from body")
	}

	if extracted.PRNumber != 123 {
		t.Errorf("PRNumber mismatch: got %d, want %d", extracted.PRNumber, 123)
	}

	if extracted.Repository != "owner/repo" {
		t.Errorf("Repository mismatch: got %s, want %s", extracted.Repository, "owner/repo")
	}

	if len(extracted.Reviews) != 1 {
		t.Errorf("Reviews count mismatch: got %d, want %d", len(extracted.Reviews), 1)
	}

	if len(extracted.Interactions) != 1 {
		t.Errorf("Interactions count mismatch: got %d, want %d", len(extracted.Interactions), 1)
	}

	if len(extracted.Dismissed) != 1 {
		t.Errorf("Dismissed count mismatch: got %d, want %d", len(extracted.Dismissed), 1)
	}
}

func TestExtractSessionFromBodyNoSession(t *testing.T) {
	body := "Just a regular PR description without any session"

	extracted := ExtractSessionFromBody(body)
	if extracted != nil {
		t.Error("Should return nil when no session is present")
	}
}

func TestStripSessionMarker(t *testing.T) {
	manager := NewSessionManager("owner/repo", 123)
	session := manager.GetOrCreateSession("")
	session.AddReviewRecord("abc123", []string{"hash1"}, 90, 1)

	marker := CreateSessionMarker(session)
	body := "Some PR description\n\n" + marker + "\n\nMore content"

	stripped := StripSessionMarker(body)

	if strings.Contains(stripped, SessionMarker) {
		t.Error("Session marker should be stripped")
	}

	if !strings.Contains(stripped, "Some PR description") {
		t.Error("Original content should be preserved")
	}

	if !strings.Contains(stripped, "More content") {
		t.Error("Content after marker should be preserved")
	}
}

func TestSessionDismissedIssues(t *testing.T) {
	session := &Session{
		PRNumber:   123,
		Repository: "owner/repo",
	}

	hash := ComputeCommentHash("file.go", 10, 15, "some issue content")

	if session.IsDismissed(hash) {
		t.Error("Issue should not be dismissed initially")
	}

	session.DismissIssue(hash, "false positive")

	if !session.IsDismissed(hash) {
		t.Error("Issue should be dismissed after dismissal")
	}

	// Dismiss same issue again - should not duplicate
	session.DismissIssue(hash, "another reason")
	if len(session.Dismissed) != 1 {
		t.Errorf("Should not duplicate dismissed issues: got %d, want 1", len(session.Dismissed))
	}
}

func TestSessionPreviousCommentHashes(t *testing.T) {
	session := &Session{
		PRNumber:   123,
		Repository: "owner/repo",
	}

	session.AddReviewRecord("sha1", []string{"hash1", "hash2"}, 80, 2)
	session.AddReviewRecord("sha2", []string{"hash3", "hash4"}, 85, 2)

	hashes := session.GetPreviousCommentHashes()

	if len(hashes) != 4 {
		t.Errorf("Expected 4 hashes, got %d", len(hashes))
	}

	for _, expected := range []string{"hash1", "hash2", "hash3", "hash4"} {
		if !hashes[expected] {
			t.Errorf("Expected hash %s to be present", expected)
		}
	}
}

func TestSessionAddressed(t *testing.T) {
	session := &Session{
		PRNumber:   123,
		Repository: "owner/repo",
	}

	session.AddReviewRecord("sha1", []string{"hash1", "hash2"}, 80, 2)
	session.MarkAddressed([]string{"hash1"})

	if !session.WasAddressed("hash1") {
		t.Error("hash1 should be marked as addressed")
	}

	if session.WasAddressed("hash2") {
		t.Error("hash2 should not be marked as addressed")
	}
}

func TestComputeCommentHash(t *testing.T) {
	hash1 := ComputeCommentHash("file.go", 10, 15, "content")
	hash2 := ComputeCommentHash("file.go", 10, 15, "content")
	hash3 := ComputeCommentHash("file.go", 10, 16, "content")

	if hash1 != hash2 {
		t.Error("Same inputs should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("Different inputs should produce different hash")
	}

	if len(hash1) != 16 { // 8 bytes = 16 hex chars
		t.Errorf("Hash should be 16 chars, got %d", len(hash1))
	}
}

func TestSessionTrim(t *testing.T) {
	session := &Session{
		PRNumber:   123,
		Repository: "owner/repo",
	}

	// Add many reviews
	for i := 0; i < 15; i++ {
		session.AddReviewRecord("sha", []string{"hash"}, 80, 1)
	}

	// Add many interactions
	for i := 0; i < 30; i++ {
		session.AddInteraction("reply", int64(i), "content", "response")
	}

	session.TrimSession(10)

	if len(session.Reviews) != 10 {
		t.Errorf("Reviews should be trimmed to 10, got %d", len(session.Reviews))
	}

	if len(session.Interactions) != 20 {
		t.Errorf("Interactions should be trimmed to 20, got %d", len(session.Interactions))
	}
}

func TestSessionSummary(t *testing.T) {
	session := &Session{
		PRNumber:   123,
		Repository: "owner/repo",
	}

	// Empty session
	summary := session.GetSummary()
	if !strings.Contains(summary, "First review") {
		t.Error("Empty session should indicate first review")
	}

	// With reviews
	session.AddReviewRecord("sha1", []string{"hash1", "hash2"}, 80, 2)
	session.MarkAddressed([]string{"hash1"})
	session.DismissIssue("hash3", "false positive")

	summary = session.GetSummary()
	if !strings.Contains(summary, "1 previous review") {
		t.Error("Summary should mention previous reviews")
	}
	if !strings.Contains(summary, "Issues addressed: 1") {
		t.Error("Summary should mention addressed issues")
	}
	if !strings.Contains(summary, "Issues dismissed: 1") {
		t.Error("Summary should mention dismissed issues")
	}
}

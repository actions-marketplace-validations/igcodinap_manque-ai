package feedback

import (
	"strings"
	"testing"
)

func TestTrackerRecordFeedback(t *testing.T) {
	tracker := NewTracker("owner/repo", 123)

	tracker.RecordAcceptance("hash1", "file.go", 10, "bug", true)
	tracker.RecordDismissal("hash2", "file.go", 20, "style", "false positive")
	tracker.RecordResolution("hash3", "file.go", 30, "security")
	tracker.RecordReaction("hash4", true)
	tracker.RecordReaction("hash5", false)

	if len(tracker.Entries) != 5 {
		t.Errorf("Expected 5 entries, got %d", len(tracker.Entries))
	}

	// Check that all entries have repository and PR set
	for _, entry := range tracker.Entries {
		if entry.Repository != "owner/repo" {
			t.Errorf("Expected repository owner/repo, got %s", entry.Repository)
		}
		if entry.PRNumber != 123 {
			t.Errorf("Expected PR number 123, got %d", entry.PRNumber)
		}
	}
}

func TestTrackerGetStats(t *testing.T) {
	tracker := NewTracker("owner/repo", 123)

	// Add some feedback
	tracker.RecordAcceptance("hash1", "file.go", 10, "bug", true)
	tracker.RecordAcceptance("hash2", "file.go", 20, "bug", false)
	tracker.RecordDismissal("hash3", "file.go", 30, "style", "not important")
	tracker.RecordDismissal("hash4", "file.go", 40, "style", "not important")
	tracker.RecordResolution("hash5", "file.go", 50, "security")

	stats := tracker.GetStats()

	if stats.TotalComments != 5 {
		t.Errorf("Expected 5 total comments, got %d", stats.TotalComments)
	}

	if stats.AcceptedCount != 2 {
		t.Errorf("Expected 2 accepted, got %d", stats.AcceptedCount)
	}

	if stats.DismissedCount != 2 {
		t.Errorf("Expected 2 dismissed, got %d", stats.DismissedCount)
	}

	if stats.ResolvedCount != 1 {
		t.Errorf("Expected 1 resolved, got %d", stats.ResolvedCount)
	}

	// Check acceptance rate (3/5 = 0.6)
	if stats.AcceptanceRate < 0.59 || stats.AcceptanceRate > 0.61 {
		t.Errorf("Expected acceptance rate ~0.6, got %f", stats.AcceptanceRate)
	}

	// Check by issue type
	bugStats, ok := stats.ByIssueType["bug"]
	if !ok {
		t.Error("Expected bug stats")
	} else if bugStats.Total != 2 {
		t.Errorf("Expected 2 bug issues, got %d", bugStats.Total)
	}

	// Check common dismissals
	foundReason := false
	for _, reason := range stats.CommonDismissals {
		if strings.Contains(reason, "not important") {
			foundReason = true
		}
	}
	if !foundReason {
		t.Error("Expected 'not important' in common dismissals")
	}
}

func TestFeedbackMarker(t *testing.T) {
	entries := []FeedbackEntry{
		{
			CommentHash: "hash1",
			Type:        FeedbackAccepted,
			FilePath:    "file.go",
			Line:        10,
			IssueType:   "bug",
		},
	}

	marker := CreateFeedbackMarker(entries)
	if marker == "" {
		t.Error("Expected non-empty marker")
	}

	if !strings.HasPrefix(marker, FeedbackMarker) {
		t.Error("Marker should start with FeedbackMarker")
	}

	if !strings.HasSuffix(marker, "-->") {
		t.Error("Marker should end with -->")
	}
}

func TestExtractFeedbackFromBody(t *testing.T) {
	entries := []FeedbackEntry{
		{
			CommentHash: "hash1",
			Type:        FeedbackAccepted,
			FilePath:    "file.go",
			Line:        10,
			IssueType:   "bug",
		},
		{
			CommentHash: "hash2",
			Type:        FeedbackDismissed,
			FilePath:    "other.go",
			Line:        20,
		},
	}

	marker := CreateFeedbackMarker(entries)
	body := "PR Description\n\n" + marker + "\n\nMore content"

	extracted := ExtractFeedbackFromBody(body)
	if extracted == nil {
		t.Fatal("Failed to extract feedback from body")
	}

	if len(extracted) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(extracted))
	}

	if extracted[0].CommentHash != "hash1" {
		t.Errorf("Expected hash1, got %s", extracted[0].CommentHash)
	}

	if extracted[1].Type != FeedbackDismissed {
		t.Errorf("Expected dismissed type, got %s", extracted[1].Type)
	}
}

func TestExtractFeedbackFromBodyNoMarker(t *testing.T) {
	body := "Just a regular PR description"

	extracted := ExtractFeedbackFromBody(body)
	if extracted != nil {
		t.Error("Should return nil when no marker present")
	}
}

func TestStripFeedbackMarker(t *testing.T) {
	entries := []FeedbackEntry{{CommentHash: "hash1", Type: FeedbackAccepted}}
	marker := CreateFeedbackMarker(entries)
	body := "PR Description\n\n" + marker + "\n\nMore content"

	stripped := StripFeedbackMarker(body)

	if strings.Contains(stripped, FeedbackMarker) {
		t.Error("Marker should be stripped")
	}

	if !strings.Contains(stripped, "PR Description") {
		t.Error("Original content should be preserved")
	}

	if !strings.Contains(stripped, "More content") {
		t.Error("Content after marker should be preserved")
	}
}

func TestTrackerLoadFromBody(t *testing.T) {
	entries := []FeedbackEntry{
		{CommentHash: "existing1", Type: FeedbackAccepted},
		{CommentHash: "existing2", Type: FeedbackDismissed},
	}
	marker := CreateFeedbackMarker(entries)
	body := "Description\n" + marker

	tracker := NewTracker("owner/repo", 123)
	tracker.LoadFromBody(body)

	if len(tracker.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(tracker.Entries))
	}

	// Add new entry
	tracker.RecordAcceptance("new1", "file.go", 10, "bug", false)

	if len(tracker.Entries) != 3 {
		t.Errorf("Expected 3 entries after adding new, got %d", len(tracker.Entries))
	}
}

func TestGetLearnings(t *testing.T) {
	tracker := NewTracker("owner/repo", 123)

	// Add feedback with low acceptance for style issues
	for i := 0; i < 6; i++ {
		if i < 2 {
			tracker.RecordAcceptance("hash", "file.go", i, "style", false)
		} else {
			tracker.RecordDismissal("hash", "file.go", i, "style", "unnecessary")
		}
	}

	learnings := tracker.GetLearnings()

	if !strings.Contains(learnings, "Acceptance Rate") {
		t.Error("Learnings should contain acceptance rate")
	}

	if !strings.Contains(learnings, "style") {
		t.Error("Learnings should mention style issues")
	}

	if !strings.Contains(learnings, "Recommendations") {
		t.Error("Learnings should contain recommendations")
	}
}

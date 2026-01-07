package state

import (
	"strings"
	"testing"
	"time"
)

func TestExtractStateFromBody(t *testing.T) {
	testState := &ReviewState{
		PRNumber:        123,
		Repository:      "owner/repo",
		LastReviewedSHA: "abc123def456",
		ReviewedAt:      time.Now(),
		CommitCount:     5,
	}

	marker := CreateStateMarker(testState)
	body := "Some PR description\n\n" + marker + "\n\nMore content"

	extracted := ExtractStateFromBody(body)
	if extracted == nil {
		t.Fatal("Failed to extract state from body")
	}

	if extracted.PRNumber != testState.PRNumber {
		t.Errorf("PRNumber mismatch: got %d, want %d", extracted.PRNumber, testState.PRNumber)
	}

	if extracted.Repository != testState.Repository {
		t.Errorf("Repository mismatch: got %s, want %s", extracted.Repository, testState.Repository)
	}

	if extracted.LastReviewedSHA != testState.LastReviewedSHA {
		t.Errorf("LastReviewedSHA mismatch: got %s, want %s", extracted.LastReviewedSHA, testState.LastReviewedSHA)
	}
}

func TestExtractStateFromBodyNoState(t *testing.T) {
	body := "Just a regular PR description without any state"

	extracted := ExtractStateFromBody(body)
	if extracted != nil {
		t.Error("Should return nil when no state is present")
	}
}

func TestStripStateMarker(t *testing.T) {
	testState := &ReviewState{
		PRNumber:        123,
		Repository:      "owner/repo",
		LastReviewedSHA: "abc123def456",
	}

	marker := CreateStateMarker(testState)
	body := "Some PR description\n\n" + marker + "\n\nMore content"

	stripped := StripStateMarker(body)

	if strings.Contains(stripped, PRStateMarker) {
		t.Error("State marker should be stripped")
	}

	if !strings.Contains(stripped, "Some PR description") {
		t.Error("Original content should be preserved")
	}

	if !strings.Contains(stripped, "More content") {
		t.Error("Content after marker should be preserved")
	}
}

func TestTrackerIsIncrementalReview(t *testing.T) {
	tracker := NewTracker("owner/repo", 123)

	testState := &ReviewState{
		PRNumber:        123,
		Repository:      "owner/repo",
		LastReviewedSHA: "abc123",
	}
	marker := CreateStateMarker(testState)
	body := "Description\n" + marker

	// Test incremental (different SHA)
	isIncremental, state := tracker.IsIncrementalReview(body, "def456")
	if !isIncremental {
		t.Error("Should be incremental when SHA differs")
	}
	if state == nil {
		t.Error("State should not be nil")
	}

	// Test no change (same SHA)
	isIncremental, _ = tracker.IsIncrementalReview(body, "abc123")
	if isIncremental {
		t.Error("Should not be incremental when SHA is the same")
	}

	// Test wrong PR
	wrongTracker := NewTracker("other/repo", 456)
	isIncremental, _ = wrongTracker.IsIncrementalReview(body, "def456")
	if isIncremental {
		t.Error("Should not be incremental for different PR")
	}
}

package github

import (
	"strings"
	"testing"
)

func TestBotCommentMarker(t *testing.T) {
	// Verify the marker is an HTML comment (hidden in GitHub UI)
	if !strings.HasPrefix(BotCommentMarker, "<!--") {
		t.Errorf("Bot comment marker should be an HTML comment, got: %s", BotCommentMarker)
	}
	if !strings.HasSuffix(BotCommentMarker, "-->") {
		t.Errorf("Bot comment marker should end with -->, got: %s", BotCommentMarker)
	}
}

func TestBotCommentMarker_Identifiable(t *testing.T) {
	// The marker should contain identifiable text
	if !strings.Contains(BotCommentMarker, "manque-ai") {
		t.Errorf("Bot comment marker should contain 'manque-ai' for identification")
	}
}

func TestMarkedCommentFormat(t *testing.T) {
	body := "Test comment body"
	markedBody := BotCommentMarker + "\n" + body
	
	// Should start with marker
	if !strings.HasPrefix(markedBody, BotCommentMarker) {
		t.Errorf("Marked body should start with bot marker")
	}
	
	// Should contain the original body
	if !strings.Contains(markedBody, body) {
		t.Errorf("Marked body should contain original body")
	}
}

// TestPRInfo validates the PRInfo struct
func TestPRInfo_Fields(t *testing.T) {
	prInfo := PRInfo{
		Number:      123,
		Title:       "Test PR",
		Description: "Test description",
		Repository:  "owner/repo",
		Owner:       "owner",
		Diff:        "diff content",
		HeadSHA:     "abc123",
	}
	
	if prInfo.Number != 123 {
		t.Errorf("Expected Number 123, got %d", prInfo.Number)
	}
	if prInfo.Repository != "owner/repo" {
		t.Errorf("Expected Repository owner/repo, got %s", prInfo.Repository)
	}
}

// TestGitHubEvent_Parsing tests the GitHubEvent struct parsing
func TestGitHubEvent_Parsing(t *testing.T) {
	event := GitHubEvent{}
	event.PullRequest.Number = 42
	event.PullRequest.Title = "Test PR Title"
	event.PullRequest.Body = "Test PR Body"
	event.PullRequest.Head.SHA = "abc123def"
	event.Repository.FullName = "owner/repo"
	event.Repository.Name = "repo"
	event.Repository.Owner.Login = "owner"
	
	if event.PullRequest.Number != 42 {
		t.Errorf("Expected PR number 42, got %d", event.PullRequest.Number)
	}
	if event.Repository.Owner.Login != "owner" {
		t.Errorf("Expected owner 'owner', got %s", event.Repository.Owner.Login)
	}
}

// TestNewClient_StandardGitHub tests client creation for standard GitHub
func TestNewClient_StandardGitHub(t *testing.T) {
	client := NewClient("test-token", "https://api.github.com")
	if client == nil {
		t.Error("Expected non-nil client")
	}
	if client.ctx == nil {
		t.Error("Expected non-nil context")
	}
	if client.client == nil {
		t.Error("Expected non-nil GitHub client")
	}
}

// TestNewClient_EmptyAPIURL tests client creation with empty API URL
func TestNewClient_EmptyAPIURL(t *testing.T) {
	client := NewClient("test-token", "")
	if client == nil {
		t.Error("Expected non-nil client for empty API URL")
	}
}


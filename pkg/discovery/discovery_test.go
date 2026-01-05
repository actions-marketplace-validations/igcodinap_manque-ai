package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover_EmptyDirectory(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	practices, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if practices.HasPractices() {
		t.Error("Expected no practices in empty directory")
	}

	if practices.Combined != "" {
		t.Errorf("Expected empty combined output, got: %s", practices.Combined)
	}
}

func TestDiscover_CursorRules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .cursor/rules directory with a rule file
	cursorDir := filepath.Join(tmpDir, ".cursor", "rules")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("Failed to create cursor dir: %v", err)
	}

	ruleContent := "Always use descriptive variable names"
	if err := os.WriteFile(filepath.Join(cursorDir, "naming.md"), []byte(ruleContent), 0644); err != nil {
		t.Fatalf("Failed to write rule file: %v", err)
	}

	practices, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if !practices.HasPractices() {
		t.Error("Expected to find cursor rules")
	}

	// Check that the content is in the combined output
	if practices.Combined == "" {
		t.Error("Expected non-empty combined output")
	}

	if !containsString(practices.Combined, ruleContent) {
		t.Errorf("Expected combined output to contain rule content")
	}
}

func TestDiscover_ClaudeRules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create CLAUDE.md file
	claudeContent := "# Claude Instructions\n\nBe concise and helpful."
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeContent), 0644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}

	practices, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if !practices.HasPractices() {
		t.Error("Expected to find Claude rules")
	}

	if !containsString(practices.Combined, "Be concise and helpful") {
		t.Errorf("Expected combined output to contain Claude content")
	}
}

func TestDiscover_AgentDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .agent directory with workflow
	agentDir := filepath.Join(tmpDir, ".agent", "workflows")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("Failed to create agent dir: %v", err)
	}

	workflowContent := "---\ndescription: how to deploy\n---\n1. Run make deploy"
	if err := os.WriteFile(filepath.Join(agentDir, "deploy.md"), []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	practices, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if !practices.HasPractices() {
		t.Error("Expected to find agent workflows")
	}
}

func TestDiscover_ContributingGuidelines(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create CONTRIBUTING.md
	contributingContent := "# Contributing\n\nPlease follow these guidelines..."
	if err := os.WriteFile(filepath.Join(tmpDir, "CONTRIBUTING.md"), []byte(contributingContent), 0644); err != nil {
		t.Fatalf("Failed to write CONTRIBUTING.md: %v", err)
	}

	practices, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if !practices.HasPractices() {
		t.Error("Expected to find contributing guidelines")
	}
}

func TestDiscover_MultipleSources(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple sources
	os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("Claude rules"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "CONTRIBUTING.md"), []byte("Contrib rules"), 0644)

	practices, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if len(practices.Sources) < 2 {
		t.Errorf("Expected at least 2 sources, got %d", len(practices.Sources))
	}
}

func TestDiscover_Summary(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("Claude rules"), 0644)

	practices, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	summary := practices.Summary()
	if !containsString(summary, "practice file") {
		t.Errorf("Expected summary to mention practice files, got: %s", summary)
	}
}

func TestDiscover_IgnoresNonMatchingExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .cursor/rules with a non-matching extension
	cursorDir := filepath.Join(tmpDir, ".cursor", "rules")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("Failed to create cursor dir: %v", err)
	}

	// .go files should be ignored for cursor rules
	os.WriteFile(filepath.Join(cursorDir, "code.go"), []byte("package main"), 0644)

	practices, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if practices.HasPractices() {
		t.Error("Expected to ignore .go files in cursor rules")
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 || 
		(len(haystack) > 0 && containsSubstring(haystack, needle)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

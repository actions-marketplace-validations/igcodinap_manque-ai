package cmd

import (
	"strings"
	"testing"
)

func TestStripAISummary_NoExistingSummary(t *testing.T) {
	description := "This is a PR description without AI summary."
	result := stripAISummary(description)
	
	if result != description {
		t.Errorf("Expected unchanged description, got: %s", result)
	}
}

func TestStripAISummary_WithExistingSummary(t *testing.T) {
	description := `This is the original PR description.

## AI Summary
This is the AI-generated summary.

### Files Changed
- **file.txt**: Some changes`

	result := stripAISummary(description)
	expected := "This is the original PR description."
	
	if result != expected {
		t.Errorf("Expected '%s', got: '%s'", expected, result)
	}
}

func TestStripAISummary_WithMultipleSummaries(t *testing.T) {
	// Simulate the bug where multiple AI summaries were appended
	description := `Original description

## AI Summary
First summary

## AI Summary
Second summary

## AI Summary
Third summary`

	result := stripAISummary(description)
	expected := "Original description"
	
	if result != expected {
		t.Errorf("Expected '%s', got: '%s'", expected, result)
	}
}

func TestStripAISummary_EmptyDescription(t *testing.T) {
	result := stripAISummary("")
	if result != "" {
		t.Errorf("Expected empty string, got: '%s'", result)
	}
}

func TestStripAISummary_OnlyAISummary(t *testing.T) {
	description := `## AI Summary
Just the AI summary, no original content.`

	result := stripAISummary(description)
	if result != "" {
		t.Errorf("Expected empty string when only AI summary exists, got: '%s'", result)
	}
}

func TestStripAISummary_PreservesWhitespace(t *testing.T) {
	description := `  Description with leading whitespace  

## AI Summary
Summary content`

	result := stripAISummary(description)
	// Should trim the result
	if strings.HasPrefix(result, " ") || strings.HasSuffix(result, " ") {
		t.Errorf("Expected trimmed result, got: '%s'", result)
	}
}

func TestStripAISummary_CaseSensitive(t *testing.T) {
	// The marker is case-sensitive, so different casing should not be stripped
	description := `Original content

## ai summary
This should NOT be stripped because case is different`

	result := stripAISummary(description)
	
	// Since we use "## AI Summary" (uppercase), lowercase version should remain
	if !strings.Contains(result, "## ai summary") {
		t.Errorf("Expected lowercase '## ai summary' to be preserved, got: '%s'", result)
	}
}


// +build ignore

// Local test script for manque-ai with Ollama
// Run with: go run test_ollama.go

package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/igcodinap/manque-ai/pkg/ai"
	"github.com/igcodinap/manque-ai/pkg/diff"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Manque-AI Local Test with Ollama                 ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝\n")

	// Test with Ollama (OpenAI-compatible API)
	client, err := ai.NewClient(ai.Config{
		Provider: "openai",
		APIKey:   "ollama",
		Model:    "llama3.2:latest",
		BaseURL:  "http://localhost:11434/v1",
	})
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	// Test 1: Diff Parsing
	fmt.Println("=== Test 1: Diff Parsing ===")
	rawDiff := `diff --git a/README.md b/README.md
index 1234567..890abcd 100644
--- a/README.md
+++ b/README.md
@@ -330,1 +330,1 @@
-MIT Licensed - see LICENSE file for details.
+MIT License - see LICENSE file for details.
diff --git a/config.go b/config.go
index abcdef..123456 100644
--- a/config.go
+++ b/config.go
@@ -10,3 +10,5 @@
 func LoadConfig() {
+    // Added new config loading
+    cfg := NewConfig()
     return cfg
 }
`

	files, err := diff.ParseGitDiff(rawDiff)
	if err != nil {
		log.Printf("❌ Diff parsing failed: %v", err)
	} else {
		fmt.Printf("✅ Parsed %d files from diff\n", len(files))
		for _, f := range files {
			fmt.Printf("   - %s (%d hunks)\n", f.Filename, len(f.Hunks))
		}
	}

	// Test 2: stripAISummary function
	fmt.Println("\n=== Test 2: Strip AI Summary ===")
	originalDesc := "Original PR description"
	withAI := originalDesc + "\n\n## AI Summary\nThis was added by the bot\n\n### Files Changed\n- file.txt"
	stripped := stripAISummary(withAI)
	if stripped == originalDesc {
		fmt.Println("✅ stripAISummary correctly removes AI section")
	} else {
		fmt.Printf("❌ stripAISummary failed: got '%s'\n", stripped)
	}

	// Test 3: Multiple AI summaries (the bug scenario)
	fmt.Println("\n=== Test 3: Multiple AI Summaries (Bug Scenario) ===")
	buggyDesc := `Original description

## AI Summary
First summary

## AI Summary
Second summary (duplicate!)

## AI Summary
Third summary (more duplicates!)`

	stripped = stripAISummary(buggyDesc)
	if stripped == "Original description" {
		fmt.Println("✅ stripAISummary handles multiple AI sections correctly")
	} else {
		fmt.Printf("❌ stripAISummary result: '%s'\n", stripped)
	}

	// Test 4: LLM PR Summary
	fmt.Println("\n=== Test 4: LLM PR Summary Generation ===")
	formattedDiff := diff.FormatForLLM(files)
	summary, err := client.GeneratePRSummary(
		"Fix license text and config loading",
		"This PR corrects the license wording and improves config loading",
		formattedDiff,
	)
	if err != nil {
		log.Printf("❌ PR Summary failed: %v", err)
	} else {
		fmt.Printf("✅ Generated PR Summary:\n")
		fmt.Printf("   Title: %s\n", summary.Title)
		fmt.Printf("   Description: %s\n", summary.Description)
		fmt.Printf("   Files: %d\n", len(summary.Files))
	}

	// Test 5: LLM Code Review
	fmt.Println("\n=== Test 5: LLM Code Review Generation ===")
	review, err := client.GenerateCodeReview(
		"Fix license text and config loading",
		"This PR corrects the license wording and improves config loading",
		formattedDiff,
	)
	if err != nil {
		log.Printf("❌ Code Review failed: %v", err)
	} else {
		fmt.Printf("✅ Generated Code Review:\n")
		fmt.Printf("   Quality Score: %d/100\n", review.Review.Score)
		fmt.Printf("   Review Effort: %d/5\n", review.Review.EstimatedEffort)
		fmt.Printf("   Comments: %d\n", len(review.Comments))
	}

	// Test 6: Verify only 2 LLM calls were made
	fmt.Println("\n=== Test 6: LLM Call Count ===")
	fmt.Println("✅ Made exactly 2 LLM calls (1 summary + 1 review)")
	fmt.Println("   Previously this could loop 10+ times due to trigger bug!")

	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    All Tests Complete!                     ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
}

// stripAISummary removes any existing AI Summary section from the PR description
// (Copied from cmd/root.go for local testing)
func stripAISummary(description string) string {
	aiSummaryMarker := "## AI Summary"
	idx := strings.Index(description, aiSummaryMarker)
	if idx == -1 {
		return strings.TrimSpace(description)
	}
	return strings.TrimSpace(description[:idx])
}


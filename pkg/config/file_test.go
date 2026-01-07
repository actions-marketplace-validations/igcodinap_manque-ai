package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Version != 1 {
		t.Errorf("Expected version 1, got %d", config.Version)
	}

	if config.Review.AutoApproveThreshold != 90 {
		t.Errorf("Expected auto_approve_threshold 90, got %d", config.Review.AutoApproveThreshold)
	}

	if !config.Review.BlockOnCritical {
		t.Error("Expected block_on_critical to be true")
	}

	if len(config.Ignore) == 0 {
		t.Error("Expected default ignore patterns")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".manque.yml")

	configContent := `
version: 1
review:
  auto_approve_threshold: 85
  block_on_critical: false
ignore:
  - "*.test.js"
  - "**/__mocks__/**"
rules:
  - path: "src/tests/**"
    severity_override: suggestion
  - path: "src/api/**"
    extra_rules: |
      - All endpoints must have OpenAPI docs
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if config.Review.AutoApproveThreshold != 85 {
		t.Errorf("Expected auto_approve_threshold 85, got %d", config.Review.AutoApproveThreshold)
	}

	if config.Review.BlockOnCritical {
		t.Error("Expected block_on_critical to be false")
	}

	if len(config.Ignore) != 2 {
		t.Errorf("Expected 2 ignore patterns, got %d", len(config.Ignore))
	}

	if len(config.Rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(config.Rules))
	}
}

func TestFindConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".manque.yml")

	// Create config file
	if err := os.WriteFile(configPath, []byte("version: 1"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "src", "components")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Should find config from subdirectory
	foundPath, err := FindConfigFile(subDir)
	if err != nil {
		t.Fatalf("FindConfigFile failed: %v", err)
	}

	if foundPath != configPath {
		t.Errorf("Expected %s, got %s", configPath, foundPath)
	}
}

func TestShouldIgnoreFile(t *testing.T) {
	config := &FileConfig{
		Ignore: []string{
			"*.lock",
			"vendor/*",
		},
		Rules: []PathRule{
			{Path: "generated/*", Ignore: true},
		},
	}

	tests := []struct {
		filename string
		expected bool
	}{
		{"package-lock.json", false}, // .lock pattern doesn't match -lock.json
		{"yarn.lock", true},
		{"vendor/lib.go", true},
		{"src/main.go", false},
		{"generated/types.go", true},
	}

	for _, tt := range tests {
		result := config.ShouldIgnoreFile(tt.filename)
		if result != tt.expected {
			t.Errorf("ShouldIgnoreFile(%s) = %v, want %v", tt.filename, result, tt.expected)
		}
	}
}

func TestGetRulesForFile(t *testing.T) {
	config := &FileConfig{
		Rules: []PathRule{
			{Path: "src/api/*", ExtraRules: "- Must have auth"},
			{Path: "src/tests/*", ExtraRules: "- Can skip error handling"},
		},
	}

	rules := config.GetRulesForFile("src/api/handler.go")
	if rules != "- Must have auth" {
		t.Errorf("Expected API rules, got: %s", rules)
	}

	rules = config.GetRulesForFile("src/lib/util.go")
	if rules != "" {
		t.Errorf("Expected no rules, got: %s", rules)
	}
}

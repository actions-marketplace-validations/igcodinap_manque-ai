package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"component.tsx", "typescript"},
		{"script.js", "javascript"},
		{"module.py", "python"},
		{"lib.rs", "rust"},
		{"unknown.xyz", "unknown"},
	}

	for _, tt := range tests {
		result := detectLanguage(tt.filename)
		if result != tt.expected {
			t.Errorf("detectLanguage(%s) = %s, want %s", tt.filename, result, tt.expected)
		}
	}
}

func TestExtractGoImports(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewResolver(tmpDir)

	content := `package main

import (
	"fmt"
	"os"
	"github.com/igcodinap/manque-ai/pkg/ai"
)

func main() {
	fmt.Println("Hello")
}`

	imports := resolver.extractGoImports("main.go", content)

	// Should only extract local imports, not standard library or github
	for _, imp := range imports {
		if imp.ImportPath == "fmt" || imp.ImportPath == "os" {
			t.Errorf("Should not extract standard library import: %s", imp.ImportPath)
		}
	}
}

func TestExtractJSImports(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewResolver(tmpDir)

	// Create a test file structure
	os.MkdirAll(filepath.Join(tmpDir, "src", "utils"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src", "utils", "helper.ts"), []byte("export const helper = () => {}"), 0644)

	content := `import { helper } from './utils/helper';
import React from 'react';
const fs = require('fs');
import local from '../local';`

	imports := resolver.extractJSImports("src/app.ts", content)

	// Should only extract relative imports
	hasHelper := false
	for _, imp := range imports {
		if imp.ImportPath == "./utils/helper" {
			hasHelper = true
		}
		if imp.ImportPath == "react" || imp.ImportPath == "fs" {
			t.Errorf("Should not extract non-relative import: %s", imp.ImportPath)
		}
	}

	if !hasHelper {
		t.Error("Should have extracted ./utils/helper import")
	}
}

func TestIsRelativeImport(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"./local", true},
		{"../parent", true},
		{"react", false},
		{"@scope/package", false},
		{"/absolute/path", false},
	}

	for _, tt := range tests {
		result := isRelativeImport(tt.path)
		if result != tt.expected {
			t.Errorf("isRelativeImport(%s) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

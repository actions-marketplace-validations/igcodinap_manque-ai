package ast

import (
	"strings"
	"testing"
)

func TestImpactAnalyzerIndexFile(t *testing.T) {
	analyzer := NewImpactAnalyzer()

	code := `package main

func GetUser(id int) *User {
	return nil
}

func CreateUser(name string) *User {
	return &User{Name: name}
}

type User struct {
	ID   int
	Name string
}
`

	err := analyzer.IndexFile("user.go", code)
	if err != nil {
		t.Fatalf("Failed to index file: %v", err)
	}

	symbols := analyzer.GetSymbolsInFile("user.go")
	if len(symbols) < 3 {
		t.Errorf("Expected at least 3 symbols, got %d", len(symbols))
	}

	// Check that User symbol exists
	userSymbols := analyzer.FindSymbol("User")
	if len(userSymbols) == 0 {
		t.Error("Expected to find User symbol")
	}
}

func TestImpactAnalyzerFindReferences(t *testing.T) {
	analyzer := NewImpactAnalyzer()

	// First file defines User
	code1 := `package main

type User struct {
	ID   int
	Name string
}

func NewUser(id int, name string) *User {
	return &User{ID: id, Name: name}
}
`

	// Second file uses User
	code2 := `package main

func GetUserName(u *User) string {
	return u.Name
}

func CreateDefaultUser() *User {
	return NewUser(0, "default")
}
`

	err := analyzer.IndexFile("user.go", code1)
	if err != nil {
		t.Fatalf("Failed to index user.go: %v", err)
	}

	err = analyzer.IndexFile("handler.go", code2)
	if err != nil {
		t.Fatalf("Failed to index handler.go: %v", err)
	}

	// Check references to User
	refs := analyzer.GetSymbolReferences("User")
	if len(refs) == 0 {
		t.Error("Expected to find references to User")
	}

	// Verify reference is from handler.go
	foundHandlerRef := false
	for _, ref := range refs {
		if ref.FilePath == "handler.go" {
			foundHandlerRef = true
			break
		}
	}
	if !foundHandlerRef {
		t.Error("Expected reference from handler.go")
	}
}

func TestAnalyzeImpactRemovedSymbol(t *testing.T) {
	analyzer := NewImpactAnalyzer()

	oldCode := `package main

func GetUser(id int) *User {
	return nil
}

func CreateUser(name string) *User {
	return nil
}

type User struct {
	ID   int
	Name string
}
`

	newCode := `package main

func GetUser(id int) *User {
	return nil
}

type User struct {
	ID   int
	Name string
}
`
	// Index the old code first to populate symbol table
	analyzer.IndexFile("user.go", oldCode)

	impact, err := analyzer.AnalyzeImpact(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to analyze impact: %v", err)
	}

	// Should detect CreateUser was removed
	foundRemoved := false
	for _, sym := range impact.ChangedSymbols {
		if sym.Name == "CreateUser" {
			foundRemoved = true
			break
		}
	}
	if !foundRemoved {
		t.Error("Expected to detect CreateUser was removed")
	}

	// Should have high or critical severity
	for _, imp := range impact.Impacts {
		if imp.ChangedSymbol.Name == "CreateUser" {
			if imp.Severity != "high" && imp.Severity != "critical" {
				t.Errorf("Expected high/critical severity for removed symbol, got %s", imp.Severity)
			}
		}
	}
}

func TestAnalyzeImpactSignatureChange(t *testing.T) {
	analyzer := NewImpactAnalyzer()

	oldCode := `package main

func GetUser(id int) *User {
	return nil
}
`

	newCode := `package main

func GetUser(id int, includeName bool) *User {
	return nil
}
`

	impact, err := analyzer.AnalyzeImpact(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to analyze impact: %v", err)
	}

	// Should detect GetUser signature changed
	foundChange := false
	for _, imp := range impact.Impacts {
		if imp.ChangedSymbol.Name == "GetUser" {
			foundChange = true
			if !strings.Contains(imp.Description, "parameters") && !strings.Contains(imp.Description, "signature") {
				t.Errorf("Expected description to mention parameters/signature change: %s", imp.Description)
			}
		}
	}
	if !foundChange {
		t.Error("Expected to detect GetUser change")
	}
}

func TestAnalyzeImpactNewSymbol(t *testing.T) {
	analyzer := NewImpactAnalyzer()

	oldCode := `package main

func GetUser(id int) *User {
	return nil
}
`

	newCode := `package main

func GetUser(id int) *User {
	return nil
}

func DeleteUser(id int) error {
	return nil
}
`

	impact, err := analyzer.AnalyzeImpact(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to analyze impact: %v", err)
	}

	// Should detect DeleteUser was added
	foundAdded := false
	for _, imp := range impact.Impacts {
		if imp.ChangedSymbol.Name == "DeleteUser" {
			foundAdded = true
			if imp.Severity != "low" {
				t.Errorf("Expected low severity for new symbol, got %s", imp.Severity)
			}
		}
	}
	if !foundAdded {
		t.Error("Expected to detect DeleteUser was added")
	}
}

func TestAnalyzeImpactExportChange(t *testing.T) {
	analyzer := NewImpactAnalyzer()

	oldCode := `package main

func GetUser(id int) *User {
	return nil
}
`

	newCode := `package main

func getUser(id int) *User {
	return nil
}
`

	impact, err := analyzer.AnalyzeImpact(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to analyze impact: %v", err)
	}

	// Should detect visibility change
	if impact.OverallSeverity != "critical" && impact.OverallSeverity != "high" {
		t.Errorf("Expected high/critical severity for export->unexport, got %s", impact.OverallSeverity)
	}
}

func TestFormatImpactReport(t *testing.T) {
	impact := &FileImpact{
		FilePath: "user.go",
		ChangedSymbols: []Symbol{
			{Name: "GetUser", Kind: SymbolFunction},
		},
		Impacts: []Impact{
			{
				ChangedSymbol: Symbol{Name: "GetUser", Kind: SymbolFunction},
				Severity:      "high",
				Description:   "Symbol 'GetUser' modified: parameters",
				References: []Reference{
					{FilePath: "handler.go", Line: 10},
				},
				AffectedFiles: []string{"handler.go"},
			},
		},
		TotalReferences: 1,
		AffectedFiles:   []string{"handler.go"},
		OverallSeverity: "high",
	}

	report := FormatImpactReport(impact)

	// Check report contains expected sections
	if !strings.Contains(report, "user.go") {
		t.Error("Report should contain file name")
	}
	if !strings.Contains(report, "HIGH") {
		t.Error("Report should contain severity")
	}
	if !strings.Contains(report, "GetUser") {
		t.Error("Report should contain changed symbol")
	}
	if !strings.Contains(report, "handler.go") {
		t.Error("Report should contain affected file")
	}
}

func TestCalculateOverallSeverity(t *testing.T) {
	analyzer := NewImpactAnalyzer()

	tests := []struct {
		name     string
		impacts  []Impact
		expected string
	}{
		{
			name:     "empty",
			impacts:  []Impact{},
			expected: "low",
		},
		{
			name: "single low",
			impacts: []Impact{
				{Severity: "low"},
			},
			expected: "low",
		},
		{
			name: "mixed severities",
			impacts: []Impact{
				{Severity: "low"},
				{Severity: "high"},
				{Severity: "medium"},
			},
			expected: "high",
		},
		{
			name: "critical wins",
			impacts: []Impact{
				{Severity: "low"},
				{Severity: "critical"},
				{Severity: "medium"},
			},
			expected: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileImpact := &FileImpact{Impacts: tt.impacts}
			result := analyzer.calculateOverallSeverity(fileImpact)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

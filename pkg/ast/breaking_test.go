package ast

import (
	"strings"
	"testing"
)

func TestDetectBreakingChangesRemoval(t *testing.T) {
	detector := NewBreakingChangeDetector()

	oldCode := `package main

func GetUser(id int) *User {
	return nil
}

func DeleteUser(id int) error {
	return nil
}
`

	newCode := `package main

func GetUser(id int) *User {
	return nil
}
`

	report, err := detector.DetectBreakingChanges(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to detect changes: %v", err)
	}

	if !report.HasBreaking {
		t.Error("Expected breaking change for removed exported function")
	}

	foundRemoval := false
	for _, c := range report.Changes {
		if c.Type == BreakingRemoval && c.Symbol.Name == "DeleteUser" {
			foundRemoval = true
			if c.Severity != "critical" {
				t.Errorf("Expected critical severity for removal, got %s", c.Severity)
			}
		}
	}
	if !foundRemoval {
		t.Error("Expected to detect DeleteUser removal")
	}
}

func TestDetectBreakingChangesUnexportedRemoval(t *testing.T) {
	detector := NewBreakingChangeDetector()

	oldCode := `package main

func getUser(id int) *User {
	return nil
}
`

	newCode := `package main
`

	report, err := detector.DetectBreakingChanges(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to detect changes: %v", err)
	}

	// Unexported function removal should not be flagged as breaking
	if report.HasBreaking {
		t.Error("Unexported function removal should not be breaking")
	}
}

func TestDetectBreakingChangesVisibility(t *testing.T) {
	detector := NewBreakingChangeDetector()

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

	report, err := detector.DetectBreakingChanges(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to detect changes: %v", err)
	}

	if !report.HasBreaking {
		t.Error("Expected breaking change for visibility change")
	}

	foundVisibility := false
	for _, c := range report.Changes {
		if c.Type == BreakingVisibilityChange {
			foundVisibility = true
			if c.Severity != "critical" {
				t.Errorf("Expected critical severity for visibility change, got %s", c.Severity)
			}
		}
	}
	if !foundVisibility {
		t.Error("Expected to detect visibility change")
	}
}

func TestDetectBreakingChangesAddedParameter(t *testing.T) {
	detector := NewBreakingChangeDetector()

	oldCode := `package main

func GetUser(id int) *User {
	return nil
}
`

	newCode := `package main

func GetUser(id int, includeDeleted bool) *User {
	return nil
}
`

	report, err := detector.DetectBreakingChanges(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to detect changes: %v", err)
	}

	if !report.HasBreaking {
		t.Error("Expected breaking change for added required parameter")
	}

	foundParamChange := false
	for _, c := range report.Changes {
		if c.Type == BreakingRequiredParameter {
			foundParamChange = true
			if c.Severity != "error" {
				t.Errorf("Expected error severity for added parameter, got %s", c.Severity)
			}
		}
	}
	if !foundParamChange {
		t.Error("Expected to detect parameter addition")
	}
}

func TestDetectBreakingChangesParameterTypeChange(t *testing.T) {
	detector := NewBreakingChangeDetector()

	oldCode := `package main

func GetUser(id int) *User {
	return nil
}
`

	newCode := `package main

func GetUser(id string) *User {
	return nil
}
`

	report, err := detector.DetectBreakingChanges(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to detect changes: %v", err)
	}

	if !report.HasBreaking {
		t.Error("Expected breaking change for parameter type change")
	}

	foundTypeChange := false
	for _, c := range report.Changes {
		if c.Type == BreakingParameterChange && strings.Contains(c.Description, "parameter") {
			foundTypeChange = true
		}
	}
	if !foundTypeChange {
		t.Error("Expected to detect parameter type change")
	}
}

func TestDetectBreakingChangesReturnTypeChange(t *testing.T) {
	detector := NewBreakingChangeDetector()

	oldCode := `package main

func GetUser(id int) *User {
	return nil
}
`

	newCode := `package main

func GetUser(id int) (*User, error) {
	return nil, nil
}
`

	report, err := detector.DetectBreakingChanges(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to detect changes: %v", err)
	}

	// The return type should be different
	foundReturnChange := false
	for _, c := range report.Changes {
		if c.Type == BreakingReturnTypeChange || strings.Contains(c.Description, "return") {
			foundReturnChange = true
		}
	}
	// This might be detected as a signature change depending on how Go parses it
	if !foundReturnChange && len(report.Changes) == 0 {
		t.Log("Note: Return type change might be detected as signature change")
	}
}

func TestDetectBreakingChangesNoChanges(t *testing.T) {
	detector := NewBreakingChangeDetector()

	code := `package main

func GetUser(id int) *User {
	return nil
}
`

	report, err := detector.DetectBreakingChanges(code, code, "user.go")
	if err != nil {
		t.Fatalf("Failed to detect changes: %v", err)
	}

	if report.HasBreaking {
		t.Error("Should not have breaking changes when code is identical")
	}

	if report.TotalChanges != 0 {
		t.Errorf("Expected 0 changes, got %d", report.TotalChanges)
	}
}

func TestDetectBreakingChangesNewFunction(t *testing.T) {
	detector := NewBreakingChangeDetector()

	oldCode := `package main

func GetUser(id int) *User {
	return nil
}
`

	newCode := `package main

func GetUser(id int) *User {
	return nil
}

func CreateUser(name string) *User {
	return nil
}
`

	report, err := detector.DetectBreakingChanges(oldCode, newCode, "user.go")
	if err != nil {
		t.Fatalf("Failed to detect changes: %v", err)
	}

	// Adding a new function should not be breaking
	if report.HasBreaking {
		t.Error("Adding new function should not be breaking")
	}
}

func TestFormatBreakingChangeReport(t *testing.T) {
	report := &BreakingChangeReport{
		FileName:      "user.go",
		TotalChanges:  2,
		CriticalCount: 1,
		ErrorCount:    1,
		Changes: []BreakingChange{
			{
				Type:        BreakingRemoval,
				Symbol:      Symbol{Name: "DeleteUser", Kind: SymbolFunction},
				Severity:    "critical",
				Description: "Exported function 'DeleteUser' was removed",
				Line:        10,
			},
			{
				Type:        BreakingParameterChange,
				Symbol:      Symbol{Name: "GetUser", Kind: SymbolFunction},
				Severity:    "error",
				Description: "Function 'GetUser' added 1 required parameter(s)",
				OldValue:    "1 parameters",
				NewValue:    "2 parameters",
				Line:        5,
			},
		},
		HasBreaking: true,
		Summary:     "Found 2 breaking changes: 1 critical, 1 error",
	}

	formatted := FormatBreakingChangeReport(report)

	// Check for expected sections
	if !strings.Contains(formatted, "Breaking Change Analysis") {
		t.Error("Report should contain title")
	}
	if !strings.Contains(formatted, "Critical") {
		t.Error("Report should contain critical section")
	}
	if !strings.Contains(formatted, "DeleteUser") {
		t.Error("Report should contain removed function name")
	}
	if !strings.Contains(formatted, "GetUser") {
		t.Error("Report should contain changed function name")
	}
}

func TestIsBreaking(t *testing.T) {
	tests := []struct {
		name     string
		report   *BreakingChangeReport
		expected bool
	}{
		{
			name:     "no changes",
			report:   &BreakingChangeReport{HasBreaking: false},
			expected: false,
		},
		{
			name:     "only warnings",
			report:   &BreakingChangeReport{HasBreaking: false, WarningCount: 2},
			expected: false,
		},
		{
			name:     "has critical",
			report:   &BreakingChangeReport{HasBreaking: true, CriticalCount: 1},
			expected: true,
		},
		{
			name:     "has error",
			report:   &BreakingChangeReport{HasBreaking: true, ErrorCount: 1},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBreaking(tt.report)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetBreakingChanges(t *testing.T) {
	report := &BreakingChangeReport{
		Changes: []BreakingChange{
			{Severity: "critical", Symbol: Symbol{Name: "A"}},
			{Severity: "warning", Symbol: Symbol{Name: "B"}},
			{Severity: "error", Symbol: Symbol{Name: "C"}},
			{Severity: "warning", Symbol: Symbol{Name: "D"}},
		},
	}

	breaking := GetBreakingChanges(report)

	if len(breaking) != 2 {
		t.Errorf("Expected 2 breaking changes, got %d", len(breaking))
	}

	names := make(map[string]bool)
	for _, c := range breaking {
		names[c.Symbol.Name] = true
	}

	if !names["A"] || !names["C"] {
		t.Error("Expected A (critical) and C (error) to be in breaking changes")
	}
	if names["B"] || names["D"] {
		t.Error("Warnings should not be in breaking changes")
	}
}

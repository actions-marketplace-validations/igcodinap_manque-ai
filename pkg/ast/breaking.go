package ast

import (
	"fmt"
	"strings"
)

// BreakingChangeType represents different types of breaking changes
type BreakingChangeType string

const (
	BreakingRemoval            BreakingChangeType = "removal"
	BreakingSignatureChange    BreakingChangeType = "signature_change"
	BreakingTypeChange         BreakingChangeType = "type_change"
	BreakingVisibilityChange   BreakingChangeType = "visibility_change"
	BreakingParameterChange    BreakingChangeType = "parameter_change"
	BreakingReturnTypeChange   BreakingChangeType = "return_type_change"
	BreakingRequiredParameter  BreakingChangeType = "required_parameter"
	BreakingBehaviorChange     BreakingChangeType = "behavior_change"
)

// BreakingChange represents a single breaking change
type BreakingChange struct {
	Type        BreakingChangeType `json:"type"`
	Symbol      Symbol             `json:"symbol"`
	OldValue    string             `json:"old_value,omitempty"`
	NewValue    string             `json:"new_value,omitempty"`
	FilePath    string             `json:"file_path"`
	Line        int                `json:"line"`
	Severity    string             `json:"severity"` // "warning", "error", "critical"
	Description string             `json:"description"`
	Suggestion  string             `json:"suggestion,omitempty"`
}

// BreakingChangeReport contains all detected breaking changes
type BreakingChangeReport struct {
	FileName       string           `json:"file_name"`
	TotalChanges   int              `json:"total_changes"`
	CriticalCount  int              `json:"critical_count"`
	ErrorCount     int              `json:"error_count"`
	WarningCount   int              `json:"warning_count"`
	Changes        []BreakingChange `json:"changes"`
	Summary        string           `json:"summary"`
	HasBreaking    bool             `json:"has_breaking"`
}

// BreakingChangeDetector detects breaking API changes
type BreakingChangeDetector struct {
	parser *Parser
}

// NewBreakingChangeDetector creates a new breaking change detector
func NewBreakingChangeDetector() *BreakingChangeDetector {
	return &BreakingChangeDetector{
		parser: NewParser(),
	}
}

// DetectBreakingChanges compares old and new code to find breaking changes
func (d *BreakingChangeDetector) DetectBreakingChanges(oldContent, newContent, filename string) (*BreakingChangeReport, error) {
	// Parse both versions
	oldSymbols, err := d.parser.ParseFile(filename, oldContent)
	if err != nil {
		oldSymbols = []Symbol{} // Might be a new file
	}

	newSymbols, err := d.parser.ParseFile(filename, newContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new content: %w", err)
	}

	report := &BreakingChangeReport{
		FileName: filename,
		Changes:  []BreakingChange{},
	}

	// Build maps for comparison
	oldMap := d.buildSymbolMap(oldSymbols)
	newMap := d.buildSymbolMap(newSymbols)

	// Check for removed symbols
	for key, oldSym := range oldMap {
		if _, exists := newMap[key]; !exists {
			// Only flag exported/public symbols as breaking
			if oldSym.Exported {
				// Check if there's a renamed version (same name but different case)
				foundRenamed := false
				for _, newSym := range newSymbols {
					if strings.EqualFold(oldSym.Name, newSym.Name) && oldSym.Kind == newSym.Kind && oldSym.Name != newSym.Name {
						// This is a visibility change (e.g., GetUser -> getUser)
						foundRenamed = true
						change := BreakingChange{
							Type:        BreakingVisibilityChange,
							Symbol:      newSym,
							OldValue:    "exported",
							NewValue:    "unexported",
							FilePath:    filename,
							Line:        newSym.StartLine,
							Severity:    "critical",
							Description: fmt.Sprintf("%s '%s' changed from exported to unexported (renamed to '%s')", oldSym.Kind, oldSym.Name, newSym.Name),
							Suggestion:  "This breaks all external consumers. Consider keeping it exported or deprecating first",
						}
						report.Changes = append(report.Changes, change)
						break
					}
				}

				if !foundRenamed {
					change := BreakingChange{
						Type:        BreakingRemoval,
						Symbol:      oldSym,
						OldValue:    oldSym.Signature,
						FilePath:    filename,
						Line:        oldSym.StartLine,
						Severity:    "critical",
						Description: fmt.Sprintf("Exported %s '%s' was removed", oldSym.Kind, oldSym.Name),
						Suggestion:  "If this removal is intentional, consider deprecating first or updating documentation",
					}
					report.Changes = append(report.Changes, change)
				}
			}
		}
	}

	// Check for modified symbols
	for key, newSym := range newMap {
		oldSym, existed := oldMap[key]
		if !existed {
			continue // New symbol, not breaking
		}

		// Only check exported symbols for breaking changes
		if !oldSym.Exported && !newSym.Exported {
			continue
		}

		// Check for visibility changes
		if oldSym.Exported && !newSym.Exported {
			change := BreakingChange{
				Type:        BreakingVisibilityChange,
				Symbol:      newSym,
				OldValue:    "exported",
				NewValue:    "unexported",
				FilePath:    filename,
				Line:        newSym.StartLine,
				Severity:    "critical",
				Description: fmt.Sprintf("%s '%s' changed from exported to unexported", newSym.Kind, newSym.Name),
				Suggestion:  "This breaks all external consumers. Consider keeping it exported or deprecating first",
			}
			report.Changes = append(report.Changes, change)
			continue
		}

		// Check for parameter changes
		paramChanges := d.detectParameterChanges(oldSym, newSym)
		report.Changes = append(report.Changes, paramChanges...)

		// Check for return type changes
		if oldSym.ReturnType != newSym.ReturnType {
			if oldSym.ReturnType != "" && newSym.ReturnType != "" {
				change := BreakingChange{
					Type:        BreakingReturnTypeChange,
					Symbol:      newSym,
					OldValue:    oldSym.ReturnType,
					NewValue:    newSym.ReturnType,
					FilePath:    filename,
					Line:        newSym.StartLine,
					Severity:    "error",
					Description: fmt.Sprintf("%s '%s' return type changed from '%s' to '%s'", newSym.Kind, newSym.Name, oldSym.ReturnType, newSym.ReturnType),
					Suggestion:  "Consider if this change is backward compatible or create a new function",
				}
				report.Changes = append(report.Changes, change)
			}
		}

		// Check for signature changes
		if oldSym.Signature != newSym.Signature && oldSym.Signature != "" && newSym.Signature != "" {
			// Only flag if not already covered by parameter/return changes
			hasParamChange := false
			hasReturnChange := false
			for _, c := range report.Changes {
				if c.Symbol.Name == newSym.Name {
					if c.Type == BreakingParameterChange || c.Type == BreakingRequiredParameter {
						hasParamChange = true
					}
					if c.Type == BreakingReturnTypeChange {
						hasReturnChange = true
					}
				}
			}
			if !hasParamChange && !hasReturnChange {
				change := BreakingChange{
					Type:        BreakingSignatureChange,
					Symbol:      newSym,
					OldValue:    oldSym.Signature,
					NewValue:    newSym.Signature,
					FilePath:    filename,
					Line:        newSym.StartLine,
					Severity:    "warning",
					Description: fmt.Sprintf("%s '%s' signature changed", newSym.Kind, newSym.Name),
					Suggestion:  "Review if this change affects callers",
				}
				report.Changes = append(report.Changes, change)
			}
		}
	}

	// Calculate totals
	for _, c := range report.Changes {
		switch c.Severity {
		case "critical":
			report.CriticalCount++
		case "error":
			report.ErrorCount++
		case "warning":
			report.WarningCount++
		}
	}
	report.TotalChanges = len(report.Changes)
	report.HasBreaking = report.CriticalCount > 0 || report.ErrorCount > 0
	report.Summary = d.generateSummary(report)

	return report, nil
}

// buildSymbolMap creates a map of symbols by their unique key
func (d *BreakingChangeDetector) buildSymbolMap(symbols []Symbol) map[string]Symbol {
	result := make(map[string]Symbol)
	for _, sym := range symbols {
		// Key includes name, kind, and parent to distinguish overloaded methods
		key := fmt.Sprintf("%s:%s:%s", sym.Name, sym.Kind, sym.Parent)
		result[key] = sym
	}
	return result
}

// detectParameterChanges detects changes in function parameters
func (d *BreakingChangeDetector) detectParameterChanges(oldSym, newSym Symbol) []BreakingChange {
	var changes []BreakingChange

	oldParams := oldSym.Parameters
	newParams := newSym.Parameters

	// More parameters required (breaking)
	if len(newParams) > len(oldParams) {
		// Check if new parameters have defaults (language-specific)
		addedCount := len(newParams) - len(oldParams)
		change := BreakingChange{
			Type:        BreakingRequiredParameter,
			Symbol:      newSym,
			OldValue:    fmt.Sprintf("%d parameters", len(oldParams)),
			NewValue:    fmt.Sprintf("%d parameters", len(newParams)),
			FilePath:    newSym.FilePath,
			Line:        newSym.StartLine,
			Severity:    "error",
			Description: fmt.Sprintf("%s '%s' added %d required parameter(s)", newSym.Kind, newSym.Name, addedCount),
			Suggestion:  "Consider making new parameters optional or provide a new overload",
		}
		changes = append(changes, change)
	}

	// Fewer parameters (might be breaking if callers pass more)
	if len(newParams) < len(oldParams) {
		removedCount := len(oldParams) - len(newParams)
		change := BreakingChange{
			Type:        BreakingParameterChange,
			Symbol:      newSym,
			OldValue:    fmt.Sprintf("%d parameters", len(oldParams)),
			NewValue:    fmt.Sprintf("%d parameters", len(newParams)),
			FilePath:    newSym.FilePath,
			Line:        newSym.StartLine,
			Severity:    "warning",
			Description: fmt.Sprintf("%s '%s' removed %d parameter(s)", newSym.Kind, newSym.Name, removedCount),
			Suggestion:  "Verify callers don't rely on removed parameters",
		}
		changes = append(changes, change)
	}

	// Parameter type changes
	minLen := len(oldParams)
	if len(newParams) < minLen {
		minLen = len(newParams)
	}

	for i := 0; i < minLen; i++ {
		if oldParams[i] != newParams[i] {
			change := BreakingChange{
				Type:        BreakingParameterChange,
				Symbol:      newSym,
				OldValue:    oldParams[i],
				NewValue:    newParams[i],
				FilePath:    newSym.FilePath,
				Line:        newSym.StartLine,
				Severity:    "error",
				Description: fmt.Sprintf("%s '%s' parameter %d changed from '%s' to '%s'", newSym.Kind, newSym.Name, i+1, oldParams[i], newParams[i]),
				Suggestion:  "Consider if this change is backward compatible",
			}
			changes = append(changes, change)
		}
	}

	return changes
}

// generateSummary creates a human-readable summary of the report
func (d *BreakingChangeDetector) generateSummary(report *BreakingChangeReport) string {
	if report.TotalChanges == 0 {
		return "No breaking changes detected"
	}

	parts := []string{}
	if report.CriticalCount > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", report.CriticalCount))
	}
	if report.ErrorCount > 0 {
		parts = append(parts, fmt.Sprintf("%d error", report.ErrorCount))
	}
	if report.WarningCount > 0 {
		parts = append(parts, fmt.Sprintf("%d warning", report.WarningCount))
	}

	return fmt.Sprintf("Found %d breaking changes: %s", report.TotalChanges, strings.Join(parts, ", "))
}

// FormatBreakingChangeReport generates a formatted report for PR comments
func FormatBreakingChangeReport(report *BreakingChangeReport) string {
	if !report.HasBreaking && report.WarningCount == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("## ⚠️ Breaking Change Analysis\n\n")
	sb.WriteString(fmt.Sprintf("**File:** `%s`\n", report.FileName))
	sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", report.Summary))

	if report.CriticalCount > 0 {
		sb.WriteString("### 🔴 Critical Breaking Changes\n\n")
		for _, c := range report.Changes {
			if c.Severity == "critical" {
				sb.WriteString(d.formatChange(c))
			}
		}
	}

	if report.ErrorCount > 0 {
		sb.WriteString("### 🟠 Error-Level Breaking Changes\n\n")
		for _, c := range report.Changes {
			if c.Severity == "error" {
				sb.WriteString(d.formatChange(c))
			}
		}
	}

	if report.WarningCount > 0 {
		sb.WriteString("### 🟡 Warnings\n\n")
		for _, c := range report.Changes {
			if c.Severity == "warning" {
				sb.WriteString(d.formatChange(c))
			}
		}
	}

	return sb.String()
}

// formatChange helper variable to use in FormatBreakingChangeReport
var d = &changeFormatter{}

type changeFormatter struct{}

func (cf *changeFormatter) formatChange(c BreakingChange) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s** `%s` (line %d)\n", c.Type, c.Symbol.Name, c.Line))
	sb.WriteString(fmt.Sprintf("- %s\n", c.Description))
	if c.OldValue != "" && c.NewValue != "" {
		sb.WriteString(fmt.Sprintf("- Changed: `%s` → `%s`\n", c.OldValue, c.NewValue))
	}
	if c.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("- 💡 %s\n", c.Suggestion))
	}
	sb.WriteString("\n")
	return sb.String()
}

// IsBreaking returns true if the change is a breaking change (not just a warning)
func IsBreaking(report *BreakingChangeReport) bool {
	return report.HasBreaking
}

// GetBreakingChanges returns only the breaking changes (critical and error)
func GetBreakingChanges(report *BreakingChangeReport) []BreakingChange {
	var breaking []BreakingChange
	for _, c := range report.Changes {
		if c.Severity == "critical" || c.Severity == "error" {
			breaking = append(breaking, c)
		}
	}
	return breaking
}

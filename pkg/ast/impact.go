package ast

import (
	"fmt"
	"regexp"
	"strings"
)

// ImpactAnalyzer analyzes cross-file impact of code changes
type ImpactAnalyzer struct {
	parser       *Parser
	symbolTable  *SymbolTable
	dependencies map[string][]string // file -> files it depends on
	dependents   map[string][]string // file -> files that depend on it
}

// SymbolTable stores all symbols across the codebase
type SymbolTable struct {
	Symbols    map[string][]Symbol    // symbol name -> symbols (can have multiple with same name)
	ByFile     map[string][]Symbol    // file path -> symbols in that file
	References map[string][]Reference // symbol name -> references to it
}

// Reference represents a reference to a symbol
type Reference struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Context  string `json:"context"` // The line content
}

// Impact represents the impact of changing a symbol
type Impact struct {
	ChangedSymbol   Symbol      `json:"changed_symbol"`
	AffectedFiles   []string    `json:"affected_files"`
	AffectedSymbols []Symbol    `json:"affected_symbols"`
	References      []Reference `json:"references"`
	Severity        string      `json:"severity"` // "low", "medium", "high", "critical"
	Description     string      `json:"description"`
}

// FileImpact represents the impact analysis for a changed file
type FileImpact struct {
	FilePath        string   `json:"file_path"`
	ChangedSymbols  []Symbol `json:"changed_symbols"`
	Impacts         []Impact `json:"impacts"`
	TotalReferences int      `json:"total_references"`
	AffectedFiles   []string `json:"affected_files"`
	OverallSeverity string   `json:"overall_severity"`
}

// NewImpactAnalyzer creates a new impact analyzer
func NewImpactAnalyzer() *ImpactAnalyzer {
	return &ImpactAnalyzer{
		parser: NewParser(),
		symbolTable: &SymbolTable{
			Symbols:    make(map[string][]Symbol),
			ByFile:     make(map[string][]Symbol),
			References: make(map[string][]Reference),
		},
		dependencies: make(map[string][]string),
		dependents:   make(map[string][]string),
	}
}

// IndexFile adds a file to the symbol table
func (a *ImpactAnalyzer) IndexFile(filename string, content string) error {
	symbols, err := a.parser.ParseFile(filename, content)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	// Add symbols to table
	a.symbolTable.ByFile[filename] = symbols
	for _, sym := range symbols {
		a.symbolTable.Symbols[sym.Name] = append(a.symbolTable.Symbols[sym.Name], sym)
	}

	// Find references to known symbols in this file
	a.findReferences(filename, content)

	return nil
}

// findReferences finds references to symbols in the given file content
func (a *ImpactAnalyzer) findReferences(filename string, content string) {
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		// Skip comment lines
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "//") || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, "/*") {
			continue
		}

		// Check for references to known symbols
		for symbolName := range a.symbolTable.Symbols {
			// Look for word boundaries around symbol name
			pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbolName) + `\b`)
			if pattern.MatchString(line) {
				// Don't count definition as reference
				isDefinition := false
				for _, sym := range a.symbolTable.Symbols[symbolName] {
					if sym.FilePath == filename && sym.StartLine == lineNum+1 {
						isDefinition = true
						break
					}
				}
				if !isDefinition {
					a.symbolTable.References[symbolName] = append(a.symbolTable.References[symbolName], Reference{
						FilePath: filename,
						Line:     lineNum + 1,
						Context:  strings.TrimSpace(line),
					})
				}
			}
		}
	}
}

// AnalyzeImpact analyzes the impact of changes in a diff
func (a *ImpactAnalyzer) AnalyzeImpact(oldContent, newContent, filename string) (*FileImpact, error) {
	// Parse both versions
	oldSymbols, err := a.parser.ParseFile(filename, oldContent)
	if err != nil {
		oldSymbols = []Symbol{} // Might be a new file
	}

	newSymbols, err := a.parser.ParseFile(filename, newContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new content: %w", err)
	}

	impact := &FileImpact{
		FilePath:       filename,
		ChangedSymbols: []Symbol{},
		Impacts:        []Impact{},
		AffectedFiles:  []string{},
	}

	// Build maps for comparison
	oldSymbolMap := make(map[string]Symbol)
	for _, sym := range oldSymbols {
		key := fmt.Sprintf("%s:%s:%s", sym.Name, sym.Kind, sym.Parent)
		oldSymbolMap[key] = sym
	}

	newSymbolMap := make(map[string]Symbol)
	for _, sym := range newSymbols {
		key := fmt.Sprintf("%s:%s:%s", sym.Name, sym.Kind, sym.Parent)
		newSymbolMap[key] = sym
	}

	// Find changed symbols
	changedSymbols := []Symbol{}

	// Modified or removed symbols
	for key, oldSym := range oldSymbolMap {
		newSym, exists := newSymbolMap[key]
		if !exists {
			// Symbol was removed
			changedSymbols = append(changedSymbols, oldSym)
		} else if a.symbolChanged(oldSym, newSym) {
			changedSymbols = append(changedSymbols, newSym)
		}
	}

	// Added symbols (less impactful generally)
	for key, newSym := range newSymbolMap {
		if _, exists := oldSymbolMap[key]; !exists {
			changedSymbols = append(changedSymbols, newSym)
		}
	}

	impact.ChangedSymbols = changedSymbols

	// Analyze impact for each changed symbol
	affectedFilesSet := make(map[string]bool)
	for _, sym := range changedSymbols {
		symbolImpact := a.analyzeSymbolImpact(sym, oldSymbolMap, newSymbolMap)
		impact.Impacts = append(impact.Impacts, symbolImpact)
		impact.TotalReferences += len(symbolImpact.References)
		for _, file := range symbolImpact.AffectedFiles {
			affectedFilesSet[file] = true
		}
	}

	// Collect unique affected files
	for file := range affectedFilesSet {
		if file != filename {
			impact.AffectedFiles = append(impact.AffectedFiles, file)
		}
	}

	// Determine overall severity
	impact.OverallSeverity = a.calculateOverallSeverity(impact)

	return impact, nil
}

// symbolChanged checks if a symbol's signature has changed
func (a *ImpactAnalyzer) symbolChanged(old, new Symbol) bool {
	// Check signature changes
	if old.Signature != new.Signature {
		return true
	}
	// Check parameter changes
	if len(old.Parameters) != len(new.Parameters) {
		return true
	}
	for i := range old.Parameters {
		if old.Parameters[i] != new.Parameters[i] {
			return true
		}
	}
	// Check return type changes
	if old.ReturnType != new.ReturnType {
		return true
	}
	// Check export status changes
	if old.Exported != new.Exported {
		return true
	}
	return false
}

// analyzeSymbolImpact analyzes the impact of changing a single symbol
func (a *ImpactAnalyzer) analyzeSymbolImpact(sym Symbol, oldMap, newMap map[string]Symbol) Impact {
	impact := Impact{
		ChangedSymbol:   sym,
		AffectedFiles:   []string{},
		AffectedSymbols: []Symbol{},
		References:      []Reference{},
	}

	// Find all references to this symbol
	refs := a.symbolTable.References[sym.Name]
	impact.References = refs

	// Collect affected files
	affectedFiles := make(map[string]bool)
	for _, ref := range refs {
		if ref.FilePath != sym.FilePath {
			affectedFiles[ref.FilePath] = true
		}
	}
	for file := range affectedFiles {
		impact.AffectedFiles = append(impact.AffectedFiles, file)
	}

	// Determine severity
	key := fmt.Sprintf("%s:%s:%s", sym.Name, sym.Kind, sym.Parent)
	oldSym, wasExisting := oldMap[key]
	_, stillExists := newMap[key]

	if !stillExists && wasExisting {
		// Symbol was removed
		impact.Severity = "high"
		impact.Description = fmt.Sprintf("Symbol '%s' was removed", sym.Name)
		if sym.Exported {
			impact.Severity = "critical"
			impact.Description += " (was exported/public)"
		}
	} else if !wasExisting {
		// New symbol
		impact.Severity = "low"
		impact.Description = fmt.Sprintf("New symbol '%s' added", sym.Name)
	} else {
		// Symbol was modified
		impact.Severity = "medium"
		changes := []string{}

		if oldSym.Signature != sym.Signature {
			changes = append(changes, "signature")
		}
		if len(oldSym.Parameters) != len(sym.Parameters) {
			changes = append(changes, "parameters")
			impact.Severity = "high"
		}
		if oldSym.ReturnType != sym.ReturnType {
			changes = append(changes, "return type")
			impact.Severity = "high"
		}
		if oldSym.Exported != sym.Exported {
			changes = append(changes, "visibility")
			if !sym.Exported && oldSym.Exported {
				impact.Severity = "critical"
			}
		}
		impact.Description = fmt.Sprintf("Symbol '%s' modified: %s", sym.Name, strings.Join(changes, ", "))
	}

	// Increase severity based on number of references
	if len(refs) > 10 && impact.Severity == "medium" {
		impact.Severity = "high"
	} else if len(refs) > 50 {
		impact.Severity = "critical"
	}

	return impact
}

// calculateOverallSeverity calculates overall severity from individual impacts
func (a *ImpactAnalyzer) calculateOverallSeverity(impact *FileImpact) string {
	if len(impact.Impacts) == 0 {
		return "low"
	}

	severityOrder := map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}
	maxSeverity := "low"

	for _, imp := range impact.Impacts {
		if severityOrder[imp.Severity] > severityOrder[maxSeverity] {
			maxSeverity = imp.Severity
		}
	}

	return maxSeverity
}

// GetSymbolsInFile returns all symbols in a file
func (a *ImpactAnalyzer) GetSymbolsInFile(filename string) []Symbol {
	return a.symbolTable.ByFile[filename]
}

// GetSymbolReferences returns all references to a symbol
func (a *ImpactAnalyzer) GetSymbolReferences(symbolName string) []Reference {
	return a.symbolTable.References[symbolName]
}

// FindSymbol finds a symbol by name
func (a *ImpactAnalyzer) FindSymbol(name string) []Symbol {
	return a.symbolTable.Symbols[name]
}

// GetDependents returns files that depend on the given file
func (a *ImpactAnalyzer) GetDependents(filename string) []string {
	return a.dependents[filename]
}

// FormatImpactReport generates a human-readable impact report
func FormatImpactReport(impact *FileImpact) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Impact Analysis for %s\n\n", impact.FilePath))
	sb.WriteString(fmt.Sprintf("**Overall Severity:** %s\n", strings.ToUpper(impact.OverallSeverity)))
	sb.WriteString(fmt.Sprintf("**Changed Symbols:** %d\n", len(impact.ChangedSymbols)))
	sb.WriteString(fmt.Sprintf("**Total References:** %d\n", impact.TotalReferences))
	sb.WriteString(fmt.Sprintf("**Affected Files:** %d\n\n", len(impact.AffectedFiles)))

	if len(impact.AffectedFiles) > 0 {
		sb.WriteString("### Affected Files\n")
		for _, file := range impact.AffectedFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", file))
		}
		sb.WriteString("\n")
	}

	if len(impact.Impacts) > 0 {
		sb.WriteString("### Symbol Changes\n")
		for _, imp := range impact.Impacts {
			sb.WriteString(fmt.Sprintf("\n#### %s `%s`\n", imp.ChangedSymbol.Kind, imp.ChangedSymbol.Name))
			sb.WriteString(fmt.Sprintf("- **Severity:** %s\n", imp.Severity))
			sb.WriteString(fmt.Sprintf("- **Description:** %s\n", imp.Description))
			sb.WriteString(fmt.Sprintf("- **References:** %d\n", len(imp.References)))

			if len(imp.References) > 0 && len(imp.References) <= 10 {
				sb.WriteString("- **Used in:**\n")
				for _, ref := range imp.References {
					sb.WriteString(fmt.Sprintf("  - %s:%d\n", ref.FilePath, ref.Line))
				}
			}
		}
	}

	return sb.String()
}

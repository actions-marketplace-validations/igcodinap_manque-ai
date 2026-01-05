package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaxPracticesSize is the maximum size in bytes for combined practices content.
// This prevents overwhelming the LLM context window.
const MaxPracticesSize = 15000

// RepoPractices holds discovered repository practices and guidelines
type RepoPractices struct {
	// Sources maps source file paths to their content
	Sources map[string]string
	// Combined is the formatted content ready to inject into prompts
	Combined string
}

// discoveryPattern defines a file or directory pattern to search for
type discoveryPattern struct {
	// Name is a human-readable name for this pattern
	Name string
	// Paths are the file/directory paths to check (relative to repo root)
	Paths []string
	// IsDir indicates if this pattern represents a directory to scan recursively
	IsDir bool
	// Extensions filters files by extension when scanning directories (empty = all)
	Extensions []string
}

// patterns defines all the locations where repo practices might be found
var patterns = []discoveryPattern{
	{
		Name:  "Cursor Rules",
		Paths: []string{".cursor/rules", ".cursorrules", ".cursor-rules"},
		IsDir: true,
		Extensions: []string{".md", ".mdc", ".txt"},
	},
	{
		Name:  "Claude Rules",
		Paths: []string{".claude", "CLAUDE.md", "claude.md"},
		IsDir: true,
		Extensions: []string{".md", ".mdc", ".txt"},
	},
	{
		Name:  "Agent Instructions",
		Paths: []string{".agent", ".agents", "agents.md", "AGENTS.md", ".agent/workflows"},
		IsDir: true,
		Extensions: []string{".md", ".mdc", ".txt", ".yaml", ".yml"},
	},
	{
		Name:  "GitHub Copilot",
		Paths: []string{".github/copilot-instructions.md"},
		IsDir: false,
	},
	{
		Name:  "Contributing Guidelines",
		Paths: []string{"CONTRIBUTING.md", "CONTRIBUTING.rst", "CONTRIBUTING.txt"},
		IsDir: false,
	},
	{
		Name:  "Code of Conduct",
		Paths: []string{"CODE_OF_CONDUCT.md"},
		IsDir: false,
	},
}

// Discover scans a repository for practices and guidelines files.
// It returns a RepoPractices struct with all discovered content.
func Discover(repoPath string) (*RepoPractices, error) {
	practices := &RepoPractices{
		Sources: make(map[string]string),
	}

	for _, pattern := range patterns {
		for _, path := range pattern.Paths {
			fullPath := filepath.Join(repoPath, path)
			
			info, err := os.Stat(fullPath)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				// Skip files we can't access
				continue
			}

			if info.IsDir() && pattern.IsDir {
				// Scan directory for matching files
				if err := scanDirectory(practices, fullPath, pattern.Name, pattern.Extensions); err != nil {
					// Log but don't fail on individual directory errors
					continue
				}
			} else if !info.IsDir() {
				// Read single file
				content, err := readFileWithLimit(fullPath)
				if err != nil {
					continue
				}
				if content != "" {
					relPath, _ := filepath.Rel(repoPath, fullPath)
					practices.Sources[relPath] = content
				}
			}
		}
	}

	// Build combined output
	practices.Combined = buildCombinedOutput(practices.Sources)

	return practices, nil
}

// scanDirectory recursively scans a directory for files matching the given extensions
func scanDirectory(practices *RepoPractices, dirPath, patternName string, extensions []string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		if info.IsDir() {
			return nil
		}

		// Check extension filter
		if len(extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			matched := false
			for _, allowed := range extensions {
				if ext == allowed {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		content, err := readFileWithLimit(path)
		if err != nil {
			return nil // Skip unreadable files
		}
		if content != "" {
			practices.Sources[path] = content
		}
		return nil
	})
}

// readFileWithLimit reads a file up to a reasonable size limit
func readFileWithLimit(path string) (string, error) {
	const maxFileSize = 8000 // Max bytes per file

	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	// Skip very large files
	if info.Size() > maxFileSize*2 {
		return "", nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Truncate if needed
	text := string(content)
	if len(text) > maxFileSize {
		text = text[:maxFileSize] + "\n... (truncated)"
	}

	return strings.TrimSpace(text), nil
}

// buildCombinedOutput creates the formatted output for injection into prompts
func buildCombinedOutput(sources map[string]string) string {
	if len(sources) == 0 {
		return ""
	}

	var builder strings.Builder
	totalSize := 0

	// Sort sources for consistent output
	sortedPaths := make([]string, 0, len(sources))
	for path := range sources {
		sortedPaths = append(sortedPaths, path)
	}
	// Simple sort for reproducibility
	for i := 0; i < len(sortedPaths); i++ {
		for j := i + 1; j < len(sortedPaths); j++ {
			if sortedPaths[i] > sortedPaths[j] {
				sortedPaths[i], sortedPaths[j] = sortedPaths[j], sortedPaths[i]
			}
		}
	}

	for _, path := range sortedPaths {
		content := sources[path]
		section := fmt.Sprintf("### %s\n\n%s\n\n", path, content)
		
		// Check size limit
		if totalSize+len(section) > MaxPracticesSize {
			builder.WriteString("\n... (additional files omitted due to size limit)\n")
			break
		}
		
		builder.WriteString(section)
		totalSize += len(section)
	}

	return strings.TrimSpace(builder.String())
}

// HasPractices returns true if any practices were discovered
func (p *RepoPractices) HasPractices() bool {
	return len(p.Sources) > 0
}

// Summary returns a brief summary of discovered practices
func (p *RepoPractices) Summary() string {
	if !p.HasPractices() {
		return "No repository practices discovered"
	}
	
	files := make([]string, 0, len(p.Sources))
	for path := range p.Sources {
		files = append(files, filepath.Base(path))
	}
	
	return fmt.Sprintf("Discovered %d practice file(s): %s", len(files), strings.Join(files, ", "))
}

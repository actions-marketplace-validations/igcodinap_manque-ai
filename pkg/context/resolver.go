package context

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ImportInfo represents an import statement found in a file
type ImportInfo struct {
	Source       string // The file containing the import
	ImportPath   string // The imported path
	ResolvedPath string // The resolved local file path
	Language     string // The detected language
}

// Resolver resolves imports from code files to their local paths
type Resolver struct {
	RootDir string
}

// NewResolver creates a new import resolver
func NewResolver(rootDir string) *Resolver {
	return &Resolver{RootDir: rootDir}
}

// Regular expressions for different import patterns
var (
	// Go: import "path" or import ( "path" )
	goImportRegex  = regexp.MustCompile(`(?:import\s+(?:"([^"]+)"|\(\s*(?:[^)]*?"([^"]+)"[^)]*?)*\s*\)))`)
	goSingleImport = regexp.MustCompile(`"([^"]+)"`)

	// JavaScript/TypeScript: import ... from 'path' or require('path')
	jsImportRegex = regexp.MustCompile(`(?:import\s+.*?\s+from\s+['"]([^'"]+)['"]|require\s*\(\s*['"]([^'"]+)['"]\s*\))`)

	// Python: import x or from x import y
	pyImportRegex = regexp.MustCompile(`(?:from\s+(\S+)\s+import|import\s+(\S+))`)

	// Rust: use path::to::module or mod module
	rustImportRegex = regexp.MustCompile(`(?:use\s+([\w:]+)|mod\s+(\w+))`)
)

// ExtractImports extracts imports from file content based on language
func (r *Resolver) ExtractImports(filename, content string) []ImportInfo {
	var imports []ImportInfo
	lang := detectLanguage(filename)

	switch lang {
	case "go":
		imports = r.extractGoImports(filename, content)
	case "javascript", "typescript":
		imports = r.extractJSImports(filename, content)
	case "python":
		imports = r.extractPythonImports(filename, content)
	case "rust":
		imports = r.extractRustImports(filename, content)
	}

	return imports
}

// extractGoImports extracts imports from Go code
func (r *Resolver) extractGoImports(filename, content string) []ImportInfo {
	var imports []ImportInfo

	// Find all quoted strings in import statements
	scanner := bufio.NewScanner(strings.NewReader(content))
	inImportBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		if inImportBlock || strings.HasPrefix(line, "import ") {
			matches := goSingleImport.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 1 && match[1] != "" {
					importPath := match[1]
					// Only resolve local imports (relative or same module)
					if isLocalImport(importPath) {
						resolved := r.resolveGoImport(filename, importPath)
						if resolved != "" {
							imports = append(imports, ImportInfo{
								Source:       filename,
								ImportPath:   importPath,
								ResolvedPath: resolved,
								Language:     "go",
							})
						}
					}
				}
			}
		}
	}

	return imports
}

// extractJSImports extracts imports from JavaScript/TypeScript code
func (r *Resolver) extractJSImports(filename, content string) []ImportInfo {
	var imports []ImportInfo

	matches := jsImportRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		importPath := ""
		if len(match) > 1 && match[1] != "" {
			importPath = match[1]
		} else if len(match) > 2 && match[2] != "" {
			importPath = match[2]
		}

		if importPath != "" && isRelativeImport(importPath) {
			resolved := r.resolveJSImport(filename, importPath)
			if resolved != "" {
				imports = append(imports, ImportInfo{
					Source:       filename,
					ImportPath:   importPath,
					ResolvedPath: resolved,
					Language:     detectLanguage(filename),
				})
			}
		}
	}

	return imports
}

// extractPythonImports extracts imports from Python code
func (r *Resolver) extractPythonImports(filename, content string) []ImportInfo {
	var imports []ImportInfo

	matches := pyImportRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		importPath := ""
		if len(match) > 1 && match[1] != "" {
			importPath = match[1]
		} else if len(match) > 2 && match[2] != "" {
			importPath = match[2]
		}

		if importPath != "" && !strings.Contains(importPath, ".") {
			// Likely a local module
			resolved := r.resolvePythonImport(filename, importPath)
			if resolved != "" {
				imports = append(imports, ImportInfo{
					Source:       filename,
					ImportPath:   importPath,
					ResolvedPath: resolved,
					Language:     "python",
				})
			}
		}
	}

	return imports
}

// extractRustImports extracts imports from Rust code
func (r *Resolver) extractRustImports(filename, content string) []ImportInfo {
	var imports []ImportInfo

	matches := rustImportRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		moduleName := ""
		if len(match) > 1 && match[1] != "" {
			// use statement - get first component
			parts := strings.Split(match[1], "::")
			if len(parts) > 0 && parts[0] == "crate" && len(parts) > 1 {
				moduleName = parts[1]
			}
		} else if len(match) > 2 && match[2] != "" {
			moduleName = match[2]
		}

		if moduleName != "" {
			resolved := r.resolveRustImport(filename, moduleName)
			if resolved != "" {
				imports = append(imports, ImportInfo{
					Source:       filename,
					ImportPath:   moduleName,
					ResolvedPath: resolved,
					Language:     "rust",
				})
			}
		}
	}

	return imports
}

// resolveGoImport resolves a Go import path to a local file
func (r *Resolver) resolveGoImport(sourceFile, importPath string) string {
	// For Go, we need to check if the import is from the same module
	// This is a simplified resolver - full resolution would require go.mod parsing

	// Check if it's a relative-looking path within the module
	parts := strings.Split(importPath, "/")
	if len(parts) > 0 {
		// Try to find it relative to root
		possiblePath := filepath.Join(r.RootDir, strings.Join(parts[len(parts)-2:], "/"))
		if info, err := os.Stat(possiblePath); err == nil && info.IsDir() {
			// Look for .go files
			files, _ := filepath.Glob(filepath.Join(possiblePath, "*.go"))
			if len(files) > 0 {
				return files[0] // Return first .go file
			}
		}
	}

	return ""
}

// resolveJSImport resolves a JS/TS import path to a local file
func (r *Resolver) resolveJSImport(sourceFile, importPath string) string {
	sourceDir := filepath.Dir(filepath.Join(r.RootDir, sourceFile))

	// Try common extensions
	extensions := []string{"", ".ts", ".tsx", ".js", ".jsx", "/index.ts", "/index.tsx", "/index.js", "/index.jsx"}

	for _, ext := range extensions {
		fullPath := filepath.Join(sourceDir, importPath+ext)
		if _, err := os.Stat(fullPath); err == nil {
			relPath, _ := filepath.Rel(r.RootDir, fullPath)
			return relPath
		}
	}

	return ""
}

// resolvePythonImport resolves a Python import to a local file
func (r *Resolver) resolvePythonImport(sourceFile, importPath string) string {
	sourceDir := filepath.Dir(filepath.Join(r.RootDir, sourceFile))

	// Convert dots to path separators
	modulePath := strings.ReplaceAll(importPath, ".", "/")

	// Try as file or directory with __init__.py
	possiblePaths := []string{
		filepath.Join(sourceDir, modulePath+".py"),
		filepath.Join(sourceDir, modulePath, "__init__.py"),
		filepath.Join(r.RootDir, modulePath+".py"),
		filepath.Join(r.RootDir, modulePath, "__init__.py"),
	}

	for _, fullPath := range possiblePaths {
		if _, err := os.Stat(fullPath); err == nil {
			relPath, _ := filepath.Rel(r.RootDir, fullPath)
			return relPath
		}
	}

	return ""
}

// resolveRustImport resolves a Rust module to a local file
func (r *Resolver) resolveRustImport(sourceFile, moduleName string) string {
	sourceDir := filepath.Dir(filepath.Join(r.RootDir, sourceFile))

	// Rust modules can be module.rs or module/mod.rs
	possiblePaths := []string{
		filepath.Join(sourceDir, moduleName+".rs"),
		filepath.Join(sourceDir, moduleName, "mod.rs"),
	}

	for _, fullPath := range possiblePaths {
		if _, err := os.Stat(fullPath); err == nil {
			relPath, _ := filepath.Rel(r.RootDir, fullPath)
			return relPath
		}
	}

	return ""
}

// detectLanguage detects the programming language from filename
func detectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return "unknown"
	}
}

// isLocalImport checks if an import path looks like a local/relative import
func isLocalImport(importPath string) bool {
	// Standard library and third-party packages typically start with common prefixes
	// Local packages in Go usually contain the module name
	return !strings.HasPrefix(importPath, "github.com/") &&
		!strings.HasPrefix(importPath, "golang.org/") &&
		!strings.HasPrefix(importPath, "google.golang.org/") &&
		!strings.HasPrefix(importPath, "gopkg.in/") &&
		strings.Contains(importPath, "/") &&
		!isStandardLibrary(importPath)
}

// isRelativeImport checks if an import is relative (starts with . or ..)
func isRelativeImport(importPath string) bool {
	return strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../")
}

// isStandardLibrary checks if a Go import is from the standard library
func isStandardLibrary(importPath string) bool {
	stdPackages := map[string]bool{
		"fmt": true, "os": true, "io": true, "net": true, "http": true,
		"context": true, "time": true, "strings": true, "strconv": true,
		"encoding": true, "json": true, "sync": true, "errors": true,
		"path": true, "filepath": true, "regexp": true, "bufio": true,
		"bytes": true, "sort": true, "log": true, "flag": true,
	}

	parts := strings.Split(importPath, "/")
	if len(parts) > 0 {
		return stdPackages[parts[0]]
	}
	return false
}

package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/manque-ai/pkg/diff"
)

const (
	// MaxFileSize is the maximum size of a referenced file to include
	MaxFileSize = 10000
	// MaxTotalContextSize is the maximum total context from referenced files
	MaxTotalContextSize = 50000
	// MaxFilesToFetch is the maximum number of referenced files to fetch
	MaxFilesToFetch = 10
)

// FetchedFile represents a file that was fetched for context
type FetchedFile struct {
	Path     string
	Content  string
	Language string
	Size     int
}

// Fetcher fetches referenced files for context expansion
type Fetcher struct {
	RootDir  string
	Resolver *Resolver
}

// NewFetcher creates a new context fetcher
func NewFetcher(rootDir string) *Fetcher {
	return &Fetcher{
		RootDir:  rootDir,
		Resolver: NewResolver(rootDir),
	}
}

// FetchReferencedFiles extracts imports from changed files and fetches their content
func (f *Fetcher) FetchReferencedFiles(files []diff.FileDiff) []FetchedFile {
	var fetched []FetchedFile
	seen := make(map[string]bool)
	totalSize := 0

	for _, file := range files {
		if len(fetched) >= MaxFilesToFetch {
			break
		}

		// Get the new content from the diff
		content := f.extractNewContent(file)
		if content == "" {
			continue
		}

		// Extract imports from the changed file
		imports := f.Resolver.ExtractImports(file.Filename, content)

		for _, imp := range imports {
			if seen[imp.ResolvedPath] || imp.ResolvedPath == "" {
				continue
			}
			if len(fetched) >= MaxFilesToFetch {
				break
			}

			// Fetch the file content
			fetchedFile, err := f.fetchFile(imp.ResolvedPath, imp.Language)
			if err != nil {
				continue
			}

			// Check size limits
			if totalSize+fetchedFile.Size > MaxTotalContextSize {
				continue
			}

			seen[imp.ResolvedPath] = true
			fetched = append(fetched, *fetchedFile)
			totalSize += fetchedFile.Size
		}
	}

	return fetched
}

// extractNewContent extracts the new (added/unchanged) content from a file diff
func (f *Fetcher) extractNewContent(file diff.FileDiff) string {
	var builder strings.Builder

	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == diff.LineAdded || line.Type == diff.LineContext {
				builder.WriteString(line.Content)
				builder.WriteString("\n")
			}
		}
	}

	return builder.String()
}

// fetchFile reads a file and returns its content
func (f *Fetcher) fetchFile(relPath, language string) (*FetchedFile, error) {
	fullPath := filepath.Join(f.RootDir, relPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes", info.Size())
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	return &FetchedFile{
		Path:     relPath,
		Content:  string(content),
		Language: language,
		Size:     len(content),
	}, nil
}

// FormatForLLM formats fetched files for inclusion in the LLM prompt
func FormatForLLM(files []FetchedFile) string {
	if len(files) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("\n## Referenced Files (for context)\n\n")
	builder.WriteString("The following files are imported/referenced by the changed files:\n\n")

	for _, file := range files {
		builder.WriteString(fmt.Sprintf("### %s\n", file.Path))
		builder.WriteString(fmt.Sprintf("```%s\n", file.Language))
		builder.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString("```\n\n")
	}

	return builder.String()
}

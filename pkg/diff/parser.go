package diff

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type FileDiff struct {
	Filename    string
	OldContent  string
	NewContent  string
	Hunks       []Hunk
}

type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []Line
}

type Line struct {
	Type    LineType
	Content string
	OldNum  int
	NewNum  int
}

type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineRemoved
)

var (
	fileHeaderRegex = regexp.MustCompile(`^diff --git a/(.*) b/(.*)$`)
	hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$`)
)

func ParseGitDiff(diffText string) ([]FileDiff, error) {
	scanner := bufio.NewScanner(strings.NewReader(diffText))
	var files []FileDiff
	var currentFile *FileDiff
	var currentHunk *Hunk
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Check for new file
		if match := fileHeaderRegex.FindStringSubmatch(line); match != nil {
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
				}
				files = append(files, *currentFile)
			}
			
			currentFile = &FileDiff{
				Filename: match[1], // Use the 'a/' path
				Hunks:    []Hunk{},
			}
			currentHunk = nil
			continue
		}
		
		// Check for hunk header
		if match := hunkHeaderRegex.FindStringSubmatch(line); match != nil {
			if currentHunk != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}
			
			oldStart, _ := strconv.Atoi(match[1])
			oldCount := 1
			if match[2] != "" {
				oldCount, _ = strconv.Atoi(match[2])
			}
			newStart, _ := strconv.Atoi(match[3])
			newCount := 1
			if match[4] != "" {
				newCount, _ = strconv.Atoi(match[4])
			}
			
			currentHunk = &Hunk{
				OldStart: oldStart,
				OldCount: oldCount,
				NewStart: newStart,
				NewCount: newCount,
				Lines:    []Line{},
			}
			continue
		}
		
		// Skip non-diff lines (file metadata, etc.)
		if currentHunk == nil {
			continue
		}
		
		// Parse diff lines
		if len(line) > 0 {
			switch line[0] {
			case '+':
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    LineAdded,
					Content: line[1:],
				})
			case '-':
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    LineRemoved,
					Content: line[1:],
				})
			case ' ':
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    LineContext,
					Content: line[1:],
				})
			}
		}
	}
	
	// Add the last file and hunk
	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		files = append(files, *currentFile)
	}
	
	// Calculate line numbers for each hunk
	for i := range files {
		for j := range files[i].Hunks {
			calculateLineNumbers(&files[i].Hunks[j])
		}
	}
	
	return files, scanner.Err()
}

func calculateLineNumbers(hunk *Hunk) {
	oldLineNum := hunk.OldStart
	newLineNum := hunk.NewStart
	
	for i := range hunk.Lines {
		line := &hunk.Lines[i]
		
		switch line.Type {
		case LineContext:
			line.OldNum = oldLineNum
			line.NewNum = newLineNum
			oldLineNum++
			newLineNum++
		case LineAdded:
			line.NewNum = newLineNum
			newLineNum++
		case LineRemoved:
			line.OldNum = oldLineNum
			oldLineNum++
		}
	}
}

// FormatForLLM formats the diff in the specific format expected by the LLM
func FormatForLLM(files []FileDiff) string {
	var result strings.Builder
	
	for _, file := range files {
		result.WriteString(fmt.Sprintf("## File: '%s'\n", file.Filename))
		
		for _, hunk := range file.Hunks {
			// Write hunk header
			result.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", 
				hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount))
			
			// Generate new hunk section
			result.WriteString("__new hunk__\n")
			newLineNum := hunk.NewStart
			for _, line := range hunk.Lines {
				if line.Type == LineAdded {
					result.WriteString(fmt.Sprintf("%d +%s\n", newLineNum, line.Content))
					newLineNum++
				} else if line.Type == LineContext {
					result.WriteString(fmt.Sprintf("%d  %s\n", newLineNum, line.Content))
					newLineNum++
				}
			}
			
			// Generate old hunk section
			result.WriteString("__old hunk__\n")
			for _, line := range hunk.Lines {
				if line.Type == LineRemoved {
					result.WriteString(fmt.Sprintf("-%s\n", line.Content))
				} else if line.Type == LineContext {
					result.WriteString(fmt.Sprintf(" %s\n", line.Content))
				}
			}
		}
		result.WriteString("\n")
	}
	
	return result.String()
}
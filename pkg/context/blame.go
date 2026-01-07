package context

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BlameInfo contains git blame information for a file
type BlameInfo struct {
	Filename     string
	LastModified time.Time
	Authors      []string
	AgeInDays    int
	IsStable     bool // True if not modified recently
}

// BlameLineInfo contains blame info for a specific line
type BlameLineInfo struct {
	Author    string
	Email     string
	Timestamp time.Time
	Commit    string
}

const (
	// StabilityThresholdDays - files not modified in this many days are considered stable
	StabilityThresholdDays = 30
)

var (
	blameLineRegex = regexp.MustCompile(`^([a-f0-9]+)\s+\((.+?)\s+(\d{4}-\d{2}-\d{2})\s+\d{2}:\d{2}:\d{2}\s+[+-]\d{4}\s+(\d+)\)`)
)

// GetBlameInfo runs git blame and extracts information about a file
func GetBlameInfo(filename string, startLine, endLine int) (*BlameInfo, error) {
	// Run git blame for the specific line range
	args := []string{"blame", "-l", "--date=iso"}
	if startLine > 0 && endLine > 0 {
		args = append(args, fmt.Sprintf("-L%d,%d", startLine, endLine))
	}
	args = append(args, "--", filename)

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame failed: %w", err)
	}

	return parseBlameOutput(filename, string(output))
}

// parseBlameOutput parses the git blame output
func parseBlameOutput(filename, output string) (*BlameInfo, error) {
	lines := strings.Split(output, "\n")
	authorsMap := make(map[string]bool)
	var latestTime time.Time

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse blame line
		matches := blameLineRegex.FindStringSubmatch(line)
		if len(matches) < 4 {
			continue
		}

		author := strings.TrimSpace(matches[2])
		dateStr := matches[3]

		authorsMap[author] = true

		// Parse date
		t, err := time.Parse("2006-01-02", dateStr)
		if err == nil && t.After(latestTime) {
			latestTime = t
		}
	}

	// Convert authors map to slice
	var authors []string
	for author := range authorsMap {
		authors = append(authors, author)
	}

	ageInDays := int(time.Since(latestTime).Hours() / 24)

	return &BlameInfo{
		Filename:     filename,
		LastModified: latestTime,
		Authors:      authors,
		AgeInDays:    ageInDays,
		IsStable:     ageInDays > StabilityThresholdDays,
	}, nil
}

// GetFileBlameContext gets blame context for a file diff
func GetFileBlameContext(filename string, changedLines []int) string {
	if len(changedLines) == 0 {
		return ""
	}

	// Get blame for the first and last changed line
	startLine := changedLines[0]
	endLine := changedLines[len(changedLines)-1]

	info, err := GetBlameInfo(filename, startLine, endLine)
	if err != nil {
		return ""
	}

	var context strings.Builder
	context.WriteString(fmt.Sprintf("Code age: %d days", info.AgeInDays))

	if info.IsStable {
		context.WriteString(" (stable)")
	} else {
		context.WriteString(" (recently modified)")
	}

	if len(info.Authors) > 0 {
		context.WriteString(fmt.Sprintf(", Authors: %s", strings.Join(info.Authors, ", ")))
	}

	return context.String()
}

// FormatBlameContext formats blame information for inclusion in the prompt
func FormatBlameContext(files map[string]string) string {
	if len(files) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("\n## Code History Context\n\n")

	for filename, context := range files {
		if context != "" {
			builder.WriteString(fmt.Sprintf("- `%s`: %s\n", filename, context))
		}
	}

	return builder.String()
}

// GetChangedLineNumbers extracts line numbers that were modified from a hunk
func GetChangedLineNumbers(lines []struct {
	Type   int // 0=context, 1=added, 2=removed
	NewNum int
}) []int {
	var changed []int
	for _, line := range lines {
		if line.Type == 1 { // Added line
			changed = append(changed, line.NewNum)
		}
	}
	return changed
}

// GetLogForFile gets recent commit history for a file
func GetLogForFile(filename string, limit int) ([]string, error) {
	args := []string{"log", "--oneline", fmt.Sprintf("-n%d", limit), "--", filename}
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines, nil
}

// GetFileStability determines if a file is stable (not frequently changed)
func GetFileStability(filename string) (stable bool, commitCount int, err error) {
	// Count commits in the last 30 days
	since := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	args := []string{"log", "--oneline", fmt.Sprintf("--since=%s", since), "--", filename}
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return false, 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return true, 0, nil
	}

	commitCount = len(lines)
	// Consider stable if less than 5 commits in the last 30 days
	return commitCount < 5, commitCount, nil
}

// intSliceToInterface converts []int to []interface{} for usage
func parseLineNumber(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

package commands

import (
	"regexp"
	"strings"
)

// CommandType represents the type of command
type CommandType string

const (
	CommandExplain    CommandType = "explain"
	CommandSuggestFix CommandType = "suggest_fix"
	CommandIgnore     CommandType = "ignore"
	CommandRegenerate CommandType = "regenerate"
	CommandHelp       CommandType = "help"
	CommandSummarize  CommandType = "summarize"
	CommandUnknown    CommandType = "unknown"
)

// Command represents a parsed command from a comment
type Command struct {
	Type      CommandType
	Args      string
	RawText   string
	CommentID int64
	File      string // Optional: file context from review comment
	Line      int    // Optional: line context from review comment
}

// Parser parses commands from PR comments
type Parser struct {
	botName string
	aliases []string
}

// NewParser creates a new command parser
func NewParser(botName string) *Parser {
	// Support common variations
	aliases := []string{
		"@" + botName,
		"@" + strings.ToLower(botName),
		"@manque",
		"@manque-ai",
	}
	return &Parser{
		botName: botName,
		aliases: aliases,
	}
}

// Parse extracts commands from a comment body
func (p *Parser) Parse(body string, commentID int64, file string, lineNum int) []Command {
	var cmds []Command

	// Normalize the body
	body = strings.TrimSpace(body)
	lines := strings.Split(body, "\n")

	for _, rawLine := range lines {
		trimmedLine := strings.TrimSpace(rawLine)
		if trimmedLine == "" {
			continue
		}

		// Check if line starts with any of our aliases
		for _, alias := range p.aliases {
			if strings.HasPrefix(strings.ToLower(trimmedLine), strings.ToLower(alias)) {
				// Extract the command part after the alias
				cmdPart := strings.TrimSpace(trimmedLine[len(alias):])
				cmd := p.parseCommand(cmdPart, commentID, file, lineNum)
				cmd.RawText = rawLine
				cmds = append(cmds, cmd)
				break
			}
		}
	}

	return cmds
}

// parseCommand parses the command type and arguments
func (p *Parser) parseCommand(text string, commentID int64, file string, line int) Command {
	text = strings.TrimSpace(text)
	parts := strings.SplitN(text, " ", 2)

	cmdWord := ""
	args := ""
	if len(parts) > 0 {
		cmdWord = strings.ToLower(parts[0])
	}
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	cmd := Command{
		CommentID: commentID,
		File:      file,
		Line:      line,
		Args:      args,
	}

	switch cmdWord {
	case "explain", "what", "why":
		cmd.Type = CommandExplain
	case "suggest", "fix", "suggest_fix", "suggest-fix":
		cmd.Type = CommandSuggestFix
	case "ignore", "dismiss", "skip":
		cmd.Type = CommandIgnore
	case "regenerate", "rereview", "review", "re-review":
		cmd.Type = CommandRegenerate
	case "help", "?":
		cmd.Type = CommandHelp
	case "summarize", "summary", "tldr":
		cmd.Type = CommandSummarize
	default:
		// Try to infer from full text
		cmd.Type = p.inferCommandType(text)
	}

	return cmd
}

// inferCommandType tries to infer command type from natural language
func (p *Parser) inferCommandType(text string) CommandType {
	text = strings.ToLower(text)

	// Question patterns
	questionPatterns := []string{
		"what does", "what is", "why does", "why is",
		"how does", "how is", "can you explain",
		"explain", "tell me", "describe",
	}
	for _, pattern := range questionPatterns {
		if strings.Contains(text, pattern) {
			return CommandExplain
		}
	}

	// Fix patterns
	fixPatterns := []string{
		"fix", "suggest", "how to fix", "how can i fix",
		"what should i", "recommend",
	}
	for _, pattern := range fixPatterns {
		if strings.Contains(text, pattern) {
			return CommandSuggestFix
		}
	}

	// Ignore patterns
	ignorePatterns := []string{
		"ignore", "dismiss", "false positive", "not an issue",
		"skip this", "don't flag", "not a bug",
	}
	for _, pattern := range ignorePatterns {
		if strings.Contains(text, pattern) {
			return CommandIgnore
		}
	}

	// Summary patterns
	summaryPatterns := []string{
		"summarize", "summary", "tldr", "overview",
	}
	for _, pattern := range summaryPatterns {
		if strings.Contains(text, pattern) {
			return CommandSummarize
		}
	}

	return CommandUnknown
}

// ParseMention extracts mentions from a comment body
func ParseMention(body string) []string {
	re := regexp.MustCompile(`@([a-zA-Z0-9_-]+)`)
	matches := re.FindAllStringSubmatch(body, -1)

	var mentions []string
	for _, match := range matches {
		if len(match) > 1 {
			mentions = append(mentions, match[1])
		}
	}
	return mentions
}

// IsBotMentioned checks if the bot is mentioned in the comment
func (p *Parser) IsBotMentioned(body string) bool {
	bodyLower := strings.ToLower(body)
	for _, alias := range p.aliases {
		if strings.Contains(bodyLower, strings.ToLower(alias)) {
			return true
		}
	}
	return false
}

// GetHelpText returns the help message for the bot
func GetHelpText() string {
	return `## Manque AI Commands

You can interact with me using the following commands:

| Command | Description |
|---------|-------------|
| ` + "`@manque explain`" + ` | Explain the code or issue in detail |
| ` + "`@manque suggest fix`" + ` | Get a suggested fix for this issue |
| ` + "`@manque ignore`" + ` | Dismiss this issue (won't be flagged again) |
| ` + "`@manque regenerate`" + ` | Re-run the review for this PR |
| ` + "`@manque summarize`" + ` | Get a summary of the changes |
| ` + "`@manque help`" + ` | Show this help message |

You can also ask questions naturally, like:
- "@manque what does this function do?"
- "@manque why is this a security issue?"
- "@manque how can I fix this?"
`
}

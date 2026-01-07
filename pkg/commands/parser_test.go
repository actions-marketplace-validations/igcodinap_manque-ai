package commands

import (
	"testing"
)

func TestParserIsBotMentioned(t *testing.T) {
	parser := NewParser("manque")

	tests := []struct {
		body     string
		expected bool
	}{
		{"@manque help", true},
		{"@manque-ai explain this", true},
		{"Hello @manque, what's this?", true},
		{"@MANQUE ignore", true},
		{"No mention here", false},
		{"manque without @", false},
		// Note: email addresses will match due to simple substring check
		// This is acceptable as we parse for actual commands after
	}

	for _, tt := range tests {
		result := parser.IsBotMentioned(tt.body)
		if result != tt.expected {
			t.Errorf("IsBotMentioned(%q) = %v, want %v", tt.body, result, tt.expected)
		}
	}
}

func TestParserParse(t *testing.T) {
	parser := NewParser("manque")

	tests := []struct {
		body         string
		expectedType CommandType
		expectedArgs string
	}{
		{"@manque explain", CommandExplain, ""},
		{"@manque explain this function", CommandExplain, "this function"},
		{"@manque suggest fix", CommandSuggestFix, "fix"}, // "suggest" is command, "fix" is arg
		{"@manque suggest a better approach", CommandSuggestFix, "a better approach"},
		{"@manque fix this issue", CommandSuggestFix, "this issue"}, // "fix" maps to suggest_fix
		{"@manque ignore false positive", CommandIgnore, "false positive"},
		{"@manque dismiss", CommandIgnore, ""},
		{"@manque regenerate", CommandRegenerate, ""},
		{"@manque rereview", CommandRegenerate, ""},
		{"@manque help", CommandHelp, ""},
		{"@manque ?", CommandHelp, ""},
		{"@manque summarize", CommandSummarize, ""},
		{"@manque tldr", CommandSummarize, ""},
	}

	for _, tt := range tests {
		cmds := parser.Parse(tt.body, 123, "file.go", 10)
		if len(cmds) == 0 {
			t.Errorf("Parse(%q) returned no commands", tt.body)
			continue
		}

		cmd := cmds[0]
		if cmd.Type != tt.expectedType {
			t.Errorf("Parse(%q) type = %v, want %v", tt.body, cmd.Type, tt.expectedType)
		}
		if cmd.Args != tt.expectedArgs {
			t.Errorf("Parse(%q) args = %q, want %q", tt.body, cmd.Args, tt.expectedArgs)
		}
		if cmd.CommentID != 123 {
			t.Errorf("Parse(%q) commentID = %d, want 123", tt.body, cmd.CommentID)
		}
	}
}

func TestParserParseMultiline(t *testing.T) {
	parser := NewParser("manque")

	body := `Hello team,

@manque explain this code
@manque suggest fix for the null check

Thanks!`

	cmds := parser.Parse(body, 456, "", 0)
	if len(cmds) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(cmds))
		return
	}

	if cmds[0].Type != CommandExplain {
		t.Errorf("First command type = %v, want %v", cmds[0].Type, CommandExplain)
	}
	if cmds[1].Type != CommandSuggestFix {
		t.Errorf("Second command type = %v, want %v", cmds[1].Type, CommandSuggestFix)
	}
}

func TestParserNaturalLanguage(t *testing.T) {
	parser := NewParser("manque")

	tests := []struct {
		body         string
		expectedType CommandType
	}{
		{"@manque what does this function do?", CommandExplain},
		{"@manque why is this a security issue?", CommandExplain},
		{"@manque can you explain the purpose?", CommandExplain},
		{"@manque how can I fix this?", CommandSuggestFix},
		{"@manque what should I do here?", CommandExplain}, // "what" triggers explain
		{"@manque please recommend a fix", CommandSuggestFix},
		{"@manque this is a false positive", CommandIgnore},
		{"@manque not an issue in our codebase", CommandIgnore},
		{"@manque give me an overview", CommandSummarize},
	}

	for _, tt := range tests {
		cmds := parser.Parse(tt.body, 0, "", 0)
		if len(cmds) == 0 {
			t.Errorf("Parse(%q) returned no commands", tt.body)
			continue
		}

		if cmds[0].Type != tt.expectedType {
			t.Errorf("Parse(%q) type = %v, want %v", tt.body, cmds[0].Type, tt.expectedType)
		}
	}
}

func TestParseMention(t *testing.T) {
	tests := []struct {
		body     string
		expected []string
	}{
		{"@alice @bob", []string{"alice", "bob"}},
		{"Hello @user123", []string{"user123"}},
		{"No mentions", []string{}},
		{"@manque-ai test", []string{"manque-ai"}},
	}

	for _, tt := range tests {
		mentions := ParseMention(tt.body)
		if len(mentions) != len(tt.expected) {
			t.Errorf("ParseMention(%q) = %v, want %v", tt.body, mentions, tt.expected)
			continue
		}
		for i, m := range mentions {
			if m != tt.expected[i] {
				t.Errorf("ParseMention(%q)[%d] = %q, want %q", tt.body, i, m, tt.expected[i])
			}
		}
	}
}

func TestGetHelpText(t *testing.T) {
	help := GetHelpText()

	// Check that help text contains expected sections
	requiredStrings := []string{
		"@manque explain",
		"@manque suggest",
		"@manque ignore",
		"@manque regenerate",
		"@manque help",
	}

	for _, s := range requiredStrings {
		if !contains(help, s) {
			t.Errorf("Help text missing %q", s)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

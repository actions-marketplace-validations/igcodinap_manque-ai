package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/igcodinap/manque-ai/internal"
	"github.com/igcodinap/manque-ai/pkg/ai"
	"github.com/igcodinap/manque-ai/pkg/state"
)

// Handler handles bot commands
type Handler struct {
	AIClient ai.Client
	Config   *internal.Config
}

// NewHandler creates a new command handler
func NewHandler(aiClient ai.Client, config *internal.Config) *Handler {
	return &Handler{
		AIClient: aiClient,
		Config:   config,
	}
}

// ConversationMessage represents a message in a conversation thread
type ConversationMessage struct {
	Author string
	Body   string
	IsBot  bool
}

// CommandContext contains the context for executing a command
type CommandContext struct {
	Ctx                 context.Context
	PRTitle             string
	PRDescription       string
	PRNumber            int
	Repository          string
	CommentBody         string                // The original comment from user
	FilePath            string                // File path if this is a review comment
	FileLine            int                   // Line number if this is a review comment
	CodeContext         string                // Surrounding code context
	OriginalIssue       string                // The original bot comment that user is replying to
	Session             *state.Session
	ConversationHistory []ConversationMessage // Previous messages in this thread
}

// CommandResult contains the result of executing a command
type CommandResult struct {
	Response       string
	UpdateSession  bool
	DismissIssue   bool
	DismissedHash  string
	DismissReason  string
	TriggerReview  bool
}

// Handle executes a command and returns the response
func (h *Handler) Handle(cmd Command, ctx *CommandContext) (*CommandResult, error) {
	switch cmd.Type {
	case CommandExplain:
		return h.handleExplain(cmd, ctx)
	case CommandSuggestFix:
		return h.handleSuggestFix(cmd, ctx)
	case CommandIgnore:
		return h.handleIgnore(cmd, ctx)
	case CommandRegenerate:
		return h.handleRegenerate(cmd, ctx)
	case CommandHelp:
		return h.handleHelp(cmd, ctx)
	case CommandSummarize:
		return h.handleSummarize(cmd, ctx)
	case CommandUnknown:
		return h.handleUnknown(cmd, ctx)
	default:
		return h.handleUnknown(cmd, ctx)
	}
}

func (h *Handler) handleExplain(cmd Command, ctx *CommandContext) (*CommandResult, error) {
	prompt := h.buildExplainPrompt(cmd, ctx)

	response, err := h.AIClient.GenerateResponse(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate explanation: %w", err)
	}

	return &CommandResult{
		Response:      response,
		UpdateSession: true,
	}, nil
}

func (h *Handler) handleSuggestFix(cmd Command, ctx *CommandContext) (*CommandResult, error) {
	prompt := h.buildSuggestFixPrompt(cmd, ctx)

	response, err := h.AIClient.GenerateResponse(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate fix suggestion: %w", err)
	}

	return &CommandResult{
		Response:      response,
		UpdateSession: true,
	}, nil
}

func (h *Handler) handleIgnore(cmd Command, ctx *CommandContext) (*CommandResult, error) {
	// Calculate the hash of the issue being dismissed
	var hash string
	if ctx.FilePath != "" && ctx.FileLine > 0 && ctx.OriginalIssue != "" {
		hash = state.ComputeCommentHash(ctx.FilePath, ctx.FileLine, ctx.FileLine, ctx.OriginalIssue)
	}

	reason := cmd.Args
	if reason == "" {
		reason = "Dismissed by user"
	}

	response := fmt.Sprintf("Got it! I've dismissed this issue and won't flag it again in future reviews.\n\n**Reason:** %s", reason)

	return &CommandResult{
		Response:      response,
		UpdateSession: true,
		DismissIssue:  true,
		DismissedHash: hash,
		DismissReason: reason,
	}, nil
}

func (h *Handler) handleRegenerate(cmd Command, ctx *CommandContext) (*CommandResult, error) {
	response := "I'll re-run the review for this PR. This may take a moment..."

	return &CommandResult{
		Response:      response,
		TriggerReview: true,
		UpdateSession: true,
	}, nil
}

func (h *Handler) handleHelp(_ Command, _ *CommandContext) (*CommandResult, error) {
	return &CommandResult{
		Response: GetHelpText(),
	}, nil
}

func (h *Handler) handleSummarize(cmd Command, ctx *CommandContext) (*CommandResult, error) {
	prompt := h.buildSummarizePrompt(cmd, ctx)

	response, err := h.AIClient.GenerateResponse(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	return &CommandResult{
		Response:      response,
		UpdateSession: true,
	}, nil
}

func (h *Handler) handleUnknown(cmd Command, ctx *CommandContext) (*CommandResult, error) {
	// Try to be helpful with unknown commands
	prompt := h.buildConversationalPrompt(cmd, ctx)

	response, err := h.AIClient.GenerateResponse(prompt)
	if err != nil {
		// Fallback to help message
		return &CommandResult{
			Response: fmt.Sprintf("I'm not sure what you're asking. Here's what I can help with:\n\n%s", GetHelpText()),
		}, nil
	}

	return &CommandResult{
		Response:      response,
		UpdateSession: true,
	}, nil
}

// Prompt builders

// formatConversationHistory formats the conversation history for the LLM
func formatConversationHistory(history []ConversationMessage) string {
	if len(history) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n**Conversation History:**\n")
	sb.WriteString("(Previous messages in this thread, from oldest to newest)\n\n")

	for i, msg := range history {
		role := "User"
		if msg.IsBot {
			role = "Manque AI"
		} else {
			role = fmt.Sprintf("@%s", msg.Author)
		}
		sb.WriteString(fmt.Sprintf("%d. **%s:**\n%s\n\n", i+1, role, msg.Body))
	}

	return sb.String()
}

func (h *Handler) buildExplainPrompt(cmd Command, ctx *CommandContext) string {
	var sb strings.Builder

	sb.WriteString("You are a helpful code review assistant. A user is asking for an explanation.\n\n")

	if ctx.PRTitle != "" {
		sb.WriteString(fmt.Sprintf("**PR Title:** %s\n", ctx.PRTitle))
	}

	if ctx.FilePath != "" {
		sb.WriteString(fmt.Sprintf("**File:** %s\n", ctx.FilePath))
		if ctx.FileLine > 0 {
			sb.WriteString(fmt.Sprintf("**Line:** %d\n", ctx.FileLine))
		}
	}

	if ctx.OriginalIssue != "" {
		sb.WriteString(fmt.Sprintf("\n**Original Review Comment:**\n%s\n", ctx.OriginalIssue))
	}

	if ctx.CodeContext != "" {
		sb.WriteString(fmt.Sprintf("\n**Code Context:**\n```\n%s\n```\n", ctx.CodeContext))
	}

	// Include conversation history for context
	if len(ctx.ConversationHistory) > 0 {
		sb.WriteString(formatConversationHistory(ctx.ConversationHistory))
	}

	sb.WriteString(fmt.Sprintf("\n**User's Current Question:**\n%s\n", cmd.Args))
	if cmd.Args == "" {
		sb.WriteString(ctx.CommentBody)
	}

	sb.WriteString("\n\nProvide a clear, helpful explanation. Be concise but thorough.")
	if len(ctx.ConversationHistory) > 0 {
		sb.WriteString(" Take into account the conversation history when formulating your response.")
	}

	return sb.String()
}

func (h *Handler) buildSuggestFixPrompt(cmd Command, ctx *CommandContext) string {
	var sb strings.Builder

	sb.WriteString("You are a helpful code review assistant. A user is asking for a fix suggestion.\n\n")

	if ctx.FilePath != "" {
		sb.WriteString(fmt.Sprintf("**File:** %s\n", ctx.FilePath))
		if ctx.FileLine > 0 {
			sb.WriteString(fmt.Sprintf("**Line:** %d\n", ctx.FileLine))
		}
	}

	if ctx.OriginalIssue != "" {
		sb.WriteString(fmt.Sprintf("\n**Original Issue:**\n%s\n", ctx.OriginalIssue))
	}

	if ctx.CodeContext != "" {
		sb.WriteString(fmt.Sprintf("\n**Current Code:**\n```\n%s\n```\n", ctx.CodeContext))
	}

	// Include conversation history for context
	if len(ctx.ConversationHistory) > 0 {
		sb.WriteString(formatConversationHistory(ctx.ConversationHistory))
	}

	sb.WriteString("\nProvide a suggested fix. Use GitHub's suggestion format when possible:\n")
	sb.WriteString("```suggestion\n<your fixed code here>\n```\n")
	sb.WriteString("\nExplain why this fix addresses the issue.")
	if len(ctx.ConversationHistory) > 0 {
		sb.WriteString(" Consider any clarifications or preferences mentioned in the conversation history.")
	}

	return sb.String()
}

func (h *Handler) buildSummarizePrompt(cmd Command, ctx *CommandContext) string {
	var sb strings.Builder

	sb.WriteString("You are a helpful code review assistant. Provide a concise summary.\n\n")

	if ctx.PRTitle != "" {
		sb.WriteString(fmt.Sprintf("**PR Title:** %s\n", ctx.PRTitle))
	}

	if ctx.PRDescription != "" {
		sb.WriteString(fmt.Sprintf("**PR Description:**\n%s\n", ctx.PRDescription))
	}

	if ctx.Session != nil {
		sb.WriteString(fmt.Sprintf("\n**Review History:**\n%s\n", ctx.Session.GetSummary()))
	}

	sb.WriteString("\nProvide a brief summary of this PR and its review status.")

	return sb.String()
}

func (h *Handler) buildConversationalPrompt(cmd Command, ctx *CommandContext) string {
	var sb strings.Builder

	sb.WriteString("You are a helpful code review assistant named Manque AI. ")
	sb.WriteString("A user has asked you something in a PR comment. Respond helpfully.\n\n")

	if ctx.FilePath != "" {
		sb.WriteString(fmt.Sprintf("**Context File:** %s", ctx.FilePath))
		if ctx.FileLine > 0 {
			sb.WriteString(fmt.Sprintf(" (line %d)", ctx.FileLine))
		}
		sb.WriteString("\n")
	}

	if ctx.OriginalIssue != "" {
		sb.WriteString(fmt.Sprintf("\n**Previous Bot Comment:**\n%s\n", ctx.OriginalIssue))
	}

	if ctx.CodeContext != "" {
		sb.WriteString(fmt.Sprintf("\n**Code Context:**\n```\n%s\n```\n", ctx.CodeContext))
	}

	// Include full conversation history
	if len(ctx.ConversationHistory) > 0 {
		sb.WriteString(formatConversationHistory(ctx.ConversationHistory))
	}

	sb.WriteString(fmt.Sprintf("\n**User's Current Message:**\n%s\n", ctx.CommentBody))

	sb.WriteString("\nRespond naturally and helpfully. ")
	if len(ctx.ConversationHistory) > 0 {
		sb.WriteString("Continue the conversation naturally, referencing previous messages when relevant. ")
	}
	sb.WriteString("If you're unsure what the user wants, ask clarifying questions or suggest what commands are available.")

	return sb.String()
}

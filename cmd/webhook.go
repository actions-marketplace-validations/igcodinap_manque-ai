package cmd

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/igcodinap/manque-ai/internal"
	"github.com/igcodinap/manque-ai/pkg/ai"
	"github.com/igcodinap/manque-ai/pkg/commands"
	"github.com/igcodinap/manque-ai/pkg/github"
	"github.com/igcodinap/manque-ai/pkg/state"
	"github.com/spf13/cobra"
)

var (
	webhookPort   int
	webhookSecret string
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Start webhook server for interactive commands",
	Long:  `Starts an HTTP server that listens for GitHub webhook events and responds to @manque commands in PR comments.`,
	Run:   runWebhook,
}

func init() {
	webhookCmd.Flags().IntVar(&webhookPort, "port", 8080, "Port to listen on")
	webhookCmd.Flags().StringVar(&webhookSecret, "secret", "", "GitHub webhook secret (or GITHUB_WEBHOOK_SECRET env var)")
	rootCmd.AddCommand(webhookCmd)
}

// WebhookPayload represents a GitHub webhook event
type WebhookPayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	} `json:"issue"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
	Comment struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Path     *string `json:"path"`     // File path for review comments
		Position *int    `json:"position"` // Line position for review comments
		Line     *int    `json:"line"`     // Line number for review comments
	} `json:"comment"`
	Repository struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

func runWebhook(cmd *cobra.Command, args []string) {
	debug, _ := cmd.Flags().GetBool("debug")
	internal.InitLogger(debug)

	config, err := internal.LoadConfig()
	if err != nil {
		internal.Logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	if err := config.Validate(); err != nil {
		internal.Logger.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	// Get webhook secret from flag or env
	secret := webhookSecret
	if secret == "" {
		secret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	}

	// Initialize clients
	githubClient := github.NewClient(config.GitHubToken, config.GitHubAPIURL)
	aiClient, err := ai.NewClient(ai.Config{
		Provider: config.LLMProvider,
		APIKey:   config.LLMAPIKey,
		Model:    config.LLMModel,
		BaseURL:  config.LLMBaseURL,
	})
	if err != nil {
		internal.Logger.Error("Failed to initialize AI client", "error", err)
		os.Exit(1)
	}

	handler := NewWebhookHandler(githubClient, aiClient, config, secret)

	http.HandleFunc("/webhook", handler.HandleWebhook)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", webhookPort)
	internal.Logger.Info("Starting webhook server", "port", webhookPort)
	if err := http.ListenAndServe(addr, nil); err != nil {
		internal.Logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

// WebhookHandler handles GitHub webhook events
type WebhookHandler struct {
	githubClient   *github.Client
	aiClient       ai.Client
	config         *internal.Config
	webhookSecret  string
	commandParser  *commands.Parser
	commandHandler *commands.Handler
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(ghClient *github.Client, aiClient ai.Client, config *internal.Config, secret string) *WebhookHandler {
	return &WebhookHandler{
		githubClient:   ghClient,
		aiClient:       aiClient,
		config:         config,
		webhookSecret:  secret,
		commandParser:  commands.NewParser("manque"),
		commandHandler: commands.NewHandler(aiClient, config),
	}
}

// HandleWebhook handles incoming GitHub webhook events
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Error("Failed to read request body", "error", err)
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}

	// Verify signature if secret is configured
	if h.webhookSecret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if !h.verifySignature(body, signature) {
			internal.Logger.Warn("Invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	eventType := r.Header.Get("X-GitHub-Event")
	internal.Logger.Debug("Received webhook event", "type", eventType)

	switch eventType {
	case "issue_comment":
		h.handleIssueComment(body, w)
	case "pull_request_review_comment":
		h.handleReviewComment(body, w)
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Event type not handled"))
	}
}

func (h *WebhookHandler) verifySignature(body []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	actualMAC := strings.TrimPrefix(signature, "sha256=")

	return hmac.Equal([]byte(expectedMAC), []byte(actualMAC))
}

func (h *WebhookHandler) handleIssueComment(body []byte, w http.ResponseWriter) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		internal.Logger.Error("Failed to parse webhook payload", "error", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Only handle created comments
	if payload.Action != "created" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check if bot is mentioned
	if !h.commandParser.IsBotMentioned(payload.Comment.Body) {
		w.WriteHeader(http.StatusOK)
		return
	}

	internal.Logger.Info("Bot mentioned in comment",
		"repo", payload.Repository.FullName,
		"issue", payload.Issue.Number,
		"user", payload.Comment.User.Login)

	// Parse commands from comment
	cmds := h.commandParser.Parse(payload.Comment.Body, payload.Comment.ID, "", 0)
	if len(cmds) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Get PR info for context
	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name
	prNumber := payload.Issue.Number

	// Fetch conversation history for context
	thread, err := h.githubClient.GetIssueCommentThread(owner, repo, prNumber)
	var conversationHistory []commands.ConversationMessage
	if err != nil {
		internal.Logger.Warn("Failed to fetch conversation history", "error", err)
	} else {
		// Convert to commands.ConversationMessage
		for _, msg := range thread {
			conversationHistory = append(conversationHistory, commands.ConversationMessage{
				Author: msg.Author,
				Body:   msg.Body,
				IsBot:  msg.IsBot,
			})
		}
	}

	// Build command context
	cmdCtx := &commands.CommandContext{
		Ctx:                 context.Background(),
		PRTitle:             payload.Issue.Title,
		PRDescription:       payload.Issue.Body,
		PRNumber:            prNumber,
		Repository:          payload.Repository.FullName,
		CommentBody:         payload.Comment.Body,
		ConversationHistory: conversationHistory,
	}

	// Load session if exists
	sessionManager := state.NewSessionManager(payload.Repository.FullName, prNumber)
	cmdCtx.Session = sessionManager.GetOrCreateSession(payload.Issue.Body)

	// Process commands
	for _, cmd := range cmds {
		result, err := h.commandHandler.Handle(cmd, cmdCtx)
		if err != nil {
			internal.Logger.Error("Failed to handle command", "error", err, "command", cmd.Type)
			continue
		}

		// Post response as comment
		if result.Response != "" {
			err = h.githubClient.CreateComment(owner, repo, prNumber, result.Response)
			if err != nil {
				internal.Logger.Error("Failed to post response", "error", err)
			}
		}

		// Handle dismiss action
		if result.DismissIssue && result.DismissedHash != "" && cmdCtx.Session != nil {
			cmdCtx.Session.DismissIssue(result.DismissedHash, result.DismissReason)
			// Note: Session would need to be persisted back to PR body
			// This requires updating the PR description with the new session marker
		}

		// Handle regenerate action
		if result.TriggerReview {
			internal.Logger.Info("Triggering full review", "pr", prNumber)
			// This would trigger a new review - could be done async
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Commands processed"))
}

func (h *WebhookHandler) handleReviewComment(body []byte, w http.ResponseWriter) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		internal.Logger.Error("Failed to parse webhook payload", "error", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Only handle created comments
	if payload.Action != "created" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check if bot is mentioned
	if !h.commandParser.IsBotMentioned(payload.Comment.Body) {
		w.WriteHeader(http.StatusOK)
		return
	}

	internal.Logger.Info("Bot mentioned in review comment",
		"repo", payload.Repository.FullName,
		"pr", payload.PullRequest.Number,
		"user", payload.Comment.User.Login)

	// Get file and line context from review comment
	file := ""
	line := 0
	if payload.Comment.Path != nil {
		file = *payload.Comment.Path
	}
	if payload.Comment.Line != nil {
		line = *payload.Comment.Line
	}

	// Parse commands from comment
	cmds := h.commandParser.Parse(payload.Comment.Body, payload.Comment.ID, file, line)
	if len(cmds) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name
	prNumber := payload.PullRequest.Number

	// Fetch conversation thread for context
	thread, err := h.githubClient.GetCommentThread(owner, repo, prNumber, payload.Comment.ID)
	var conversationHistory []commands.ConversationMessage
	if err != nil {
		internal.Logger.Warn("Failed to fetch conversation thread", "error", err)
	} else {
		// Convert to commands.ConversationMessage
		for _, msg := range thread {
			conversationHistory = append(conversationHistory, commands.ConversationMessage{
				Author: msg.Author,
				Body:   msg.Body,
				IsBot:  msg.IsBot,
			})
		}
	}

	// Build command context with file context
	cmdCtx := &commands.CommandContext{
		Ctx:                 context.Background(),
		PRTitle:             payload.PullRequest.Title,
		PRDescription:       payload.PullRequest.Body,
		PRNumber:            prNumber,
		Repository:          payload.Repository.FullName,
		CommentBody:         payload.Comment.Body,
		FilePath:            file,
		FileLine:            line,
		ConversationHistory: conversationHistory,
	}

	// Load session
	sessionManager := state.NewSessionManager(payload.Repository.FullName, prNumber)
	cmdCtx.Session = sessionManager.GetOrCreateSession(payload.PullRequest.Body)

	// Process commands
	for _, cmd := range cmds {
		result, err := h.commandHandler.Handle(cmd, cmdCtx)
		if err != nil {
			internal.Logger.Error("Failed to handle command", "error", err, "command", cmd.Type)
			continue
		}

		// Reply to the review comment thread
		if result.Response != "" {
			err = h.githubClient.ReplyToComment(owner, repo, prNumber, payload.Comment.ID, result.Response)
			if err != nil {
				internal.Logger.Error("Failed to reply to comment", "error", err)
				// Fall back to issue comment
				_ = h.githubClient.CreateComment(owner, repo, prNumber, result.Response)
			}
		}

		// Handle dismiss action
		if result.DismissIssue && result.DismissedHash != "" && cmdCtx.Session != nil {
			cmdCtx.Session.DismissIssue(result.DismissedHash, result.DismissReason)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Commands processed"))
}

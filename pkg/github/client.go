package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v60/github"
	"github.com/igcodinap/manque-ai/internal"
	"golang.org/x/oauth2"
)

type Client struct {
	client *github.Client
	ctx    context.Context
}

type PRInfo struct {
	Number      int
	Title       string
	Description string
	Repository  string
	Owner       string
	Diff        string
	HeadSHA     string
}

type GitHubEvent struct {
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

func NewClient(token, apiURL string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	var client *github.Client
	if apiURL != "" && apiURL != "https://api.github.com" {
		// GitHub Enterprise
		client, _ = github.NewEnterpriseClient(apiURL, apiURL, tc)
	} else {
		client = github.NewClient(tc)
	}

	return &Client{
		client: client,
		ctx:    ctx,
	}
}

func (c *Client) GetPRFromEvent(eventPath string) (*PRInfo, error) {
	data, err := os.ReadFile(eventPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read GitHub event file: %w", err)
	}

	var event GitHubEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub event: %w", err)
	}

	owner := event.Repository.Owner.Login
	repo := event.Repository.Name
	prNumber := event.PullRequest.Number

	return c.GetPR(owner, repo, prNumber)
}

func (c *Client) GetPR(owner, repo string, number int) (*PRInfo, error) {
	pr, _, err := c.client.PullRequests.Get(c.ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR: %w", err)
	}

	// Get the diff
	diff, err := c.getPRDiff(owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR diff: %w", err)
	}

	return &PRInfo{
		Number:      number,
		Title:       pr.GetTitle(),
		Description: pr.GetBody(),
		Repository:  fmt.Sprintf("%s/%s", owner, repo),
		Owner:       owner,
		Diff:        diff,
		HeadSHA:     pr.GetHead().GetSHA(),
	}, nil
}

func (c *Client) GetPRFromURL(url string) (*PRInfo, error) {
	// Parse GitHub PR URL: https://github.com/owner/repo/pull/123
	parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if len(parts) < 7 || parts[2] != "github.com" || parts[5] != "pull" {
		return nil, fmt.Errorf("invalid GitHub PR URL format")
	}

	owner := parts[3]
	repo := parts[4]
	prNumber, err := strconv.Atoi(parts[6])
	if err != nil {
		return nil, fmt.Errorf("invalid PR number: %w", err)
	}

	return c.GetPR(owner, repo, prNumber)
}

func (c *Client) getPRDiff(owner, repo string, number int) (string, error) {
	diff, _, err := c.client.PullRequests.GetRaw(c.ctx, owner, repo, number, github.RawOptions{
		Type: github.Diff,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get PR diff: %w", err)
	}

	return diff, nil
}

func (c *Client) UpdatePR(owner, repo string, number int, title, body *string) error {
	update := &github.PullRequest{}
	if title != nil {
		update.Title = title
	}
	if body != nil {
		update.Body = body
	}

	_, _, err := c.client.PullRequests.Edit(c.ctx, owner, repo, number, update)
	if err != nil {
		return fmt.Errorf("failed to update PR: %w", err)
	}

	return nil
}

// BotCommentMarker is used to identify comments created by this bot
const BotCommentMarker = "<!-- manque-ai-bot -->"

func (c *Client) CreateComment(owner, repo string, number int, body string) error {
	// Add marker to identify bot comments
	markedBody := BotCommentMarker + "\n" + body
	comment := &github.IssueComment{
		Body: &markedBody,
	}

	_, _, err := c.client.Issues.CreateComment(c.ctx, owner, repo, number, comment)
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	return nil
}

// FindBotComment finds an existing comment created by this bot
func (c *Client) FindBotComment(owner, repo string, number int) (*github.IssueComment, error) {
	comments, _, err := c.client.Issues.ListComments(c.ctx, owner, repo, number, &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	for _, comment := range comments {
		if comment.Body != nil && strings.HasPrefix(*comment.Body, BotCommentMarker) {
			return comment, nil
		}
	}

	return nil, nil
}

// CreateOrUpdateComment creates a new comment or updates an existing bot comment
func (c *Client) CreateOrUpdateComment(owner, repo string, number int, body string) error {
	existingComment, err := c.FindBotComment(owner, repo, number)
	if err != nil {
		return err
	}

	markedBody := BotCommentMarker + "\n" + body

	if existingComment != nil {
		// Update existing comment
		existingComment.Body = &markedBody
		_, _, err = c.client.Issues.EditComment(c.ctx, owner, repo, *existingComment.ID, existingComment)
		if err != nil {
			return fmt.Errorf("failed to update comment: %w", err)
		}
		return nil
	}

	// Create new comment
	comment := &github.IssueComment{
		Body: &markedBody,
	}
	_, _, err = c.client.Issues.CreateComment(c.ctx, owner, repo, number, comment)
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	return nil
}

// ListReviewComments lists all review comments on a pull request
func (c *Client) ListReviewComments(owner, repo string, number int) ([]*github.PullRequestComment, error) {
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allComments []*github.PullRequestComment
	for {
		comments, resp, err := c.client.PullRequests.ListComments(c.ctx, owner, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list review comments: %w", err)
		}
		allComments = append(allComments, comments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// ExistingComment represents a comment that already exists on a PR
type ExistingComment struct {
	ID        int64
	Path      string
	StartLine int
	EndLine   int
	Body      string
	IsBot     bool // True if created by manque-ai
}

// GetExistingCommentsByLocation returns existing comments indexed by file:line
func (c *Client) GetExistingCommentsByLocation(owner, repo string, number int) (map[string]*ExistingComment, error) {
	comments, err := c.ListReviewComments(owner, repo, number)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*ExistingComment)
	for _, comment := range comments {
		if comment.Path == nil {
			continue
		}

		startLine := 0
		endLine := 0
		if comment.StartLine != nil {
			startLine = *comment.StartLine
		}
		if comment.Line != nil {
			endLine = *comment.Line
		}

		// Create location key
		key := fmt.Sprintf("%s:%d:%d", *comment.Path, startLine, endLine)

		// Check if this is a bot comment
		isBot := comment.Body != nil && strings.Contains(*comment.Body, BotCommentMarker)

		result[key] = &ExistingComment{
			ID:        comment.GetID(),
			Path:      *comment.Path,
			StartLine: startLine,
			EndLine:   endLine,
			Body:      comment.GetBody(),
			IsBot:     isBot,
		}
	}

	return result, nil
}

// ConversationMessage represents a single message in a conversation thread
type ConversationMessage struct {
	Author    string
	Body      string
	IsBot     bool
	CreatedAt string
	ID        int64
}

// GetCommentThread gets the conversation thread for a review comment
func (c *Client) GetCommentThread(owner, repo string, number int, commentID int64) ([]ConversationMessage, error) {
	comments, err := c.ListReviewComments(owner, repo, number)
	if err != nil {
		return nil, err
	}

	var thread []ConversationMessage

	// Find the root comment and all replies
	var rootID int64
	for _, comment := range comments {
		if comment.GetID() == commentID {
			// Check if this comment is a reply to another
			// InReplyTo is *int64 in the go-github library
			if comment.InReplyTo != nil {
				rootID = *comment.InReplyTo
			} else {
				rootID = commentID
			}
			break
		}
	}

	// If we couldn't find the comment, just return the single comment
	if rootID == 0 {
		rootID = commentID
	}

	// Collect all comments in this thread
	for _, comment := range comments {
		// Include the root comment
		if comment.GetID() == rootID {
			thread = append(thread, ConversationMessage{
				Author:    comment.GetUser().GetLogin(),
				Body:      comment.GetBody(),
				IsBot:     strings.Contains(comment.GetBody(), BotCommentMarker),
				CreatedAt: comment.GetCreatedAt().String(),
				ID:        comment.GetID(),
			})
		}
		// Include replies to the root
		if comment.InReplyTo != nil && *comment.InReplyTo == rootID {
			thread = append(thread, ConversationMessage{
				Author:    comment.GetUser().GetLogin(),
				Body:      comment.GetBody(),
				IsBot:     strings.Contains(comment.GetBody(), BotCommentMarker),
				CreatedAt: comment.GetCreatedAt().String(),
				ID:        comment.GetID(),
			})
		}
	}

	return thread, nil
}

// GetIssueCommentThread gets conversation thread from issue comments
func (c *Client) GetIssueCommentThread(owner, repo string, number int) ([]ConversationMessage, error) {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var thread []ConversationMessage
	for {
		comments, resp, err := c.client.Issues.ListComments(c.ctx, owner, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list issue comments: %w", err)
		}

		for _, comment := range comments {
			thread = append(thread, ConversationMessage{
				Author:    comment.GetUser().GetLogin(),
				Body:      comment.GetBody(),
				IsBot:     strings.Contains(comment.GetBody(), BotCommentMarker),
				CreatedAt: comment.GetCreatedAt().String(),
				ID:        comment.GetID(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return thread, nil
}

// ReplyToComment adds a reply to an existing review comment
func (c *Client) ReplyToComment(owner, repo string, number int, commentID int64, body string) error {
	comment := &github.PullRequestComment{
		Body: &body,
	}

	_, _, err := c.client.PullRequests.CreateCommentInReplyTo(c.ctx, owner, repo, number, body, commentID)
	if err != nil {
		return fmt.Errorf("failed to reply to comment: %w", err)
	}
	_ = comment // unused but keeping for potential future use

	return nil
}

// UpdateComment updates an existing review comment
func (c *Client) UpdateComment(owner, repo string, commentID int64, body string) error {
	comment := &github.PullRequestComment{
		Body: &body,
	}

	_, _, err := c.client.PullRequests.EditComment(c.ctx, owner, repo, commentID, comment)
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}

	return nil
}

// CreateReviewOptions configures the review creation behavior
type CreateReviewOptions struct {
	IsIncremental bool // If true, reply to existing comments instead of creating new ones
}

func (c *Client) CreateReview(owner, repo string, number int, comments []*github.DraftReviewComment, body *string, action string) error {
	return c.CreateReviewWithOptions(owner, repo, number, comments, body, action, CreateReviewOptions{})
}

func (c *Client) CreateReviewWithOptions(owner, repo string, number int, comments []*github.DraftReviewComment, body *string, action string, opts CreateReviewOptions) error {
	internal.Logger.Debug("CreateReview called", "incoming_comments", len(comments), "action", action, "incremental", opts.IsIncremental)

	// 1. Fetch existing comments to prevent duplicates and enable threading
	existingByLocation, err := c.GetExistingCommentsByLocation(owner, repo, number)
	if err != nil {
		return fmt.Errorf("failed to fetch existing comments: %w", err)
	}
	internal.Logger.Debug("Fetched existing comments from GitHub", "count", len(existingByLocation))

	// 2. Process comments: deduplicate, thread, or create new
	var newComments []*github.DraftReviewComment
	skippedDuplicates := 0
	threadedReplies := 0

	for _, comment := range comments {
		if comment.Path == nil || comment.Body == nil {
			continue
		}

		startLine := 0
		endLine := 0
		if comment.StartLine != nil {
			startLine = *comment.StartLine
		}
		if comment.Line != nil {
			endLine = *comment.Line
		}

		locationKey := fmt.Sprintf("%s:%d:%d", *comment.Path, startLine, endLine)

		// Check if there's an existing comment at this location
		existing, hasExisting := existingByLocation[locationKey]

		if hasExisting {
			// Check if the content is the same (exact duplicate)
			if strings.TrimSpace(existing.Body) == strings.TrimSpace(*comment.Body) {
				skippedDuplicates++
				internal.Logger.Debug("Skipping duplicate comment", "path", *comment.Path, "line", endLine)
				continue
			}

			// For incremental reviews, reply to existing comment instead of creating new
			if opts.IsIncremental && existing.IsBot {
				replyBody := fmt.Sprintf("**Update on re-review:**\n\n%s", *comment.Body)
				if err := c.ReplyToComment(owner, repo, number, existing.ID, replyBody); err != nil {
					internal.Logger.Warn("Failed to reply to comment, will create new", "error", err)
					newComments = append(newComments, comment)
				} else {
					threadedReplies++
					internal.Logger.Debug("Replied to existing comment", "path", *comment.Path, "line", endLine)
				}
				continue
			}
		}

		// Create new comment
		newComments = append(newComments, comment)
	}

	internal.Logger.Debug("Comment processing complete",
		"new_comments", len(newComments),
		"skipped_duplicates", skippedDuplicates,
		"threaded_replies", threadedReplies)

	// 3. If nothing new to post, just return (or post body if it's new)
	if len(newComments) == 0 && (body == nil || *body == "") {
		internal.Logger.Debug("No new comments to post, returning early")
		return nil
	}

	// Use provided action or default to COMMENT
	event := action
	if event == "" {
		event = "COMMENT"
	}
	review := &github.PullRequestReviewRequest{
		Body:     body,
		Event:    &event,
		Comments: newComments,
	}

	internal.Logger.Debug("Posting review to GitHub", "comment_count", len(newComments))
	_, _, err = c.client.PullRequests.CreateReview(c.ctx, owner, repo, number, review)
	if err != nil {
		return fmt.Errorf("failed to create review: %w", err)
	}

	return nil
}

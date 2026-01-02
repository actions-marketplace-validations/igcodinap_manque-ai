package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v60/github"
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

func (c *Client) CreateReview(owner, repo string, number int, comments []*github.DraftReviewComment, body *string) error {
	event := "COMMENT"
	review := &github.PullRequestReviewRequest{
		Body:     body,
		Event:    &event,
		Comments: comments,
	}
	
	_, _, err := c.client.PullRequests.CreateReview(c.ctx, owner, repo, number, review)
	if err != nil {
		return fmt.Errorf("failed to create review: %w", err)
	}
	
	return nil
}
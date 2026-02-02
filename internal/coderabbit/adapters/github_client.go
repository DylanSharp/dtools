package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
	"github.com/DylanSharp/dtools/internal/coderabbit/ports"
)

// GitHubCLIClient implements ports.GitHubClient using the gh CLI
type GitHubCLIClient struct{}

// NewGitHubCLIClient creates a new GitHub CLI client
func NewGitHubCLIClient() *GitHubCLIClient {
	return &GitHubCLIClient{}
}

// ghPR is the JSON structure returned by gh pr view
type ghPR struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
	HeadRefOid  string `json:"headRefOid"`
	BaseRefOid  string `json:"baseRefOid"`
	Author     struct {
		Login string `json:"login"`
	} `json:"author"`
	State string `json:"state"`
	URL   string `json:"url"`
}

// ghReview is the JSON structure for a PR review
type ghReview struct {
	ID          int    `json:"id"`
	Body        string `json:"body"`
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ghComment is the JSON structure for a review comment
type ghComment struct {
	ID           int    `json:"id"`
	Body         string `json:"body"`
	Path         string `json:"path"`
	Line         int    `json:"line"`
	OriginalLine int    `json:"original_line"`
	Position     *int   `json:"position"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	HTMLURL      string `json:"html_url"`
	User         struct {
		Login string `json:"login"`
	} `json:"user"`
}

// GetPullRequest fetches PR details using gh CLI
func (c *GitHubCLIClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (*ports.PullRequest, error) {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", owner, repo),
		"--json", "number,title,body,headRefName,baseRefName,headRefOid,baseRefOid,author,state,url",
	}

	out, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, domain.ErrGitHubAPI("failed to fetch PR", err)
	}

	var pr ghPR
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, domain.ErrJSONParse("failed to parse PR response", err)
	}

	return &ports.PullRequest{
		Number:     pr.Number,
		Title:      pr.Title,
		Body:       pr.Body,
		Branch:     pr.HeadRefName,
		BaseBranch: pr.BaseRefName,
		HeadCommit: pr.HeadRefOid,
		BaseCommit: pr.BaseRefOid,
		Author:     pr.Author.Login,
		State:      pr.State,
		URL:        pr.URL,
	}, nil
}

// ListCodeRabbitComments fetches all CodeRabbit review comments for a PR using GraphQL
// This includes the thread's isResolved status which is not available via REST API
func (c *GitHubCLIClient) ListCodeRabbitComments(ctx context.Context, owner, repo string, number int) ([]domain.Comment, error) {
	// Use GraphQL to fetch review threads with resolved status
	query := fmt.Sprintf(`
	{
		repository(owner: "%s", name: "%s") {
			pullRequest(number: %d) {
				reviewThreads(first: 100) {
					nodes {
						id
						isResolved
						isOutdated
						comments(first: 10) {
							nodes {
								databaseId
								body
								path
								line: originalLine
								createdAt
								updatedAt
								url
								author {
									login
								}
							}
						}
					}
				}
			}
		}
	}`, owner, repo, number)

	args := []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", query)}
	out, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, domain.ErrGitHubAPI("failed to fetch review threads", err)
	}

	var response struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						Nodes []struct {
							ID         string `json:"id"`
							IsResolved bool   `json:"isResolved"`
							IsOutdated bool   `json:"isOutdated"`
							Comments   struct {
								Nodes []struct {
									DatabaseID int       `json:"databaseId"`
									Body       string    `json:"body"`
									Path       string    `json:"path"`
									Line       int       `json:"line"`
									CreatedAt  time.Time `json:"createdAt"`
									UpdatedAt  time.Time `json:"updatedAt"`
									URL        string    `json:"url"`
									Author     struct {
										Login string `json:"login"`
									} `json:"author"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal(out, &response); err != nil {
		return nil, domain.ErrJSONParse("failed to parse GraphQL response", err)
	}

	var allComments []domain.Comment
	for _, thread := range response.Data.Repository.PullRequest.ReviewThreads.Nodes {
		for _, comment := range thread.Comments.Nodes {
			// Only include CodeRabbit comments
			if !strings.Contains(strings.ToLower(comment.Author.Login), "coderabbit") {
				continue
			}

			domainComment := domain.Comment{
				ID:         comment.DatabaseID,
				FilePath:   comment.Path,
				LineNumber: comment.Line,
				Body:       comment.Body,
				AIPrompt:   extractAIPrompt(comment.Body),
				Author:     comment.Author.Login,
				CreatedAt:  comment.CreatedAt,
				UpdatedAt:  comment.UpdatedAt,
				URL:        comment.URL,
				IsNit:      isNit(comment.Body),
				IsOutdated: thread.IsOutdated,
				IsResolved: thread.IsResolved, // Now properly set from thread!
			}
			allComments = append(allComments, domainComment)
		}
	}

	// Also fetch general PR comments (issue comments) - these don't have threads
	issueCommentsArgs := []string{
		"api",
		fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, number),
		"--paginate",
	}

	issueCommentsOut, err := c.runGH(ctx, issueCommentsArgs...)
	if err == nil {
		var issueComments []ghComment
		if json.Unmarshal(issueCommentsOut, &issueComments) == nil {
			for _, comment := range issueComments {
				if !strings.Contains(strings.ToLower(comment.User.Login), "coderabbit") {
					continue
				}
				// Skip auto-generated summary comments
				if isAutoGeneratedComment(comment.Body) {
					continue
				}

				createdAt, _ := time.Parse(time.RFC3339, comment.CreatedAt)
				updatedAt, _ := time.Parse(time.RFC3339, comment.UpdatedAt)

				domainComment := domain.Comment{
					ID:        comment.ID,
					Body:      comment.Body,
					AIPrompt:  extractAIPrompt(comment.Body),
					Author:    comment.User.Login,
					CreatedAt: createdAt,
					UpdatedAt: updatedAt,
					URL:       comment.HTMLURL,
					IsNit:     isNit(comment.Body),
				}
				allComments = append(allComments, domainComment)
			}
		}
	}

	if len(allComments) == 0 {
		return nil, domain.ErrNoComments()
	}

	return allComments, nil
}

// GetLatestCommit returns the HEAD commit SHA of the PR
func (c *GitHubCLIClient) GetLatestCommit(ctx context.Context, owner, repo string, number int) (string, error) {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", owner, repo),
		"--json", "headRefOid",
		"-q", ".headRefOid",
	}

	out, err := c.runGH(ctx, args...)
	if err != nil {
		return "", domain.ErrGitHubAPI("failed to get latest commit", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// GetDiff returns the diff for the PR
func (c *GitHubCLIClient) GetDiff(ctx context.Context, owner, repo string, number int) (string, error) {
	args := []string{
		"pr", "diff", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", owner, repo),
	}

	out, err := c.runGH(ctx, args...)
	if err != nil {
		return "", domain.ErrGitHubAPI("failed to get diff", err)
	}

	return string(out), nil
}

// GetCurrentPR detects the PR number from the current branch
func (c *GitHubCLIClient) GetCurrentPR(ctx context.Context) (int, error) {
	args := []string{"pr", "view", "--json", "number", "-q", ".number"}

	out, err := c.runGH(ctx, args...)
	if err != nil {
		return 0, domain.ErrGitHubAPI("failed to detect current PR", err)
	}

	var number int
	if err := json.Unmarshal(out, &number); err != nil {
		return 0, domain.ErrJSONParse("failed to parse PR number", err)
	}

	return number, nil
}

// GetRepoInfo returns the owner and repo from the current git remote
func (c *GitHubCLIClient) GetRepoInfo(ctx context.Context) (owner, repo string, err error) {
	cmd := exec.CommandContext(ctx, "git", "config", "--get", "remote.origin.url")
	out, err := cmd.Output()
	if err != nil {
		return "", "", domain.ErrGitHubAPI("failed to get remote URL", err)
	}

	url := strings.TrimSpace(string(out))

	// Parse GitHub URL (supports both HTTPS and SSH formats)
	// https://github.com/owner/repo.git
	// git@github.com:owner/repo.git
	re := regexp.MustCompile(`github\.com[:/]([^/]+)/([^/.]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 3 {
		return "", "", domain.ErrGitHubAPI("could not parse GitHub URL from remote", nil)
	}

	return matches[1], matches[2], nil
}

// GetCurrentBranch returns the current git branch name
func (c *GitHubCLIClient) GetCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return "", domain.ErrGitHubAPI("failed to get current branch", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// ReplyToComment posts a reply to a review comment
func (c *GitHubCLIClient) ReplyToComment(ctx context.Context, owner, repo string, prNumber, commentID int, body string) error {
	// Use the GraphQL API to reply to a review comment
	// The REST API doesn't support replying directly to review comments
	args := []string{
		"api",
		fmt.Sprintf("repos/%s/%s/pulls/%d/comments/%d/replies", owner, repo, prNumber, commentID),
		"-f", fmt.Sprintf("body=%s", body),
	}

	_, err := c.runGH(ctx, args...)
	if err != nil {
		return domain.ErrGitHubAPI("failed to reply to comment", err)
	}

	return nil
}

// ResolveComment marks a review comment thread as resolved using GraphQL
func (c *GitHubCLIClient) ResolveComment(ctx context.Context, owner, repo string, prNumber, commentID int) error {
	// First, we need to get the thread ID for this comment via GraphQL
	// The REST API doesn't support resolving comments directly
	query := fmt.Sprintf(`
		query {
			repository(owner: "%s", name: "%s") {
				pullRequest(number: %d) {
					reviewThreads(first: 100) {
						nodes {
							id
							isResolved
							comments(first: 1) {
								nodes {
									databaseId
								}
							}
						}
					}
				}
			}
		}
	`, owner, repo, prNumber)

	args := []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", query)}
	out, err := c.runGH(ctx, args...)
	if err != nil {
		return domain.ErrGitHubAPI("failed to fetch review threads", err)
	}

	// Parse response to find the thread ID for our comment
	var response struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						Nodes []struct {
							ID         string `json:"id"`
							IsResolved bool   `json:"isResolved"`
							Comments   struct {
								Nodes []struct {
									DatabaseID int `json:"databaseId"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal(out, &response); err != nil {
		return domain.ErrJSONParse("failed to parse review threads", err)
	}

	// Find the thread containing our comment
	var threadID string
	for _, thread := range response.Data.Repository.PullRequest.ReviewThreads.Nodes {
		if thread.IsResolved {
			continue
		}
		for _, comment := range thread.Comments.Nodes {
			if comment.DatabaseID == commentID {
				threadID = thread.ID
				break
			}
		}
		if threadID != "" {
			break
		}
	}

	if threadID == "" {
		// Comment not found or already resolved
		return nil
	}

	// Resolve the thread
	mutation := fmt.Sprintf(`
		mutation {
			resolveReviewThread(input: {threadId: "%s"}) {
				thread {
					isResolved
				}
			}
		}
	`, threadID)

	args = []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", mutation)}
	_, err = c.runGH(ctx, args...)
	if err != nil {
		return domain.ErrGitHubAPI("failed to resolve comment thread", err)
	}

	return nil
}

// runGH executes a gh CLI command and returns the output
func (c *GitHubCLIClient) runGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh command failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}
	return out, nil
}

// extractAIPrompt extracts the "Prompt for AI Agents" section from a comment body
func extractAIPrompt(body string) string {
	patterns := []string{
		`🤖\s*Prompt for AI Agents[\s\S]*?` + "```" + `([\s\S]*?)` + "```",
		`<summary>🤖\s*Prompt for AI Agents</summary>[\s\S]*?` + "```" + `([\s\S]*?)` + "```",
		`Prompt for AI Agents[\s\S]*?` + "```" + `([\s\S]*?)` + "```",
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(body)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// isNit checks if a comment is a nitpick
func isNit(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "nit:") ||
		strings.Contains(lower, "nitpick") ||
		regexp.MustCompile(`\b(nit|nitpick)\b`).MatchString(lower)
}

// isAutoGeneratedComment checks if a comment is auto-generated
func isAutoGeneratedComment(body string) bool {
	markers := []string{
		"auto-generated comment",
		"auto-generated reply",
		"summarized by CodeRabbit",
		"## Walkthrough",
		"## Summary",
		"✅ Test",
		"All tests passed",
	}
	for _, marker := range markers {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

// parseNitpicksFromReview extracts nitpick comments from the review body HTML
func parseNitpicksFromReview(body string) []domain.Comment {
	// Look for the nitpicks section
	nitpicksRe := regexp.MustCompile(`<summary>🧹\s*Nitpick comments \((\d+)\)</summary>([\s\S]*?)</details>`)
	matches := nitpicksRe.FindStringSubmatch(body)
	if len(matches) < 3 {
		return nil
	}

	content := matches[2]
	var comments []domain.Comment

	// Parse individual nitpicks: `line-range`: **title** body
	// Go doesn't support lookahead, so use a simpler pattern and split manually
	commentRe := regexp.MustCompile("`(\\d+)(?:-(\\d+))?`:\\s*\\*\\*([^*]+)\\*\\*")
	commentMatches := commentRe.FindAllStringSubmatchIndex(content, -1)

	for i, matchIdx := range commentMatches {
		if len(matchIdx) < 8 {
			continue
		}

		lineStart := content[matchIdx[2]:matchIdx[3]]
		title := content[matchIdx[6]:matchIdx[7]]

		// Get body: from end of title to next comment or end
		bodyStart := matchIdx[1]
		var bodyEnd int
		if i+1 < len(commentMatches) {
			bodyEnd = commentMatches[i+1][0]
		} else {
			bodyEnd = len(content)
		}
		body := strings.TrimSpace(content[bodyStart:bodyEnd])

		// Extract file path from surrounding context if available
		filePath := ""
		idx := matchIdx[0]
		if idx > 0 {
			fileRe := regexp.MustCompile(`<summary>([^<]+)</summary>`)
			fileMatches := fileRe.FindAllStringSubmatch(content[:idx], -1)
			if len(fileMatches) > 0 {
				filePath = strings.TrimSpace(fileMatches[len(fileMatches)-1][1])
				// Clean up the file path
				filePath = regexp.MustCompile(`\s*\(\d+\)`).ReplaceAllString(filePath, "")
			}
		}

		comment := domain.Comment{
			ID:        -i - 1000, // Synthetic ID for nitpicks
			FilePath:  filePath,
			LineNumber: parseInt(lineStart),
			Body:      fmt.Sprintf("**%s** %s", title, body),
			IsNit:     true,
			CreatedAt: time.Now(),
		}
		comments = append(comments, comment)
	}

	return comments
}

// parseInt parses a string to int, returning 0 on error
func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// parseOutsideDiffFromReview extracts outside-diff comments from the review body
func parseOutsideDiffFromReview(body string) []domain.Comment {
	// Look for the outside diff section
	outsideRe := regexp.MustCompile(`⚠️\s*Outside diff range comments \((\d+)\)([\s\S]*?)</details>`)
	matches := outsideRe.FindStringSubmatch(body)
	if len(matches) < 3 {
		return nil
	}

	content := matches[2]
	var comments []domain.Comment

	// Parse individual comments similar to nitpicks
	// Go doesn't support lookahead, so use a simpler pattern
	commentRe := regexp.MustCompile("`(\\d+)(?:-(\\d+))?`:\\s*\\*\\*([^*]+)\\*\\*")
	commentMatches := commentRe.FindAllStringSubmatchIndex(content, -1)

	for i, matchIdx := range commentMatches {
		if len(matchIdx) < 8 {
			continue
		}

		lineStart := content[matchIdx[2]:matchIdx[3]]
		title := content[matchIdx[6]:matchIdx[7]]

		// Get body: from end of title to next comment or end
		bodyStart := matchIdx[1]
		var bodyEnd int
		if i+1 < len(commentMatches) {
			bodyEnd = commentMatches[i+1][0]
		} else {
			bodyEnd = len(content)
		}
		commentBody := strings.TrimSpace(content[bodyStart:bodyEnd])

		filePath := ""
		idx := matchIdx[0]
		if idx > 0 {
			fileRe := regexp.MustCompile(`<summary>([^<]+)</summary>`)
			fileMatches := fileRe.FindAllStringSubmatch(content[:idx], -1)
			if len(fileMatches) > 0 {
				filePath = strings.TrimSpace(fileMatches[len(fileMatches)-1][1])
				filePath = regexp.MustCompile(`\s*\(\d+\)`).ReplaceAllString(filePath, "")
			}
		}

		comment := domain.Comment{
			ID:            -i - 2000, // Synthetic ID for outside-diff
			FilePath:      filePath,
			LineNumber:    parseInt(lineStart),
			Body:          fmt.Sprintf("**%s** %s", title, commentBody),
			IsOutsideDiff: true,
			CreatedAt:     time.Now(),
		}
		comments = append(comments, comment)
	}

	return comments
}

package ports

import (
	"context"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
)

// GitHubClient abstracts GitHub API operations
type GitHubClient interface {
	// GetPullRequest fetches PR details
	GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error)

	// ListCodeRabbitComments fetches all CodeRabbit review comments for a PR
	ListCodeRabbitComments(ctx context.Context, owner, repo string, number int) ([]domain.Comment, error)

	// GetLatestCommit returns the HEAD commit SHA of the PR
	GetLatestCommit(ctx context.Context, owner, repo string, number int) (string, error)

	// GetDiff returns the diff for the PR
	GetDiff(ctx context.Context, owner, repo string, number int) (string, error)

	// GetCurrentPR detects the PR number from the current branch
	GetCurrentPR(ctx context.Context) (int, error)

	// GetRepoInfo returns the owner and repo from the current git remote
	GetRepoInfo(ctx context.Context) (owner, repo string, err error)

	// GetCurrentBranch returns the current git branch name
	GetCurrentBranch(ctx context.Context) (string, error)

	// ReplyToComment posts a reply to a review comment
	ReplyToComment(ctx context.Context, owner, repo string, prNumber, commentID int, body string) error

	// ResolveComment marks a review comment thread as resolved
	ResolveComment(ctx context.Context, owner, repo string, prNumber, commentID int) error
}

// PullRequest represents GitHub PR metadata
type PullRequest struct {
	Number     int
	Title      string
	Body       string
	Branch     string
	BaseBranch string
	HeadCommit string
	BaseCommit string
	Author     string
	State      string
	URL        string
}

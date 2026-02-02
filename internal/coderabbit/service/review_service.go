package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DylanSharp/dtools/internal/coderabbit/adapters"
	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
	"github.com/DylanSharp/dtools/internal/coderabbit/ports"
	"github.com/DylanSharp/dtools/internal/coderabbit/state"
)

// ReviewService orchestrates the review process
type ReviewService struct {
	github       ports.GitHubClient
	ci           ports.CIProvider
	aiProvider   ports.AIProvider
	promptBuilder *PromptBuilder
	parser       *adapters.ClaudeStreamParser
}

// NewReviewService creates a new review service
func NewReviewService(
	github ports.GitHubClient,
	ci ports.CIProvider,
	aiProvider ports.AIProvider,
) *ReviewService {
	return &ReviewService{
		github:        github,
		ci:            ci,
		aiProvider:    aiProvider,
		promptBuilder: NewPromptBuilder(),
		parser:        adapters.NewClaudeStreamParser(),
	}
}

// ReviewConfig contains configuration for a review
type ReviewConfig struct {
	PRNumber        int
	IncludeNits     bool
	IncludeOutdated bool
	MaxDiffMb       float64
	ResetState      bool // If true, clear state before starting
	MarkAddressed   bool // If true, mark comments as resolved on GitHub
}

// StartReview initiates a PR review and returns a channel of thoughts
func (s *ReviewService) StartReview(ctx context.Context, config ReviewConfig) (*domain.Review, <-chan domain.ThoughtChunk, error) {
	// Get repo info
	owner, repo, err := s.github.GetRepoInfo(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo info: %w", err)
	}

	repository := fmt.Sprintf("%s/%s", owner, repo)
	stateKey := state.GetStateKey(owner, repo, config.PRNumber)

	// Reset state if requested
	if config.ResetState {
		_ = state.Reset(stateKey)
	}

	// Load state for filtering processed comments
	trackerState, err := state.GetOrCreate(stateKey)
	if err != nil {
		// Non-fatal - continue without state tracking
		trackerState = &state.TrackerState{
			ProcessedCommentIDs: []int{},
			ProcessedByHash:     []string{},
			SeenComments:        make(map[int]state.SeenInfo),
		}
	}

	// Create review
	review := domain.NewReview(config.PRNumber, repository)
	review.Status = domain.ReviewStatusFetching

	// Fetch PR details
	pr, err := s.github.GetPullRequest(ctx, owner, repo, config.PRNumber)
	if err != nil {
		return nil, nil, err
	}

	review.Branch = pr.Branch
	review.BaseBranch = pr.BaseBranch
	review.HeadCommit = pr.HeadCommit
	review.BaseCommit = pr.BaseCommit
	review.Title = pr.Title
	review.Author = pr.Author

	// Fetch CodeRabbit comments
	comments, err := s.github.ListCodeRabbitComments(ctx, owner, repo, config.PRNumber)
	if err != nil {
		// No comments is not a fatal error
		if _, ok := err.(*domain.ReviewError); !ok || err.(*domain.ReviewError).Code != domain.ErrCodeNoComments {
			return nil, nil, err
		}
	}

	// Filter comments based on config (nits, outdated, etc.)
	filteredComments := s.filterComments(comments, config)

	// Track total found for UI display
	review.TotalFoundCount = len(filteredComments)

	// Filter out already processed comments using state
	unprocessedComments := state.FilterUnprocessed(trackerState, filteredComments)
	review.Comments = unprocessedComments
	review.RemainingCount = len(unprocessedComments)
	review.NewCommentsCount = len(unprocessedComments)
	review.AlreadyAddressed = review.TotalFoundCount - review.NewCommentsCount

	// Fetch CI status (includes failures and pending checks)
	ciStatus, err := s.ci.GetCIStatus(ctx, owner, repo, pr.HeadCommit)
	if err != nil {
		// CI status is optional - log but continue with empty status
		ciStatus = domain.CIStatus{}
	}
	review.CIFailures = ciStatus.Failures
	review.CIPendingCount = ciStatus.PendingCount
	review.CIPendingNames = ciStatus.PendingNames
	review.CIAllComplete = ciStatus.AllComplete()

	// Check if there's anything to review
	// Only mark satisfied if: no comments, no CI failures, AND all CI checks complete
	if len(unprocessedComments) == 0 && len(ciStatus.Failures) == 0 && ciStatus.AllComplete() {
		review.Status = domain.ReviewStatusSatisfied
		review.MarkSatisfied()
		return review, nil, nil
	}

	// Build prompt
	prompt := s.promptBuilder.BuildReviewPrompt(review)

	// Start Claude streaming
	review.Status = domain.ReviewStatusReviewing
	chunks, err := s.aiProvider.StreamReview(ctx, prompt)
	if err != nil {
		review.MarkFailed()
		return nil, nil, err
	}

	// Filter and transform chunks to thoughts
	thoughts := s.parser.FilterThoughts(chunks)

	// Capture values for goroutine
	markAddressed := config.MarkAddressed
	ghClient := s.github

	// Wrap the channel to track review state
	trackedThoughts := make(chan domain.ThoughtChunk, 100)
	go func() {
		defer close(trackedThoughts)
		for thought := range thoughts {
			review.AddThought(thought)
			review.ProcessedCount++
			review.CurrentFile = thought.File
			trackedThoughts <- thought
		}
		review.MarkCompleted()

		// Mark comments as processed after Claude finishes
		_ = state.MarkProcessed(stateKey, unprocessedComments, "")

		// Mark comments as resolved on GitHub if enabled
		if markAddressed {
			for _, comment := range unprocessedComments {
				if comment.ID > 0 { // Only real comments, not synthetic ones
					_ = ghClient.ResolveComment(ctx, owner, repo, config.PRNumber, comment.ID)
				}
			}
		}
	}()

	return review, trackedThoughts, nil
}

// DetectCurrentPR detects the PR number from the current branch
func (s *ReviewService) DetectCurrentPR(ctx context.Context) (int, error) {
	return s.github.GetCurrentPR(ctx)
}

// GetRepoInfo returns the owner and repo
func (s *ReviewService) GetRepoInfo(ctx context.Context) (owner, repo string, err error) {
	return s.github.GetRepoInfo(ctx)
}

// GetCurrentBranch returns the current branch name
func (s *ReviewService) GetCurrentBranch(ctx context.Context) (string, error) {
	return s.github.GetCurrentBranch(ctx)
}

// FetchReviewData fetches review data without starting Claude
func (s *ReviewService) FetchReviewData(ctx context.Context, config ReviewConfig) (*domain.Review, error) {
	owner, repo, err := s.github.GetRepoInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo info: %w", err)
	}

	repository := fmt.Sprintf("%s/%s", owner, repo)
	stateKey := state.GetStateKey(owner, repo, config.PRNumber)

	// Reset state if requested
	if config.ResetState {
		_ = state.Reset(stateKey)
	}

	// Load state for filtering
	trackerState, err := state.GetOrCreate(stateKey)
	if err != nil {
		trackerState = &state.TrackerState{
			ProcessedCommentIDs: []int{},
			ProcessedByHash:     []string{},
			SeenComments:        make(map[int]state.SeenInfo),
		}
	}

	review := domain.NewReview(config.PRNumber, repository)

	// Fetch PR details
	pr, err := s.github.GetPullRequest(ctx, owner, repo, config.PRNumber)
	if err != nil {
		return nil, err
	}

	review.Branch = pr.Branch
	review.BaseBranch = pr.BaseBranch
	review.HeadCommit = pr.HeadCommit
	review.BaseCommit = pr.BaseCommit
	review.Title = pr.Title
	review.Author = pr.Author

	// Fetch comments
	comments, err := s.github.ListCodeRabbitComments(ctx, owner, repo, config.PRNumber)
	if err != nil {
		if rerr, ok := err.(*domain.ReviewError); !ok || rerr.Code != domain.ErrCodeNoComments {
			return nil, err
		}
	}

	// Filter by config then by state
	filteredComments := s.filterComments(comments, config)
	review.TotalFoundCount = len(filteredComments)
	review.Comments = state.FilterUnprocessed(trackerState, filteredComments)
	review.RemainingCount = len(review.Comments)
	review.NewCommentsCount = len(review.Comments)
	review.AlreadyAddressed = review.TotalFoundCount - review.NewCommentsCount

	// Fetch CI status
	ciStatus, err := s.ci.GetCIStatus(ctx, owner, repo, pr.HeadCommit)
	if err == nil {
		review.CIFailures = ciStatus.Failures
		review.CIPendingCount = ciStatus.PendingCount
		review.CIPendingNames = ciStatus.PendingNames
		review.CIAllComplete = ciStatus.AllComplete()
	}

	return review, nil
}

// filterComments filters comments based on configuration
func (s *ReviewService) filterComments(comments []domain.Comment, config ReviewConfig) []domain.Comment {
	var filtered []domain.Comment

	for _, c := range comments {
		// Skip nits if not included
		if c.IsNit && !config.IncludeNits {
			continue
		}

		// Skip outdated if not included
		if c.IsOutdated && !config.IncludeOutdated {
			continue
		}

		// Skip resolved comments
		if c.IsResolved {
			continue
		}

		filtered = append(filtered, c)
	}

	return filtered
}

// CheckSatisfaction checks if CodeRabbit is satisfied with the current state
func (s *ReviewService) CheckSatisfaction(ctx context.Context, review *domain.Review) (SatisfactionResult, error) {
	detector := NewSatisfactionDetector()

	// Analyze Claude's thoughts
	thoughtResult := detector.AnalyzeReview(review)

	// Re-fetch comments to check current state
	owner, repo := s.parseRepository(review.Repository)
	comments, err := s.github.ListCodeRabbitComments(ctx, owner, repo, review.PRNumber)
	if err != nil {
		// If we can't fetch comments, use thought analysis only
		return thoughtResult, nil
	}

	// Analyze current comment state
	commentResult := detector.AnalyzeCodeRabbitReview(comments)

	// Combine results - both need to indicate satisfaction
	combined := SatisfactionResult{
		IsSatisfied:    thoughtResult.IsSatisfied && commentResult.IsSatisfied,
		Confidence:     (thoughtResult.Confidence + commentResult.Confidence) / 2,
		Reasons:        append(thoughtResult.Reasons, commentResult.Reasons...),
		ActionRequired: append(thoughtResult.ActionRequired, commentResult.ActionRequired...),
	}

	return combined, nil
}

// parseRepository parses "owner/repo" into separate values
func (s *ReviewService) parseRepository(repository string) (owner, repo string) {
	parts := strings.Split(repository, "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return repository, ""
}

// WatchOptions configures watch mode behavior
type WatchOptions struct {
	PollInterval         time.Duration
	CooldownDuration     time.Duration
	BatchWaitDuration    time.Duration // Wait for more comments before processing
	RequireManualConfirm bool
	IncludeNits          bool
	IncludeOutdated      bool
}

// DefaultWatchOptions returns default watch configuration
func DefaultWatchOptions() WatchOptions {
	return WatchOptions{
		PollInterval:         15 * time.Second,
		CooldownDuration:     3 * time.Minute,
		BatchWaitDuration:    30 * time.Second, // Wait for CodeRabbit to finish posting
		RequireManualConfirm: true,
		IncludeNits:          true,
		IncludeOutdated:      true,
	}
}

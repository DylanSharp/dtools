package domain

import (
	"fmt"
	"time"
)

// ReviewStatus represents the current state of a review
type ReviewStatus string

const (
	ReviewStatusPending    ReviewStatus = "pending"
	ReviewStatusFetching   ReviewStatus = "fetching"
	ReviewStatusReviewing  ReviewStatus = "reviewing"
	ReviewStatusCompleted  ReviewStatus = "completed"
	ReviewStatusSatisfied  ReviewStatus = "satisfied"
	ReviewStatusFailed     ReviewStatus = "failed"
)

// Review represents a complete PR review session
type Review struct {
	ID          string
	PRNumber    int
	Repository  string // owner/repo format
	Branch      string
	BaseBranch  string
	HeadCommit  string
	BaseCommit  string
	Title       string
	Author      string
	Status      ReviewStatus
	StartedAt   time.Time
	CompletedAt *time.Time

	// Review content
	Comments   []Comment
	CIFailures []CITestFailure
	Thoughts   []ThoughtChunk

	// Processing state
	ProcessedCount  int
	RemainingCount  int
	CurrentFile     string

	// Satisfaction tracking
	Satisfied       bool
	LastSatisfyCheck time.Time
	SatisfyCheckCount int
}

// NewReview creates a new Review with default values
func NewReview(prNumber int, repository string) *Review {
	return &Review{
		ID:         generateID(prNumber, repository),
		PRNumber:   prNumber,
		Repository: repository,
		Status:     ReviewStatusPending,
		StartedAt:  time.Now(),
		Comments:   []Comment{},
		CIFailures: []CITestFailure{},
		Thoughts:   []ThoughtChunk{},
	}
}

// generateID creates a unique ID for a review
func generateID(prNumber int, repository string) string {
	return fmt.Sprintf("%s#%d", repository, prNumber)
}

// TotalComments returns the total number of comments
func (r *Review) TotalComments() int {
	return len(r.Comments)
}

// AddThought appends a new thought chunk
func (r *Review) AddThought(thought ThoughtChunk) {
	r.Thoughts = append(r.Thoughts, thought)
}

// MarkCompleted marks the review as completed
func (r *Review) MarkCompleted() {
	now := time.Now()
	r.CompletedAt = &now
	r.Status = ReviewStatusCompleted
}

// MarkSatisfied marks the review as satisfied
func (r *Review) MarkSatisfied() {
	now := time.Now()
	r.CompletedAt = &now
	r.Status = ReviewStatusSatisfied
	r.Satisfied = true
}

// MarkFailed marks the review as failed
func (r *Review) MarkFailed() {
	r.Status = ReviewStatusFailed
}

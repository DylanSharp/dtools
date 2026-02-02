package domain

import (
	"time"
)

// StoryStatus represents the current state of a story
type StoryStatus string

const (
	StoryStatusPending   StoryStatus = "pending"
	StoryStatusBlocked   StoryStatus = "blocked"
	StoryStatusRunning   StoryStatus = "running"
	StoryStatusCompleted StoryStatus = "completed"
	StoryStatusFailed    StoryStatus = "failed"
)

// Story represents a user story from the PRD
type Story struct {
	ID                 string            `json:"id"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AcceptanceCriteria []string          `json:"acceptance_criteria"`
	DependsOn          []string          `json:"depends_on"`
	Priority           int               `json:"priority"`
	Status             StoryStatus       `json:"status"`
	StartedAt          *time.Time        `json:"started_at,omitempty"`
	CompletedAt        *time.Time        `json:"completed_at,omitempty"`
	Error              string            `json:"error,omitempty"`
	Attempts           int               `json:"attempts"`
	Notes              string            `json:"notes,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// NewStory creates a new story with default values
func NewStory(id, title string) *Story {
	return &Story{
		ID:                 id,
		Title:              title,
		AcceptanceCriteria: []string{},
		DependsOn:          []string{},
		Priority:           1,
		Status:             StoryStatusPending,
		Attempts:           0,
		Metadata:           make(map[string]string),
	}
}

// IsPending returns true if the story hasn't started
func (s *Story) IsPending() bool {
	return s.Status == StoryStatusPending
}

// IsBlocked returns true if the story is blocked by dependencies
func (s *Story) IsBlocked() bool {
	return s.Status == StoryStatusBlocked
}

// IsRunning returns true if the story is currently executing
func (s *Story) IsRunning() bool {
	return s.Status == StoryStatusRunning
}

// IsCompleted returns true if the story finished successfully
func (s *Story) IsCompleted() bool {
	return s.Status == StoryStatusCompleted
}

// IsFailed returns true if the story failed
func (s *Story) IsFailed() bool {
	return s.Status == StoryStatusFailed
}

// IsFinished returns true if the story is completed or failed
func (s *Story) IsFinished() bool {
	return s.IsCompleted() || s.IsFailed()
}

// MarkRunning marks the story as running
func (s *Story) MarkRunning() {
	now := time.Now()
	s.Status = StoryStatusRunning
	s.StartedAt = &now
	s.Attempts++
}

// MarkCompleted marks the story as completed
func (s *Story) MarkCompleted() {
	now := time.Now()
	s.Status = StoryStatusCompleted
	s.CompletedAt = &now
	s.Error = ""
}

// MarkFailed marks the story as failed with an error message
func (s *Story) MarkFailed(err string) {
	s.Status = StoryStatusFailed
	s.Error = err
}

// MarkBlocked marks the story as blocked
func (s *Story) MarkBlocked() {
	s.Status = StoryStatusBlocked
}

// MarkPending resets the story to pending
func (s *Story) MarkPending() {
	s.Status = StoryStatusPending
}

// Duration returns the time spent on the story
func (s *Story) Duration() time.Duration {
	if s.StartedAt == nil {
		return 0
	}
	if s.CompletedAt != nil {
		return s.CompletedAt.Sub(*s.StartedAt)
	}
	return time.Since(*s.StartedAt)
}

// HasDependencies returns true if the story has dependencies
func (s *Story) HasDependencies() bool {
	return len(s.DependsOn) > 0
}

// CanRun checks if the story can run given a set of completed story IDs
func (s *Story) CanRun(completedIDs map[string]bool) bool {
	if !s.IsPending() && !s.IsBlocked() {
		return false
	}
	for _, depID := range s.DependsOn {
		if !completedIDs[depID] {
			return false
		}
	}
	return true
}

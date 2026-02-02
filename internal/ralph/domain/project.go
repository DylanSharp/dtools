package domain

import (
	"fmt"
	"time"
)

// ProjectStatus represents the current state of a project
type ProjectStatus string

const (
	ProjectStatusInitialized ProjectStatus = "initialized"
	ProjectStatusRunning     ProjectStatus = "running"
	ProjectStatusCompleted   ProjectStatus = "completed"
	ProjectStatusFailed      ProjectStatus = "failed"
	ProjectStatusPaused      ProjectStatus = "paused"
)

// Project represents a PRD execution session
type Project struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	PRDPath      string        `json:"prd_path"`
	WorkDir      string        `json:"work_dir"`
	Stories      []*Story      `json:"stories"`
	Status       ProjectStatus `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	StartedAt    *time.Time    `json:"started_at,omitempty"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	CurrentStory *string       `json:"current_story,omitempty"` // ID of currently executing story
}

// NewProject creates a new project with default values
func NewProject(name, prdPath, workDir string) *Project {
	now := time.Now()
	return &Project{
		ID:        generateProjectID(name, now),
		Name:      name,
		PRDPath:   prdPath,
		WorkDir:   workDir,
		Stories:   []*Story{},
		Status:    ProjectStatusInitialized,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// generateProjectID creates a unique ID for a project
func generateProjectID(name string, t time.Time) string {
	return fmt.Sprintf("%s-%d", name, t.UnixNano())
}

// AddStory adds a story to the project
func (p *Project) AddStory(story *Story) {
	p.Stories = append(p.Stories, story)
	p.UpdatedAt = time.Now()
}

// GetStory returns a story by ID
func (p *Project) GetStory(id string) *Story {
	for _, s := range p.Stories {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// GetStoryByIndex returns a story by index
func (p *Project) GetStoryByIndex(index int) *Story {
	if index < 0 || index >= len(p.Stories) {
		return nil
	}
	return p.Stories[index]
}

// TotalStories returns the total number of stories
func (p *Project) TotalStories() int {
	return len(p.Stories)
}

// CompletedStories returns the number of completed stories
func (p *Project) CompletedStories() int {
	count := 0
	for _, s := range p.Stories {
		if s.IsCompleted() {
			count++
		}
	}
	return count
}

// PendingStories returns the number of pending stories
func (p *Project) PendingStories() int {
	count := 0
	for _, s := range p.Stories {
		if s.IsPending() {
			count++
		}
	}
	return count
}

// BlockedStories returns the number of blocked stories
func (p *Project) BlockedStories() int {
	count := 0
	for _, s := range p.Stories {
		if s.IsBlocked() {
			count++
		}
	}
	return count
}

// FailedStories returns the number of failed stories
func (p *Project) FailedStories() int {
	count := 0
	for _, s := range p.Stories {
		if s.IsFailed() {
			count++
		}
	}
	return count
}

// RunningStories returns the number of currently running stories
func (p *Project) RunningStories() int {
	count := 0
	for _, s := range p.Stories {
		if s.IsRunning() {
			count++
		}
	}
	return count
}

// RemainingStories returns the number of stories not yet completed
func (p *Project) RemainingStories() int {
	return p.TotalStories() - p.CompletedStories()
}

// Progress returns the completion percentage (0-100)
func (p *Project) Progress() int {
	if p.TotalStories() == 0 {
		return 100
	}
	return (p.CompletedStories() * 100) / p.TotalStories()
}

// GetCompletedIDs returns a map of completed story IDs
func (p *Project) GetCompletedIDs() map[string]bool {
	ids := make(map[string]bool)
	for _, s := range p.Stories {
		if s.IsCompleted() {
			ids[s.ID] = true
		}
	}
	return ids
}

// GetCompletedStories returns all completed stories
func (p *Project) GetCompletedStories() []*Story {
	var stories []*Story
	for _, s := range p.Stories {
		if s.IsCompleted() {
			stories = append(stories, s)
		}
	}
	return stories
}

// IsComplete returns true if all stories are completed
func (p *Project) IsComplete() bool {
	for _, s := range p.Stories {
		if !s.IsCompleted() {
			return false
		}
	}
	return len(p.Stories) > 0
}

// HasFailures returns true if any story has failed
func (p *Project) HasFailures() bool {
	for _, s := range p.Stories {
		if s.IsFailed() {
			return true
		}
	}
	return false
}

// MarkRunning marks the project as running
func (p *Project) MarkRunning() {
	now := time.Now()
	p.Status = ProjectStatusRunning
	p.StartedAt = &now
	p.UpdatedAt = now
}

// MarkCompleted marks the project as completed
func (p *Project) MarkCompleted() {
	now := time.Now()
	p.Status = ProjectStatusCompleted
	p.CompletedAt = &now
	p.CurrentStory = nil
	p.UpdatedAt = now
}

// MarkFailed marks the project as failed
func (p *Project) MarkFailed() {
	p.Status = ProjectStatusFailed
	p.CurrentStory = nil
	p.UpdatedAt = time.Now()
}

// MarkPaused marks the project as paused
func (p *Project) MarkPaused() {
	p.Status = ProjectStatusPaused
	p.UpdatedAt = time.Now()
}

// SetCurrentStory sets the currently executing story
func (p *Project) SetCurrentStory(storyID string) {
	p.CurrentStory = &storyID
	p.UpdatedAt = time.Now()
}

// ClearCurrentStory clears the currently executing story
func (p *Project) ClearCurrentStory() {
	p.CurrentStory = nil
	p.UpdatedAt = time.Now()
}

// Duration returns the total time spent on the project
func (p *Project) Duration() time.Duration {
	if p.StartedAt == nil {
		return 0
	}
	if p.CompletedAt != nil {
		return p.CompletedAt.Sub(*p.StartedAt)
	}
	return time.Since(*p.StartedAt)
}

// UpdateBlockedStatus updates blocked status for all stories based on dependencies
func (p *Project) UpdateBlockedStatus() {
	completedIDs := p.GetCompletedIDs()
	for _, s := range p.Stories {
		if s.IsPending() || s.IsBlocked() {
			if s.CanRun(completedIDs) {
				s.MarkPending()
			} else {
				s.MarkBlocked()
			}
		}
	}
}

// StoryExists checks if a story with the given ID exists
func (p *Project) StoryExists(id string) bool {
	return p.GetStory(id) != nil
}

// ValidateDependencies checks that all dependency references are valid
func (p *Project) ValidateDependencies() error {
	for _, s := range p.Stories {
		for _, depID := range s.DependsOn {
			if !p.StoryExists(depID) {
				return fmt.Errorf("story %q depends on non-existent story %q", s.ID, depID)
			}
		}
	}
	return nil
}

// DetectCircularDependencies checks for circular dependencies in the project
func (p *Project) DetectCircularDependencies() error {
	// Build adjacency list
	deps := make(map[string][]string)
	for _, story := range p.Stories {
		deps[story.ID] = story.DependsOn
	}

	// DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	var path []string

	var hasCycle func(id string) bool
	hasCycle = func(id string) bool {
		visited[id] = true
		recStack[id] = true
		path = append(path, id)

		for _, depID := range deps[id] {
			if !visited[depID] {
				if hasCycle(depID) {
					return true
				}
			} else if recStack[depID] {
				path = append(path, depID)
				return true
			}
		}

		path = path[:len(path)-1]
		recStack[id] = false
		return false
	}

	for _, story := range p.Stories {
		if !visited[story.ID] {
			path = nil
			if hasCycle(story.ID) {
				return ErrCircularDependency(path)
			}
		}
	}

	return nil
}

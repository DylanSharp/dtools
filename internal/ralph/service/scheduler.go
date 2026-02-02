package service

import (
	"sort"

	"github.com/DylanSharp/dtools/internal/ralph/domain"
)

// Scheduler determines story execution order
type Scheduler struct{}

// NewScheduler creates a new scheduler
func NewScheduler() *Scheduler {
	return &Scheduler{}
}

// GetNextStory returns the next story that can be executed
// Returns nil if no story is ready (all blocked or completed)
func (s *Scheduler) GetNextStory(project *domain.Project) *domain.Story {
	completedIDs := project.GetCompletedIDs()

	// Get all ready stories (pending with all dependencies met)
	var readyStories []*domain.Story
	for _, story := range project.Stories {
		if story.CanRun(completedIDs) {
			readyStories = append(readyStories, story)
		}
	}

	if len(readyStories) == 0 {
		return nil
	}

	// Sort by priority (lower number = higher priority)
	sort.Slice(readyStories, func(i, j int) bool {
		return readyStories[i].Priority < readyStories[j].Priority
	})

	return readyStories[0]
}

// GetReadyStories returns all stories that are ready to execute
func (s *Scheduler) GetReadyStories(project *domain.Project) []*domain.Story {
	completedIDs := project.GetCompletedIDs()

	var readyStories []*domain.Story
	for _, story := range project.Stories {
		if story.CanRun(completedIDs) {
			readyStories = append(readyStories, story)
		}
	}

	// Sort by priority
	sort.Slice(readyStories, func(i, j int) bool {
		return readyStories[i].Priority < readyStories[j].Priority
	})

	return readyStories
}

// GetBlockedStories returns all stories that are blocked by dependencies
func (s *Scheduler) GetBlockedStories(project *domain.Project) []*domain.Story {
	completedIDs := project.GetCompletedIDs()

	var blocked []*domain.Story
	for _, story := range project.Stories {
		if (story.IsPending() || story.IsBlocked()) && !story.CanRun(completedIDs) {
			blocked = append(blocked, story)
		}
	}

	return blocked
}

// GetDependencyChain returns the chain of dependencies for a story
func (s *Scheduler) GetDependencyChain(project *domain.Project, storyID string) []string {
	visited := make(map[string]bool)
	var chain []string

	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true

		story := project.GetStory(id)
		if story == nil {
			return
		}

		for _, depID := range story.DependsOn {
			visit(depID)
		}

		chain = append(chain, id)
	}

	visit(storyID)
	return chain
}

// GetDependents returns all stories that depend on the given story
func (s *Scheduler) GetDependents(project *domain.Project, storyID string) []*domain.Story {
	var dependents []*domain.Story

	for _, story := range project.Stories {
		for _, depID := range story.DependsOn {
			if depID == storyID {
				dependents = append(dependents, story)
				break
			}
		}
	}

	return dependents
}

// ValidateDependencies validates all dependency references in the project
func (s *Scheduler) ValidateDependencies(project *domain.Project) error {
	return project.ValidateDependencies()
}

// DetectCircularDependencies checks for circular dependencies
func (s *Scheduler) DetectCircularDependencies(project *domain.Project) error {
	return project.DetectCircularDependencies()
}

// CanExecute checks if a story can be executed given the current project state
func (s *Scheduler) CanExecute(project *domain.Project, storyID string) (bool, string) {
	story := project.GetStory(storyID)
	if story == nil {
		return false, "story not found"
	}

	if story.IsCompleted() {
		return false, "story already completed"
	}

	if story.IsRunning() {
		return false, "story is already running"
	}

	completedIDs := project.GetCompletedIDs()

	// Check dependencies
	for _, depID := range story.DependsOn {
		if !completedIDs[depID] {
			depStory := project.GetStory(depID)
			if depStory == nil {
				return false, "dependency " + depID + " not found"
			}
			return false, "waiting for dependency: " + depID + " (" + depStory.Title + ")"
		}
	}

	return true, ""
}

// GetExecutionOrder returns the optimal execution order for all stories
func (s *Scheduler) GetExecutionOrder(project *domain.Project) []string {
	// Topological sort
	deps := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, story := range project.Stories {
		deps[story.ID] = story.DependsOn
		if _, ok := inDegree[story.ID]; !ok {
			inDegree[story.ID] = 0
		}
		for _, depID := range story.DependsOn {
			inDegree[story.ID]++
			if _, ok := inDegree[depID]; !ok {
				inDegree[depID] = 0
			}
		}
	}

	// Find all stories with no dependencies
	var queue []string
	for _, story := range project.Stories {
		if inDegree[story.ID] == 0 {
			queue = append(queue, story.ID)
		}
	}

	// Sort queue by priority
	sort.Slice(queue, func(i, j int) bool {
		si := project.GetStory(queue[i])
		sj := project.GetStory(queue[j])
		if si == nil || sj == nil {
			return false
		}
		return si.Priority < sj.Priority
	})

	var order []string
	for len(queue) > 0 {
		// Take the highest priority item
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)

		// Update dependents
		for _, story := range project.Stories {
			for _, depID := range story.DependsOn {
				if depID == id {
					inDegree[story.ID]--
					if inDegree[story.ID] == 0 {
						queue = append(queue, story.ID)
						// Re-sort by priority
						sort.Slice(queue, func(i, j int) bool {
							si := project.GetStory(queue[i])
							sj := project.GetStory(queue[j])
							if si == nil || sj == nil {
								return false
							}
							return si.Priority < sj.Priority
						})
					}
				}
			}
		}
	}

	return order
}

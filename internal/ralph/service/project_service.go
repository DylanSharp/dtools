package service

import (
	"context"

	"github.com/DylanSharp/dtools/internal/ralph/domain"
	"github.com/DylanSharp/dtools/internal/ralph/ports"
)

// ProjectService orchestrates ralph operations
type ProjectService struct {
	parser     ports.PRDParser
	executor   ports.Executor
	repository ports.Repository
	scheduler  *Scheduler
}

// NewProjectService creates a new project service
func NewProjectService(
	parser ports.PRDParser,
	executor ports.Executor,
	repository ports.Repository,
) *ProjectService {
	return &ProjectService{
		parser:     parser,
		executor:   executor,
		repository: repository,
		scheduler:  NewScheduler(),
	}
}

// InitProject initializes a project from a PRD file
func (s *ProjectService) InitProject(prdPath string) (*domain.Project, error) {
	// Parse PRD
	project, err := s.parser.Parse(prdPath)
	if err != nil {
		return nil, err
	}

	// Validate
	if err := s.parser.Validate(project); err != nil {
		return nil, err
	}

	// Check for circular dependencies
	if err := s.scheduler.DetectCircularDependencies(project); err != nil {
		return nil, err
	}

	// Update blocked status
	project.UpdateBlockedStatus()

	// Save state
	if err := s.repository.Save(project); err != nil {
		return nil, err
	}

	return project, nil
}

// GetProject retrieves a project by ID or PRD path
func (s *ProjectService) GetProject(idOrPath string) (*domain.Project, error) {
	// Try by ID first
	if s.repository.Exists(idOrPath) {
		return s.repository.Load(idOrPath)
	}

	// Try by PRD path
	return s.repository.LoadByPRDPath(idOrPath)
}

// ListProjects returns all known projects
func (s *ProjectService) ListProjects() ([]ports.ProjectInfo, error) {
	return s.repository.List()
}

// DeleteProject removes a project
func (s *ProjectService) DeleteProject(projectID string) error {
	return s.repository.Delete(projectID)
}

// RunProject executes all stories in a project sequentially
func (s *ProjectService) RunProject(ctx context.Context, projectID string) (<-chan domain.ExecutionEvent, error) {
	// Load project
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	// Check if already complete
	if project.IsComplete() {
		return nil, domain.ErrAllStoriesCompleted()
	}

	// Reset any stories stuck in "running" state from previous crashes
	for _, story := range project.Stories {
		if story.IsRunning() {
			story.MarkPending()
		}
	}
	project.UpdateBlockedStatus()

	// Check executor availability
	if !s.executor.IsAvailable() {
		return nil, domain.ErrClaudeNotFound()
	}

	events := make(chan domain.ExecutionEvent, 100)

	go func() {
		defer close(events)

		// Mark project as running
		project.MarkRunning()
		if err := s.repository.Save(project); err != nil {
			events <- domain.NewErrorEvent("", "failed to save project state: "+err.Error())
		}

		// Send project started event
		events <- domain.NewProjectStartedEvent(project)

		// Execute stories sequentially
		for {
			select {
			case <-ctx.Done():
				project.MarkPaused()
				if err := s.repository.Save(project); err != nil {
					events <- domain.NewErrorEvent("", "failed to save project state: "+err.Error())
				}
				events <- domain.NewErrorEvent("", "execution cancelled")
				return
			default:
			}

			// Get next story
			story := s.scheduler.GetNextStory(project)
			if story == nil {
				// No more stories to execute
				break
			}

			// Execute the story
			if err := s.executeStory(ctx, project, story, events); err != nil {
				// Story failed - continue with others if possible
				events <- domain.NewErrorEvent(story.ID, err.Error())
			}

			// Save progress
			if err := s.repository.Save(project); err != nil {
				events <- domain.NewErrorEvent("", "failed to save progress: "+err.Error())
			}
		}

		// Check final state
		if project.IsComplete() {
			project.MarkCompleted()
			events <- domain.NewProjectCompleteEvent(project)
		} else if project.HasFailures() {
			project.MarkFailed()
			events <- domain.NewExecutionEvent(domain.EventTypeProjectFailed, "", "project has failed stories")
		}

		if err := s.repository.Save(project); err != nil {
			events <- domain.NewErrorEvent("", "failed to save final state: "+err.Error())
		}
	}()

	return events, nil
}

// RunStory executes a single story
func (s *ProjectService) RunStory(ctx context.Context, projectID, storyID string) (<-chan domain.ExecutionEvent, error) {
	// Load project
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	// Get story
	story := project.GetStory(storyID)
	if story == nil {
		return nil, domain.ErrStoryNotFound(storyID)
	}

	// Check if can execute
	canRun, reason := s.scheduler.CanExecute(project, storyID)
	if !canRun {
		return nil, domain.NewError("cannot_execute", reason)
	}

	// Check executor availability
	if !s.executor.IsAvailable() {
		return nil, domain.ErrClaudeNotFound()
	}

	events := make(chan domain.ExecutionEvent, 100)

	go func() {
		defer close(events)

		// Execute the story
		if err := s.executeStory(ctx, project, story, events); err != nil {
			events <- domain.NewErrorEvent(story.ID, err.Error())
		}

		// Save progress
		s.repository.Save(project)
	}()

	return events, nil
}

// executeStory runs a single story and sends events to the channel
func (s *ProjectService) executeStory(ctx context.Context, project *domain.Project, story *domain.Story, events chan<- domain.ExecutionEvent) error {
	// Mark story as running
	story.MarkRunning()
	project.SetCurrentStory(story.ID)

	// Build execution context
	execCtx := ports.NewExecutionContext(project)

	// Execute story
	storyEvents, err := s.executor.Execute(ctx, story, execCtx)
	if err != nil {
		story.MarkFailed(err.Error())
		project.ClearCurrentStory()
		return err
	}

	// Forward events
	for event := range storyEvents {
		events <- event
	}

	// Mark story as completed
	story.MarkCompleted()
	project.ClearCurrentStory()
	project.UpdateBlockedStatus()

	return nil
}

// GetProjectStatus returns the current status of a project
func (s *ProjectService) GetProjectStatus(projectID string) (*domain.Project, error) {
	return s.GetProject(projectID)
}

// GetScheduler returns the scheduler for external use
func (s *ProjectService) GetScheduler() *Scheduler {
	return s.scheduler
}

// RefreshProject reloads a project from its PRD file
func (s *ProjectService) RefreshProject(projectID string) (*domain.Project, error) {
	// Load existing project
	existing, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	// Re-parse PRD
	updated, err := s.parser.Parse(existing.PRDPath)
	if err != nil {
		return nil, err
	}

	// Merge state from existing project
	for _, story := range updated.Stories {
		existingStory := existing.GetStory(story.ID)
		if existingStory != nil {
			// Preserve execution state
			story.Status = existingStory.Status
			story.StartedAt = existingStory.StartedAt
			story.CompletedAt = existingStory.CompletedAt
			story.Error = existingStory.Error
			story.Attempts = existingStory.Attempts
		}
	}

	// Update metadata
	updated.ID = existing.ID
	updated.CreatedAt = existing.CreatedAt
	updated.StartedAt = existing.StartedAt

	// Validate
	if err := s.parser.Validate(updated); err != nil {
		return nil, err
	}

	// Update blocked status
	updated.UpdateBlockedStatus()

	// Save
	if err := s.repository.Save(updated); err != nil {
		return nil, err
	}

	return updated, nil
}

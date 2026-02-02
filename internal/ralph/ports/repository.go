package ports

import (
	"github.com/DylanSharp/dtools/internal/ralph/domain"
)

// Repository persists project state
type Repository interface {
	// Save persists a project's state
	Save(project *domain.Project) error

	// Load retrieves a project by ID
	Load(projectID string) (*domain.Project, error)

	// LoadByPRDPath retrieves a project by its PRD path
	LoadByPRDPath(prdPath string) (*domain.Project, error)

	// List returns all known projects
	List() ([]ProjectInfo, error)

	// Delete removes a project from storage
	Delete(projectID string) error

	// Exists checks if a project exists
	Exists(projectID string) bool
}

// ProjectInfo contains summary information about a project
type ProjectInfo struct {
	ID          string
	Name        string
	PRDPath     string
	Status      domain.ProjectStatus
	TotalStories int
	CompletedStories int
	CreatedAt   string
	UpdatedAt   string
}

// ProgressInfo contains progress information for display
type ProgressInfo struct {
	TotalStories     int
	CompletedStories int
	PendingStories   int
	BlockedStories   int
	FailedStories    int
	RunningStories   int
	ProgressPercent  int
}

// GetProgressInfo extracts progress info from a project
func GetProgressInfo(project *domain.Project) ProgressInfo {
	return ProgressInfo{
		TotalStories:     project.TotalStories(),
		CompletedStories: project.CompletedStories(),
		PendingStories:   project.PendingStories(),
		BlockedStories:   project.BlockedStories(),
		FailedStories:    project.FailedStories(),
		RunningStories:   project.RunningStories(),
		ProgressPercent:  project.Progress(),
	}
}

package ports

import (
	"github.com/DylanSharp/dtools/internal/ralph/domain"
)

// PRDParser parses Product Requirement Documents
type PRDParser interface {
	// Parse reads a PRD file and returns a Project with stories
	Parse(path string) (*domain.Project, error)

	// Validate validates a project's structure and dependencies
	Validate(project *domain.Project) error
}

// PRDParseOptions contains options for parsing PRD files
type PRDParseOptions struct {
	// WorkDir overrides the working directory (defaults to PRD file's directory)
	WorkDir string

	// ProjectName overrides the project name (defaults to PRD filename)
	ProjectName string
}

// DefaultPRDParseOptions returns default parsing options
func DefaultPRDParseOptions() PRDParseOptions {
	return PRDParseOptions{}
}

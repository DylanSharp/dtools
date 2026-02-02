package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/DylanSharp/dtools/internal/ralph/domain"
	"github.com/DylanSharp/dtools/internal/ralph/ports"
)

// JSONRepository implements ports.Repository using JSON files
type JSONRepository struct {
	stateDir string
}

// NewJSONRepository creates a new JSON-based repository
func NewJSONRepository() (*JSONRepository, error) {
	// Use ~/.config/dtools/ralph/projects/ for state
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, domain.ErrStatePersistence("init", err)
	}

	stateDir := filepath.Join(homeDir, ".config", "dtools", "ralph", "projects")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, domain.ErrStatePersistence("init", err)
	}

	return &JSONRepository{
		stateDir: stateDir,
	}, nil
}

// NewJSONRepositoryWithPath creates a repository with a custom state directory
func NewJSONRepositoryWithPath(stateDir string) (*JSONRepository, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, domain.ErrStatePersistence("init", err)
	}
	return &JSONRepository{stateDir: stateDir}, nil
}

// Save persists a project's state
func (r *JSONRepository) Save(project *domain.Project) error {
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return domain.ErrStatePersistence("save", err)
	}

	filename := r.getFilename(project.ID)
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return domain.ErrStatePersistence("save", err)
	}

	return nil
}

// Load retrieves a project by ID
func (r *JSONRepository) Load(projectID string) (*domain.Project, error) {
	filename := r.getFilename(projectID)
	return r.loadFromFile(filename)
}

// LoadByPRDPath retrieves a project by its PRD path
func (r *JSONRepository) LoadByPRDPath(prdPath string) (*domain.Project, error) {
	// Get absolute path for comparison
	absPRDPath, err := filepath.Abs(prdPath)
	if err != nil {
		return nil, domain.ErrStatePersistence("load", err)
	}

	// Search through all projects
	projects, err := r.List()
	if err != nil {
		return nil, err
	}

	for _, info := range projects {
		project, err := r.Load(info.ID)
		if err != nil {
			continue
		}

		absProjectPRD, err := filepath.Abs(project.PRDPath)
		if err != nil {
			continue
		}

		if absProjectPRD == absPRDPath {
			return project, nil
		}
	}

	return nil, domain.ErrProjectNotFound(prdPath)
}

// List returns all known projects
func (r *JSONRepository) List() ([]ports.ProjectInfo, error) {
	entries, err := os.ReadDir(r.stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ports.ProjectInfo{}, nil
		}
		return nil, domain.ErrStatePersistence("list", err)
	}

	var projects []ports.ProjectInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		projectID := strings.TrimSuffix(entry.Name(), ".json")
		project, err := r.Load(projectID)
		if err != nil {
			continue // Skip invalid files
		}

		projects = append(projects, ports.ProjectInfo{
			ID:               project.ID,
			Name:             project.Name,
			PRDPath:          project.PRDPath,
			Status:           project.Status,
			TotalStories:     project.TotalStories(),
			CompletedStories: project.CompletedStories(),
			CreatedAt:        project.CreatedAt.Format("2006-01-02 15:04"),
			UpdatedAt:        project.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}

	return projects, nil
}

// Delete removes a project from storage
func (r *JSONRepository) Delete(projectID string) error {
	filename := r.getFilename(projectID)
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return domain.ErrProjectNotFound(projectID)
		}
		return domain.ErrStatePersistence("delete", err)
	}
	return nil
}

// Exists checks if a project exists
func (r *JSONRepository) Exists(projectID string) bool {
	filename := r.getFilename(projectID)
	_, err := os.Stat(filename)
	return err == nil
}

// loadFromFile loads a project from a specific file
func (r *JSONRepository) loadFromFile(filename string) (*domain.Project, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrProjectNotFound(filename)
		}
		return nil, domain.ErrStatePersistence("load", err)
	}

	var project domain.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, domain.ErrStatePersistence("load", err)
	}

	return &project, nil
}

// getFilename returns the state file path for a project ID
func (r *JSONRepository) getFilename(projectID string) string {
	// Sanitize project ID to be safe for filenames
	safeID := sanitizeFilename(projectID)
	return filepath.Join(r.stateDir, safeID+".json")
}

// sanitizeFilename makes a string safe for use as a filename
func sanitizeFilename(s string) string {
	// Replace unsafe characters with underscores
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(s)
}

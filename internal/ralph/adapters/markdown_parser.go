package adapters

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/DylanSharp/dtools/internal/ralph/domain"
	"github.com/DylanSharp/dtools/internal/ralph/ports"
)

// MarkdownPRDParser implements ports.PRDParser for markdown files
type MarkdownPRDParser struct {
	options ports.PRDParseOptions
}

// NewMarkdownPRDParser creates a new markdown PRD parser
func NewMarkdownPRDParser(options ports.PRDParseOptions) *MarkdownPRDParser {
	return &MarkdownPRDParser{options: options}
}

// Parse reads a PRD markdown file and returns a Project with stories
func (p *MarkdownPRDParser) Parse(path string) (*domain.Project, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, domain.ErrPRDNotFound(path)
	}

	file, err := os.Open(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrPRDNotFound(absPath)
		}
		return nil, domain.ErrPRDInvalid("cannot open file", err)
	}
	defer file.Close()

	// Determine project name and work directory
	projectName := p.options.ProjectName
	if projectName == "" {
		projectName = strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	}

	workDir := p.options.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(absPath)
	}

	project := domain.NewProject(projectName, absPath, workDir)

	// Parse the markdown file
	scanner := bufio.NewScanner(file)
	var currentStory *domain.Story
	var inStory bool
	var inAcceptanceCriteria bool
	var inDescription bool
	var descriptionLines []string
	var currentSection string

	// Regex patterns
	storyHeaderRegex := regexp.MustCompile(`^###?\s*(?:Story:?\s*)?\[?([A-Z0-9_-]+)\]?\s*[:\-]?\s*(.*)$`)
	priorityRegex := regexp.MustCompile(`(?i)\*\*priority\*\*:\s*(\d+)`)
	dependsOnRegex := regexp.MustCompile(`(?i)\*\*depends?\s*on\*\*:\s*\[([^\]]*)\]`)
	statusRegex := regexp.MustCompile(`(?i)\*\*status\*\*:\s*(\w+)`)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		// Check for story header (### [STORY-001] Title or ### Story: STORY-001 - Title)
		if matches := storyHeaderRegex.FindStringSubmatch(trimmedLine); len(matches) >= 3 {
			// Save previous story if exists
			if currentStory != nil {
				if len(descriptionLines) > 0 {
					currentStory.Description = strings.TrimSpace(strings.Join(descriptionLines, "\n"))
				}
				project.AddStory(currentStory)
			}

			// Start new story
			storyID := matches[1]
			storyTitle := strings.TrimSpace(matches[2])
			if storyTitle == "" {
				storyTitle = storyID
			}

			currentStory = domain.NewStory(storyID, storyTitle)
			inStory = true
			inAcceptanceCriteria = false
			inDescription = false
			descriptionLines = nil
			currentSection = ""
			continue
		}

		// Check for project title (# Title)
		if strings.HasPrefix(trimmedLine, "# ") && !inStory {
			project.Name = strings.TrimPrefix(trimmedLine, "# ")
			continue
		}

		// Process content within a story
		if inStory && currentStory != nil {
			// Check for section headers
			lowerLine := strings.ToLower(trimmedLine)

			if strings.Contains(lowerLine, "acceptance criteria") || strings.Contains(lowerLine, "criteria:") {
				if len(descriptionLines) > 0 && currentSection == "description" {
					currentStory.Description = strings.TrimSpace(strings.Join(descriptionLines, "\n"))
					descriptionLines = nil
				}
				inAcceptanceCriteria = true
				inDescription = false
				currentSection = "acceptance"
				continue
			}

			if strings.Contains(lowerLine, "description:") || strings.Contains(lowerLine, "**description**") {
				inDescription = true
				inAcceptanceCriteria = false
				currentSection = "description"
				continue
			}

			if strings.Contains(lowerLine, "notes:") || strings.Contains(lowerLine, "**notes**") {
				if len(descriptionLines) > 0 && currentSection == "description" {
					currentStory.Description = strings.TrimSpace(strings.Join(descriptionLines, "\n"))
					descriptionLines = nil
				}
				inAcceptanceCriteria = false
				inDescription = false
				currentSection = "notes"
				continue
			}

			// Parse priority
			if matches := priorityRegex.FindStringSubmatch(trimmedLine); len(matches) >= 2 {
				if priority, err := strconv.Atoi(matches[1]); err == nil {
					currentStory.Priority = priority
				}
				continue
			}

			// Parse depends_on
			if matches := dependsOnRegex.FindStringSubmatch(trimmedLine); len(matches) >= 2 {
				deps := parseDependencyList(matches[1])
				currentStory.DependsOn = deps
				continue
			}

			// Parse status
			if matches := statusRegex.FindStringSubmatch(trimmedLine); len(matches) >= 2 {
				status := parseStatus(matches[1])
				currentStory.Status = status
				continue
			}

			// Parse acceptance criteria items
			if inAcceptanceCriteria {
				if strings.HasPrefix(trimmedLine, "- [ ]") || strings.HasPrefix(trimmedLine, "- [x]") ||
					strings.HasPrefix(trimmedLine, "-") || strings.HasPrefix(trimmedLine, "*") {
					criterion := strings.TrimLeft(trimmedLine, "- [x]*")
					criterion = strings.TrimLeft(criterion, "] ")
					criterion = strings.TrimSpace(criterion)
					if criterion != "" {
						currentStory.AcceptanceCriteria = append(currentStory.AcceptanceCriteria, criterion)
					}
				}
				continue
			}

			// Collect description lines
			if inDescription || (currentSection == "" && trimmedLine != "" && !strings.HasPrefix(trimmedLine, "**")) {
				if trimmedLine != "" {
					descriptionLines = append(descriptionLines, trimmedLine)
				}
				continue
			}

			// Collect notes
			if currentSection == "notes" && trimmedLine != "" {
				currentStory.Notes = strings.TrimSpace(currentStory.Notes + "\n" + trimmedLine)
				continue
			}
		}

		// Check for overview/description section (before stories)
		if !inStory && strings.HasPrefix(trimmedLine, "## Overview") {
			currentSection = "overview"
			continue
		}

		// Collect project description
		if !inStory && currentSection == "overview" && trimmedLine != "" && !strings.HasPrefix(trimmedLine, "#") {
			project.Description = strings.TrimSpace(project.Description + "\n" + trimmedLine)
		}
	}

	// Save the last story
	if currentStory != nil {
		if len(descriptionLines) > 0 {
			currentStory.Description = strings.TrimSpace(strings.Join(descriptionLines, "\n"))
		}
		project.AddStory(currentStory)
	}

	if err := scanner.Err(); err != nil {
		return nil, domain.ErrPRDInvalid("error reading file", err)
	}

	// Initialize blocked status based on dependencies
	project.UpdateBlockedStatus()

	return project, nil
}

// Validate validates a project's structure and dependencies
func (p *MarkdownPRDParser) Validate(project *domain.Project) error {
	if project == nil {
		return domain.ErrPRDInvalid("project is nil", nil)
	}

	if len(project.Stories) == 0 {
		return domain.ErrPRDInvalid("no stories found in PRD", nil)
	}

	// Check for duplicate story IDs
	seenIDs := make(map[string]bool)
	for _, story := range project.Stories {
		if seenIDs[story.ID] {
			return domain.ErrPRDInvalid("duplicate story ID: "+story.ID, nil)
		}
		seenIDs[story.ID] = true
	}

	// Validate dependencies exist
	if err := project.ValidateDependencies(); err != nil {
		return domain.ErrPRDInvalid(err.Error(), nil)
	}

	// Check for circular dependencies
	if err := project.DetectCircularDependencies(); err != nil {
		return err
	}

	return nil
}

// parseDependencyList parses a comma-separated list of dependency IDs
func parseDependencyList(s string) []string {
	var deps []string
	for _, part := range strings.Split(s, ",") {
		dep := strings.TrimSpace(part)
		dep = strings.Trim(dep, "\"'")
		if dep != "" {
			deps = append(deps, dep)
		}
	}
	return deps
}

// parseStatus converts a status string to StoryStatus
func parseStatus(s string) domain.StoryStatus {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "completed", "done", "complete":
		return domain.StoryStatusCompleted
	case "running", "in_progress", "inprogress", "in-progress":
		return domain.StoryStatusRunning
	case "blocked":
		return domain.StoryStatusBlocked
	case "failed", "error":
		return domain.StoryStatusFailed
	default:
		return domain.StoryStatusPending
	}
}


package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/DylanSharp/dtools/internal/ralph/domain"
)

// StatusBar displays project execution progress
type StatusBar struct {
	ProjectName      string
	TotalStories     int
	CompletedStories int
	PendingStories   int
	BlockedStories   int
	FailedStories    int
	RunningStories   int
	CurrentStory     string
	CurrentStoryID   string
	Status           domain.ProjectStatus
	StartTime        time.Time
	Error            error
}

// NewStatusBar creates a new status bar
func NewStatusBar() StatusBar {
	return StatusBar{
		StartTime: time.Now(),
	}
}

// Update updates the status bar from a project
func (s *StatusBar) Update(project *domain.Project) {
	if project == nil {
		return
	}

	s.ProjectName = project.Name
	s.TotalStories = project.TotalStories()
	s.CompletedStories = project.CompletedStories()
	s.PendingStories = project.PendingStories()
	s.BlockedStories = project.BlockedStories()
	s.FailedStories = project.FailedStories()
	s.RunningStories = project.RunningStories()
	s.Status = project.Status

	if project.CurrentStory != nil {
		s.CurrentStoryID = *project.CurrentStory
		if story := project.GetStory(*project.CurrentStory); story != nil {
			s.CurrentStory = story.Title
		}
	} else {
		s.CurrentStory = ""
		s.CurrentStoryID = ""
	}

	if project.StartedAt != nil {
		s.StartTime = *project.StartedAt
	}
}

// SetError sets an error state
func (s *StatusBar) SetError(err error) {
	s.Error = err
}

// ClearError clears the error state
func (s *StatusBar) ClearError() {
	s.Error = nil
}

// Render renders the status bar to the given width
func (s *StatusBar) Render(width int) string {
	if width < 40 {
		width = 40
	}

	var parts []string

	// Project name
	if s.ProjectName != "" {
		parts = append(parts, titleStyle.Render(s.ProjectName))
	}

	// Separator
	parts = append(parts, mutedStyle.Render("│"))

	// Progress stats
	stats := fmt.Sprintf("Done: %s  Active: %s  Blocked: %s  Left: %s",
		successStyle.Render(fmt.Sprintf("%d", s.CompletedStories)),
		runningStyle.Render(fmt.Sprintf("%d", s.RunningStories)),
		warningStyle.Render(fmt.Sprintf("%d", s.BlockedStories)),
		mutedStyle.Render(fmt.Sprintf("%d", s.TotalStories-s.CompletedStories)),
	)
	parts = append(parts, stats)

	// Separator
	parts = append(parts, mutedStyle.Render("│"))

	// Progress bar
	progressBar := s.renderProgressBar(20)
	parts = append(parts, progressBar)

	// First line
	line1 := strings.Join(parts, " ")

	// Second line - current story and elapsed time
	var line2Parts []string

	if s.CurrentStory != "" {
		storyText := fmt.Sprintf("▶ %s: %s", s.CurrentStoryID, s.CurrentStory)
		// Truncate if too long
		maxLen := width - 25
		if len(storyText) > maxLen && maxLen > 10 {
			storyText = storyText[:maxLen-3] + "..."
		}
		line2Parts = append(line2Parts, runningStyle.Render(storyText))
	} else if s.Status == domain.ProjectStatusCompleted {
		line2Parts = append(line2Parts, successStyle.Render("✓ All stories complete!"))
	} else if s.FailedStories > 0 {
		line2Parts = append(line2Parts, errorStyle.Render(fmt.Sprintf("✗ %d failed stories", s.FailedStories)))
	}

	// Elapsed time
	elapsed := s.formatElapsed()
	if len(line2Parts) > 0 {
		line2Parts = append(line2Parts, mutedStyle.Render("│"))
	}
	line2Parts = append(line2Parts, mutedStyle.Render("Elapsed: "+elapsed))

	line2 := strings.Join(line2Parts, " ")

	// Error line if present
	var lines []string
	if s.Error != nil {
		errorLine := errorStyle.Render("Error: " + s.Error.Error())
		lines = append(lines, errorLine)
	}
	lines = append(lines, line1, line2)

	// Border
	border := mutedStyle.Render(strings.Repeat("─", width))
	return border + "\n" + strings.Join(lines, "\n")
}

// renderProgressBar renders a progress bar
func (s *StatusBar) renderProgressBar(width int) string {
	if s.TotalStories == 0 {
		return progressBarEmptyStyle.Render(strings.Repeat("░", width))
	}

	percent := (s.CompletedStories * 100) / s.TotalStories
	filled := (percent * width) / 100
	empty := width - filled

	bar := progressBarFilledStyle.Render(strings.Repeat("█", filled)) +
		progressBarEmptyStyle.Render(strings.Repeat("░", empty))

	return fmt.Sprintf("[%s] %d%%", bar, percent)
}

// formatElapsed formats the elapsed time
func (s *StatusBar) formatElapsed() string {
	elapsed := time.Since(s.StartTime)
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// RenderCompact renders a compact single-line status bar
func (s *StatusBar) RenderCompact(width int) string {
	percent := 0
	if s.TotalStories > 0 {
		percent = (s.CompletedStories * 100) / s.TotalStories
	}

	status := fmt.Sprintf("%d/%d (%d%%)", s.CompletedStories, s.TotalStories, percent)

	if s.CurrentStory != "" {
		maxLen := width - len(status) - 10
		story := s.CurrentStory
		if len(story) > maxLen && maxLen > 10 {
			story = story[:maxLen-3] + "..."
		}
		return fmt.Sprintf("%s │ %s", status, story)
	}

	return status
}

// RenderStatusLine renders just the status line (for non-TUI mode)
func (s *StatusBar) RenderStatusLine() string {
	var parts []string

	// Status icon and project
	switch s.Status {
	case domain.ProjectStatusRunning:
		parts = append(parts, runningStyle.Render("▶"))
	case domain.ProjectStatusCompleted:
		parts = append(parts, successStyle.Render("✓"))
	case domain.ProjectStatusFailed:
		parts = append(parts, errorStyle.Render("✗"))
	default:
		parts = append(parts, mutedStyle.Render("○"))
	}

	if s.ProjectName != "" {
		parts = append(parts, s.ProjectName)
	}

	// Progress
	parts = append(parts, fmt.Sprintf("[%d/%d]", s.CompletedStories, s.TotalStories))

	// Current story
	if s.CurrentStory != "" {
		parts = append(parts, "→", s.CurrentStory)
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(parts, " "))
}

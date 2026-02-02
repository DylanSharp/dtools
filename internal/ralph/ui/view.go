package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/DylanSharp/dtools/internal/ralph/domain"
)

const (
	statusBarHeight   = 3
	helpHeight        = 1
	headerHeight      = 4
	minViewportHeight = 5
)

// RenderView renders the complete TUI view
func RenderView(m *Model) string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var sections []string

	// Header
	header := renderHeader(m)
	sections = append(sections, header)

	// Calculate viewport height
	viewportHeight := m.height - headerHeight - statusBarHeight - helpHeight - 2
	if viewportHeight < minViewportHeight {
		viewportHeight = minViewportHeight
	}

	// Main content - events/thoughts
	content := renderEventList(m, viewportHeight)
	sections = append(sections, content)

	// Help line
	help := renderHelp(m)
	sections = append(sections, help)

	// Status bar
	statusBar := m.statusBar.Render(m.width)
	sections = append(sections, statusBar)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header section
func renderHeader(m *Model) string {
	if m.project == nil {
		return headerStyle.Width(m.width).Render("Ralph - PRD Agent Loop")
	}

	title := titleStyle.Render(fmt.Sprintf("Ralph - %s", m.project.Name))

	var stats []string
	stats = append(stats, fmt.Sprintf("Stories: %d/%d", m.project.CompletedStories(), m.project.TotalStories()))
	stats = append(stats, fmt.Sprintf("Progress: %d%%", m.project.Progress()))

	statsLine := mutedStyle.Render(strings.Join(stats, " │ "))

	return headerStyle.Width(m.width).Render(title + "\n" + statsLine)
}

// renderEventList renders the scrollable event list
func renderEventList(m *Model, height int) string {
	if len(m.events) == 0 {
		if m.streaming {
			return mutedStyle.Render("Waiting for Claude...")
		}
		return mutedStyle.Render("No events yet. Run a project to see progress.")
	}

	var lines []string
	for _, event := range m.events {
		line := renderEvent(event, m.width)
		lines = append(lines, line)
	}

	// Apply scroll offset
	start := m.scrollOffset
	if start < 0 {
		start = 0
	}
	if start >= len(lines) {
		start = len(lines) - 1
		if start < 0 {
			start = 0
		}
	}

	end := start + height
	if end > len(lines) {
		end = len(lines)
	}

	visibleLines := lines[start:end]

	// Pad with empty lines if needed
	for len(visibleLines) < height {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n")
}

// renderEvent renders a single event
func renderEvent(event domain.ExecutionEvent, width int) string {
	switch event.Type {
	case domain.EventTypeProjectStarted:
		return successStyle.Render("▶ Project started: " + event.Content)

	case domain.EventTypeProjectComplete:
		return successStyle.Render("✓ Project complete: " + event.Content)

	case domain.EventTypeProjectFailed:
		return errorStyle.Render("✗ Project failed: " + event.Content)

	case domain.EventTypeStoryStarted:
		return highlightStyle.Render(fmt.Sprintf("━━━ Starting: [%s] %s ━━━", event.StoryID, event.Content))

	case domain.EventTypeStoryCompleted:
		return successStyle.Render(fmt.Sprintf("✓ Completed: [%s] %s", event.StoryID, event.Content))

	case domain.EventTypeStoryFailed:
		return errorStyle.Render(fmt.Sprintf("✗ Failed: [%s] %s", event.StoryID, event.Content))

	case domain.EventTypeThought:
		return renderThought(event, width)

	case domain.EventTypeError:
		return errorStyle.Render("Error: " + event.Content)

	default:
		return event.Content
	}
}

// renderThought renders a thought event with appropriate styling
func renderThought(event domain.ExecutionEvent, width int) string {
	style := GetThoughtStyle(string(event.ThoughtType))

	// Truncate long content
	content := event.Content
	maxLen := width - 4
	if maxLen > 0 && len(content) > maxLen {
		content = content[:maxLen-3] + "..."
	}

	// Add file context if present
	if event.File != "" {
		fileRef := mutedStyle.Render(fmt.Sprintf("[%s]", event.File))
		return style.Render(content) + " " + fileRef
	}

	return style.Render(content)
}

// renderHelp renders the help line
func renderHelp(m *Model) string {
	var keys []string

	if m.streaming {
		keys = append(keys, "streaming...")
	}

	keys = append(keys,
		"q: quit",
		"↑/↓: scroll",
		"g/G: top/bottom",
	)

	if !m.streaming && m.project != nil && !m.project.IsComplete() {
		keys = append(keys, "r: restart")
	}

	return helpStyle.Render(strings.Join(keys, " │ "))
}

// RenderError renders an error dialog
func RenderError(err error, width int) string {
	if width < 40 {
		width = 40
	}

	title := errorStyle.Render("Error")
	message := err.Error()

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", message)
	return boxStyle.Width(width - 4).Render(content)
}

// RenderStoryList renders a list of stories with their status
func RenderStoryList(project *domain.Project, currentID string, width int) string {
	if project == nil || len(project.Stories) == 0 {
		return mutedStyle.Render("No stories")
	}

	var lines []string
	for _, story := range project.Stories {
		icon := GetStatusIcon(string(story.Status))
		style := GetStoryStatusStyle(string(story.Status))

		isCurrent := currentID != "" && story.ID == currentID
		prefix := "  "
		if isCurrent {
			prefix = "▶ "
		}

		line := fmt.Sprintf("%s%s %s: %s", prefix, icon, story.ID, story.Title)

		// Add dependency info for blocked stories
		if story.IsBlocked() && len(story.DependsOn) > 0 {
			deps := strings.Join(story.DependsOn, ", ")
			line += mutedStyle.Render(fmt.Sprintf(" (waiting: %s)", deps))
		}

		// Truncate if needed
		maxLen := width - 2
		if maxLen > 0 && len(line) > maxLen {
			line = line[:maxLen-3] + "..."
		}

		lines = append(lines, style.Render(line))
	}

	return strings.Join(lines, "\n")
}

// RenderProgressSummary renders a progress summary
func RenderProgressSummary(project *domain.Project) string {
	if project == nil {
		return ""
	}

	var parts []string

	parts = append(parts, fmt.Sprintf("Total: %d", project.TotalStories()))
	parts = append(parts, successStyle.Render(fmt.Sprintf("Done: %d", project.CompletedStories())))

	if project.RunningStories() > 0 {
		parts = append(parts, runningStyle.Render(fmt.Sprintf("Running: %d", project.RunningStories())))
	}
	if project.BlockedStories() > 0 {
		parts = append(parts, warningStyle.Render(fmt.Sprintf("Blocked: %d", project.BlockedStories())))
	}
	if project.FailedStories() > 0 {
		parts = append(parts, errorStyle.Render(fmt.Sprintf("Failed: %d", project.FailedStories())))
	}

	return strings.Join(parts, " │ ")
}

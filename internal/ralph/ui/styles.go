package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	colorPrimary   = lipgloss.Color("39")  // Cyan
	colorSecondary = lipgloss.Color("141") // Purple
	colorSuccess   = lipgloss.Color("82")  // Green
	colorWarning   = lipgloss.Color("214") // Yellow/Orange
	colorError     = lipgloss.Color("196") // Red
	colorMuted     = lipgloss.Color("245") // Gray
	colorHighlight = lipgloss.Color("51")  // Bright cyan
)

// Text styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	highlightStyle = lipgloss.NewStyle().
			Foreground(colorHighlight).
			Bold(true)
)

// Story status styles
var (
	pendingStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	blockedStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	runningStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	completedStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	failedStyle = lipgloss.NewStyle().
			Foreground(colorError)
)

// Thought type styles
var (
	thoughtAnalysisStyle = lipgloss.NewStyle().
				Foreground(colorSecondary)

	thoughtProgressStyle = lipgloss.NewStyle().
				Foreground(colorPrimary)

	thoughtSuggestionStyle = lipgloss.NewStyle().
				Foreground(colorWarning)

	thoughtCodeStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	thoughtGeneralStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
)

// Box styles
var (
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorMuted).
			MarginBottom(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

// Progress bar styles
var (
	progressBarFilledStyle = lipgloss.NewStyle().
				Foreground(colorSuccess)

	progressBarEmptyStyle = lipgloss.NewStyle().
				Foreground(colorMuted)
)

// GetStoryStatusStyle returns the appropriate style for a story status
func GetStoryStatusStyle(status string) lipgloss.Style {
	switch status {
	case "pending":
		return pendingStyle
	case "blocked":
		return blockedStyle
	case "running":
		return runningStyle
	case "completed":
		return completedStyle
	case "failed":
		return failedStyle
	default:
		return mutedStyle
	}
}

// GetThoughtStyle returns the appropriate style for a thought type
func GetThoughtStyle(thoughtType string) lipgloss.Style {
	switch thoughtType {
	case "analysis":
		return thoughtAnalysisStyle
	case "progress":
		return thoughtProgressStyle
	case "suggestion":
		return thoughtSuggestionStyle
	case "code":
		return thoughtCodeStyle
	default:
		return thoughtGeneralStyle
	}
}

// GetStatusIcon returns an icon for a story status
func GetStatusIcon(status string) string {
	switch status {
	case "pending":
		return "○"
	case "blocked":
		return "⏸"
	case "running":
		return "▶"
	case "completed":
		return "✓"
	case "failed":
		return "✗"
	default:
		return "?"
	}
}

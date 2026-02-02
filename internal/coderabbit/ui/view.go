package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
)

const (
	statusBarHeight = 1
	helpHeight      = 1
	headerHeight    = 2
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

	// Main content (thoughts viewport)
	viewportHeight := m.height - statusBarHeight - helpHeight - headerHeight
	if viewportHeight < minViewportHeight {
		viewportHeight = minViewportHeight
	}

	content := renderThoughts(m.thoughts, m.width, viewportHeight, m.scrollOffset, m.streaming, m.satisfied)
	sections = append(sections, content)

	// Help line
	help := renderHelp(m)
	sections = append(sections, help)

	// Status bar
	statusBar := m.statusBar.Render(m.width)
	sections = append(sections, statusBar)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the top header
func renderHeader(m *Model) string {
	title := "Claude Code Review"
	if m.review != nil && m.review.Title != "" {
		title = fmt.Sprintf("Review: %s", m.review.Title)
		if len(title) > m.width-4 {
			title = title[:m.width-7] + "..."
		}
	}

	var subtitle string
	if m.review != nil {
		subtitle = fmt.Sprintf("PR #%d on %s", m.review.PRNumber, m.review.Branch)
	}

	header := HeaderStyle.Width(m.width).Render(title)
	if subtitle != "" {
		subtitleLine := DimStyle.Render(subtitle)
		header = lipgloss.JoinVertical(lipgloss.Left, header, subtitleLine)
	}

	return header
}

// renderThoughts renders the scrollable thoughts area
func renderThoughts(thoughts []domain.ThoughtChunk, width, height, scrollOffset int, streaming, satisfied bool) string {
	if len(thoughts) == 0 {
		var message string
		if satisfied {
			message = "✓ No outstanding comments - CodeRabbit is satisfied!"
		} else if streaming {
			message = "Waiting for Claude's response..."
		} else {
			message = "Fetching CodeRabbit review..."
		}
		placeholder := DimStyle.Render(message)
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, placeholder)
	}

	// Render each thought
	var lines []string
	for _, thought := range thoughts {
		line := renderThought(thought, width-4)
		lines = append(lines, line)
	}

	// Join all lines
	content := strings.Join(lines, "\n")
	allLines := strings.Split(content, "\n")

	// Apply scroll offset
	totalLines := len(allLines)
	if scrollOffset > totalLines-height {
		scrollOffset = totalLines - height
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Get visible lines
	endLine := scrollOffset + height
	if endLine > totalLines {
		endLine = totalLines
	}

	visibleLines := allLines[scrollOffset:endLine]

	// Pad to fill viewport if needed
	for len(visibleLines) < height {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n")
}

// renderThought renders a single thought chunk
func renderThought(thought domain.ThoughtChunk, maxWidth int) string {
	// Choose style based on thought type
	var style lipgloss.Style
	var bullet string

	switch thought.Type {
	case domain.ThoughtTypeProgress:
		style = ThoughtProgressStyle
		bullet = "●"
	case domain.ThoughtTypeSuggestion:
		style = ThoughtSuggestionStyle
		bullet = "◆"
	case domain.ThoughtTypeAnalysis:
		style = ThoughtAnalysisStyle
		bullet = "▸"
	default:
		style = ThoughtStyle
		bullet = "·"
	}

	// Build the line
	bulletStyled := ThoughtBulletStyle.Render(bullet)

	// Add file reference if available
	content := thought.Content
	if thought.File != "" {
		fileRef := FileReferenceStyle.Render(fmt.Sprintf("[%s]", thought.File))
		content = fileRef + " " + content
	}

	// Word wrap if too long
	if len(content) > maxWidth-4 {
		content = wordWrap(content, maxWidth-4)
	}

	return bulletStyled + " " + style.Render(content)
}

// renderHelp renders the help line
func renderHelp(m *Model) string {
	var bindings []string

	if m.watchMode {
		if m.confirmingExit {
			bindings = append(bindings,
				HelpKeyStyle.Render("y")+" "+HelpDescStyle.Render("confirm"),
				HelpKeyStyle.Render("n")+" "+HelpDescStyle.Render("continue watching"),
			)
		} else {
			bindings = append(bindings,
				HelpKeyStyle.Render("q")+" "+HelpDescStyle.Render("quit"),
				HelpKeyStyle.Render("↑/↓")+" "+HelpDescStyle.Render("scroll"),
				HelpKeyStyle.Render("o")+" "+HelpDescStyle.Render("open PR"),
			)
		}
	} else {
		bindings = append(bindings,
			HelpKeyStyle.Render("q")+" "+HelpDescStyle.Render("quit"),
			HelpKeyStyle.Render("↑/↓")+" "+HelpDescStyle.Render("scroll"),
			HelpKeyStyle.Render("r")+" "+HelpDescStyle.Render("refresh"),
			HelpKeyStyle.Render("o")+" "+HelpDescStyle.Render("open PR"),
		)
	}

	help := strings.Join(bindings, "  ")
	return HelpStyle.Render(help)
}

// wordWrap wraps text to fit within maxWidth
func wordWrap(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLength := 0

	for i, word := range words {
		wordLen := len(word)

		if lineLength+wordLen+1 > maxWidth && lineLength > 0 {
			result.WriteString("\n  ")
			lineLength = 2
		} else if i > 0 {
			result.WriteString(" ")
			lineLength++
		}

		result.WriteString(word)
		lineLength += wordLen
	}

	return result.String()
}

// RenderConfirmDialog renders the manual confirmation dialog
func RenderConfirmDialog(width int) string {
	message := `
CodeRabbit appears satisfied with the changes.

Do you want to exit watch mode?

  [y] Yes, exit watch mode
  [n] No, continue watching

`
	box := ActiveBorderStyle.
		Width(width - 4).
		Render(SuccessStyle.Render(message))

	return lipgloss.Place(width, 10, lipgloss.Center, lipgloss.Center, box)
}

// RenderError renders an error message
func RenderError(err error, width int) string {
	message := fmt.Sprintf("Error: %v\n\nPress any key to continue...", err)
	box := BorderStyle.
		BorderForeground(Red).
		Width(width - 4).
		Render(ErrorStyle.Render(message))

	return lipgloss.Place(width, 10, lipgloss.Center, lipgloss.Center, box)
}

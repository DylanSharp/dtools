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

	// Build view state
	viewState := ThoughtViewState{
		Streaming: m.streaming,
		Fetching:  m.fetching,
		Complete:  m.complete,
		Satisfied: m.satisfied,
		WatchMode: m.watchMode,
	}
	if m.review != nil {
		viewState.TotalFound = m.review.TotalFoundCount
		viewState.AlreadyAddressed = m.review.AlreadyAddressed
		viewState.NewComments = m.review.NewCommentsCount
		viewState.CIFailureCount = len(m.review.CIFailures)
		viewState.CIPendingCount = m.review.CIPendingCount
		viewState.CIAllComplete = m.review.CIAllComplete
		// Check if CodeRabbit is one of the pending CI checks
		for _, name := range m.review.CIPendingNames {
			if strings.Contains(strings.ToLower(name), "coderabbit") {
				viewState.CodeRabbitPending = true
				break
			}
		}
	}

	content := renderThoughts(m.thoughts, m.width, viewportHeight, m.scrollOffset, viewState)
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

// ThoughtViewState holds the state needed for rendering thoughts
type ThoughtViewState struct {
	Streaming          bool
	Fetching           bool
	Complete           bool
	Satisfied          bool
	WatchMode          bool
	TotalFound         int
	AlreadyAddressed   int
	NewComments        int
	CIFailureCount     int
	CIPendingCount     int
	CIAllComplete      bool
	CodeRabbitPending  bool // True if CodeRabbit review check is still running
}

// renderThoughts renders the scrollable thoughts area
func renderThoughts(thoughts []domain.ThoughtChunk, width, height, scrollOffset int, state ThoughtViewState) string {
	if len(thoughts) == 0 {
		var message string
		if state.Satisfied && state.TotalFound == 0 && state.CIFailureCount == 0 && state.CIPendingCount == 0 {
			// All clear: no comments, no CI failures, no pending CI
			if state.WatchMode {
				message = "✓ No CodeRabbit comments found\n✓ All CI checks passed\n\nWaiting for new comments..."
			} else {
				message = "✓ No CodeRabbit comments found\n✓ All CI checks passed\n\nPR looks good!"
			}
		} else if state.CIPendingCount > 0 && state.CIFailureCount == 0 && state.CodeRabbitPending {
			// CodeRabbit is still reviewing - be specific about this
			message = "◐ CodeRabbit is reviewing the PR...\n\nWaiting for CodeRabbit to finish..."
		} else if state.CIPendingCount > 0 && state.CIFailureCount == 0 {
			// Other CI checks still running (not CodeRabbit)
			message = fmt.Sprintf("◐ %d CI check(s) still running\n\nWaiting for CI to complete...", state.CIPendingCount)
		} else if state.Satisfied && state.CIFailureCount > 0 {
			// CodeRabbit is satisfied but CI is failing
			if state.WatchMode {
				message = fmt.Sprintf("✓ CodeRabbit satisfied\n✗ %d CI check(s) failing\n\nWaiting for CI to pass...", state.CIFailureCount)
			} else {
				message = fmt.Sprintf("✓ CodeRabbit satisfied\n✗ %d CI check(s) failing - fix required", state.CIFailureCount)
			}
		} else if state.Complete && state.NewComments == 0 && state.AlreadyAddressed > 0 && state.CIFailureCount == 0 && state.CIPendingCount == 0 {
			// All comments addressed, CI complete and passing
			if state.WatchMode {
				message = fmt.Sprintf("✓ All %d comments already addressed\n✓ All CI checks passed\n\nWaiting for new comments...", state.AlreadyAddressed)
			} else {
				message = fmt.Sprintf("✓ All %d comments already addressed\n✓ All CI checks passed", state.AlreadyAddressed)
			}
		} else if state.Complete && state.NewComments == 0 && state.AlreadyAddressed > 0 && state.CIFailureCount == 0 && state.CIPendingCount > 0 {
			// All comments addressed, but CI still running
			if state.WatchMode {
				message = fmt.Sprintf("✓ All %d comments already addressed\n◐ %d CI check(s) still running\n\nWaiting for CI to complete...", state.AlreadyAddressed, state.CIPendingCount)
			} else {
				message = fmt.Sprintf("✓ All %d comments already addressed\n◐ %d CI check(s) still running\n\nWaiting for CI to complete...", state.AlreadyAddressed, state.CIPendingCount)
			}
		} else if state.Complete && state.NewComments == 0 && state.CIFailureCount > 0 {
			// No new comments but CI is failing
			if state.AlreadyAddressed > 0 {
				message = fmt.Sprintf("✓ All %d comments addressed\n✗ %d CI check(s) failing - fix required", state.AlreadyAddressed, state.CIFailureCount)
			} else {
				message = fmt.Sprintf("✓ No CodeRabbit comments\n✗ %d CI check(s) failing - fix required", state.CIFailureCount)
			}
		} else if state.Complete && state.NewComments == 0 && state.CIPendingCount > 0 {
			// No comments, CI still running
			message = fmt.Sprintf("✓ No CodeRabbit comments\n◐ %d CI check(s) still running\n\nWaiting for CI to complete...", state.CIPendingCount)
		} else if state.Streaming && (state.NewComments > 0 || state.CIFailureCount > 0) {
			var parts []string
			if state.NewComments > 0 {
				parts = append(parts, fmt.Sprintf("%d comments", state.NewComments))
			}
			if state.CIFailureCount > 0 {
				parts = append(parts, fmt.Sprintf("%d CI failures", state.CIFailureCount))
			}
			message = fmt.Sprintf("Found %s, passing to Claude...", strings.Join(parts, " and "))
		} else if state.Streaming {
			message = "Waiting for Claude's response..."
		} else if state.Fetching {
			message = "Checking for CodeRabbit comments and CI status..."
		} else if state.Complete {
			if state.WatchMode {
				message = "✓ Review complete\n\nWaiting for new comments..."
			} else {
				message = "✓ Review complete"
			}
		} else {
			message = "Initializing..."
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
	// Handle header type specially (no bullet, dimmed)
	if thought.Type == domain.ThoughtTypeHeader {
		return DimStyle.Render(thought.Content)
	}

	// Handle comment type (CodeRabbit comments being shown)
	if thought.Type == domain.ThoughtTypeComment {
		// Word wrap if too long
		content := thought.Content
		if len(content) > maxWidth-2 {
			content = wordWrap(content, maxWidth-2)
		}
		return CommentStyle.Render(content)
	}

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

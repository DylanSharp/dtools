package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
	"github.com/DylanSharp/dtools/internal/coderabbit/service"
)

// StatusBar renders the bottom status line
type StatusBar struct {
	Branch            string
	PRNumber          int
	Repository        string
	CommentsProcessed int
	CommentsTotal     int
	CurrentFile       string
	Status            domain.ReviewStatus
	WatchState        service.WatchState
	CooldownRemaining time.Duration
	StartTime         time.Time
	Error             error
}

// NewStatusBar creates a new status bar with default values
func NewStatusBar() StatusBar {
	return StatusBar{
		StartTime: time.Now(),
		Status:    domain.ReviewStatusPending,
	}
}

// Render renders the status bar to fit the given width
func (s StatusBar) Render(width int) string {
	// Build sections
	var sections []string

	// Brand badge
	brand := StatusBarBrandStyle.Render("Review")
	sections = append(sections, brand)

	// Branch and PR
	if s.Branch != "" {
		branchSection := StatusBarSectionStyle.Render(s.Branch)
		sections = append(sections, branchSection)
	}

	if s.PRNumber > 0 {
		prSection := StatusBarSectionStyle.Render(fmt.Sprintf("PR#%d", s.PRNumber))
		sections = append(sections, prSection)
	}

	// Comments progress
	if s.CommentsTotal > 0 {
		progress := fmt.Sprintf("%d/%d", s.CommentsProcessed, s.CommentsTotal)
		progressSection := StatusBarSectionStyle.Render("Comments: " + progress)
		sections = append(sections, progressSection)
	}

	// Current file
	if s.CurrentFile != "" {
		// Truncate if too long
		file := s.CurrentFile
		if len(file) > 30 {
			file = "..." + file[len(file)-27:]
		}
		fileSection := FileReferenceStyle.Render(file)
		sections = append(sections, fileSection)
	}

	// Status indicator
	statusSection := s.renderStatus()
	sections = append(sections, statusSection)

	// Elapsed time
	elapsed := time.Since(s.StartTime)
	elapsedStr := formatDuration(elapsed)
	elapsedSection := DimStyle.Render(elapsedStr)
	sections = append(sections, elapsedSection)

	// Join sections with dividers
	divider := StatusBarDividerStyle.Render(" │ ")
	content := strings.Join(sections, divider)

	// Pad to full width
	contentWidth := lipgloss.Width(content)
	if contentWidth < width {
		padding := strings.Repeat(" ", width-contentWidth)
		content = content + StatusBarStyle.Render(padding)
	}

	return StatusBarStyle.Width(width).Render(content)
}

// renderStatus renders the current status with appropriate styling
func (s StatusBar) renderStatus() string {
	// Handle error state
	if s.Error != nil {
		return StatusBarErrorStyle.Render("● Error")
	}

	// Handle watch mode states
	if s.WatchState != "" {
		switch s.WatchState {
		case service.WatchStatePolling:
			return StatusBarSectionStyle.Render("◌ Polling...")
		case service.WatchStateBatchWait:
			return StatusBarWarningStyle.Render("◐ Batching...")
		case service.WatchStateProcessing:
			return StatusBarProgressStyle.Render("● Processing")
		case service.WatchStateCooldown:
			remaining := formatDuration(s.CooldownRemaining)
			return StatusBarWarningStyle.Render(fmt.Sprintf("◑ Cooldown %s", remaining))
		case service.WatchStateSatisfied:
			return StatusBarProgressStyle.Render("✓ Satisfied")
		case service.WatchStateError:
			return StatusBarErrorStyle.Render("● Error")
		}
	}

	// Handle review status
	switch s.Status {
	case domain.ReviewStatusPending:
		return StatusBarSectionStyle.Render("○ Pending")
	case domain.ReviewStatusFetching:
		return StatusBarSectionStyle.Render("◌ Fetching...")
	case domain.ReviewStatusReviewing:
		return StatusBarProgressStyle.Render("● Reviewing")
	case domain.ReviewStatusCompleted:
		return StatusBarProgressStyle.Render("✓ Complete")
	case domain.ReviewStatusSatisfied:
		return StatusBarProgressStyle.Render("✓ Satisfied")
	case domain.ReviewStatusFailed:
		return StatusBarErrorStyle.Render("✗ Failed")
	default:
		return StatusBarSectionStyle.Render("○ Unknown")
	}
}

// Update updates the status bar with review information
func (s *StatusBar) Update(review *domain.Review) {
	if review == nil {
		return
	}

	s.Branch = review.Branch
	s.PRNumber = review.PRNumber
	s.Repository = review.Repository
	s.CommentsTotal = len(review.Comments)
	s.CommentsProcessed = review.ProcessedCount
	s.CurrentFile = review.CurrentFile
	s.Status = review.Status
}

// SetWatchState updates the watch mode state
func (s *StatusBar) SetWatchState(state service.WatchState, cooldownRemaining time.Duration) {
	s.WatchState = state
	s.CooldownRemaining = cooldownRemaining
}

// SetError sets the error state
func (s *StatusBar) SetError(err error) {
	s.Error = err
}

// ClearError clears the error state
func (s *StatusBar) ClearError() {
	s.Error = nil
}

// formatDuration formats a duration as HH:MM:SS or MM:SS
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// RenderProgressBar renders a progress bar with the given completion percentage
func RenderProgressBar(completed, total, width int) string {
	if total == 0 {
		empty := strings.Repeat("░", width)
		return ProgressEmptyStyle.Render("[" + empty + "]")
	}

	percent := float64(completed) / float64(total)
	filled := int(percent * float64(width))
	empty := width - filled

	filledStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", empty)

	bar := ProgressFilledStyle.Render(filledStr) + ProgressEmptyStyle.Render(emptyStr)
	percentStr := fmt.Sprintf(" %3d%%", int(percent*100))

	return "[" + bar + "]" + percentStr
}

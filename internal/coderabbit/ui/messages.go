package ui

import (
	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
	"github.com/DylanSharp/dtools/internal/coderabbit/service"
)

// ThoughtMsg carries a new thought chunk from Claude
type ThoughtMsg struct {
	Thought domain.ThoughtChunk
}

// ReviewCompleteMsg signals that the review is complete
type ReviewCompleteMsg struct {
	Review *domain.Review
}

// ReviewStartedMsg signals that a review has started
type ReviewStartedMsg struct {
	Review   *domain.Review
	Thoughts <-chan domain.ThoughtChunk
}

// WatchEventMsg carries watch mode events
type WatchEventMsg struct {
	Event service.WatchEvent
}

// ErrorMsg carries error information
type ErrorMsg struct {
	Err error
}

// StatusUpdateMsg requests a status bar update
type StatusUpdateMsg struct{}

// TickMsg is sent periodically for updates
type TickMsg struct{}

// ManualConfirmMsg is sent when user confirms satisfaction
type ManualConfirmMsg struct {
	Confirmed bool
}

// WindowSizeMsg is sent when the terminal is resized
type WindowSizeMsg struct {
	Width  int
	Height int
}

// QuitMsg signals the program should exit
type QuitMsg struct{}

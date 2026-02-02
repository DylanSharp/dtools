package ui

import (
	"github.com/DylanSharp/dtools/internal/ralph/domain"
)

// ExecutionEventMsg wraps an execution event for Bubbletea
type ExecutionEventMsg struct {
	Event domain.ExecutionEvent
}

// ProjectLoadedMsg indicates the project has been loaded
type ProjectLoadedMsg struct {
	Project *domain.Project
}

// ProjectCompleteMsg indicates the project has completed
type ProjectCompleteMsg struct {
	Project *domain.Project
}

// ErrorMsg wraps an error for Bubbletea
type ErrorMsg struct {
	Err error
}

// TickMsg is sent periodically for UI updates
type TickMsg struct{}

// StreamStartedMsg indicates streaming has started
type StreamStartedMsg struct {
	Events <-chan domain.ExecutionEvent
}

// StreamEndedMsg indicates streaming has ended
type StreamEndedMsg struct{}

package ports

import (
	"context"

	"github.com/DylanSharp/dtools/internal/ralph/domain"
)

// Executor runs stories using an AI agent (Claude)
type Executor interface {
	// Execute runs a story and returns a channel of execution events
	Execute(ctx context.Context, story *domain.Story, execCtx ExecutionContext) (<-chan domain.ExecutionEvent, error)

	// IsAvailable checks if the executor (Claude CLI) is available
	IsAvailable() bool
}

// ExecutionContext provides context for story execution
type ExecutionContext struct {
	// Project is the current project being executed
	Project *domain.Project

	// CompletedStories contains summaries of previously completed stories
	CompletedStories []*domain.Story

	// WorkDir is the working directory for execution
	WorkDir string

	// PRDPath is the path to the PRD file
	PRDPath string

	// AdditionalContext is extra context to include in the prompt
	AdditionalContext string
}

// NewExecutionContext creates a new execution context
func NewExecutionContext(project *domain.Project) ExecutionContext {
	return ExecutionContext{
		Project:          project,
		CompletedStories: project.GetCompletedStories(),
		WorkDir:          project.WorkDir,
		PRDPath:          project.PRDPath,
	}
}

// WithAdditionalContext adds extra context to the execution context
func (c ExecutionContext) WithAdditionalContext(ctx string) ExecutionContext {
	c.AdditionalContext = ctx
	return c
}

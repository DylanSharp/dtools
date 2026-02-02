package domain

import (
	"fmt"
)

// Error codes for ralph domain errors
const (
	ErrCodePRDNotFound         = "prd_not_found"
	ErrCodePRDInvalid          = "prd_invalid"
	ErrCodeProjectNotFound     = "project_not_found"
	ErrCodeStoryNotFound       = "story_not_found"
	ErrCodeCircularDependency  = "circular_dependency"
	ErrCodeInvalidDependency   = "invalid_dependency"
	ErrCodeClaudeNotFound      = "claude_not_found"
	ErrCodeClaudeError         = "claude_error"
	ErrCodeExecutionFailed     = "execution_failed"
	ErrCodeStatePersistence    = "state_persistence"
	ErrCodeNoStoriesReady      = "no_stories_ready"
	ErrCodeAllStoriesCompleted = "all_stories_completed"
)

// RalphError represents a domain-specific error
type RalphError struct {
	Code    string
	Message string
	Cause   error
}

// Error implements the error interface
func (e *RalphError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *RalphError) Unwrap() error {
	return e.Cause
}

// NewError creates a new RalphError
func NewError(code, message string) *RalphError {
	return &RalphError{
		Code:    code,
		Message: message,
	}
}

// WrapError creates a new RalphError that wraps another error
func WrapError(code, message string, cause error) *RalphError {
	return &RalphError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// Pre-defined error constructors

// ErrPRDNotFound returns an error for missing PRD file
func ErrPRDNotFound(path string) *RalphError {
	return NewError(ErrCodePRDNotFound, fmt.Sprintf("PRD file not found: %s", path))
}

// ErrPRDInvalid returns an error for invalid PRD format
func ErrPRDInvalid(reason string, cause error) *RalphError {
	return WrapError(ErrCodePRDInvalid, fmt.Sprintf("invalid PRD format: %s", reason), cause)
}

// ErrProjectNotFound returns an error for missing project
func ErrProjectNotFound(id string) *RalphError {
	return NewError(ErrCodeProjectNotFound, fmt.Sprintf("project not found: %s", id))
}

// ErrStoryNotFound returns an error for missing story
func ErrStoryNotFound(id string) *RalphError {
	return NewError(ErrCodeStoryNotFound, fmt.Sprintf("story not found: %s", id))
}

// ErrCircularDependency returns an error for circular dependencies
func ErrCircularDependency(path []string) *RalphError {
	return NewError(ErrCodeCircularDependency, fmt.Sprintf("circular dependency detected: %v", path))
}

// ErrInvalidDependency returns an error for invalid dependency reference
func ErrInvalidDependency(storyID, depID string) *RalphError {
	return NewError(ErrCodeInvalidDependency, fmt.Sprintf("story %q depends on non-existent story %q", storyID, depID))
}

// ErrClaudeNotFound returns an error when Claude CLI is not found
func ErrClaudeNotFound() *RalphError {
	return NewError(ErrCodeClaudeNotFound, "Claude CLI not found. Please install Claude Code first.")
}

// ErrClaudeError returns an error for Claude execution failures
func ErrClaudeError(message string, cause error) *RalphError {
	return WrapError(ErrCodeClaudeError, message, cause)
}

// ErrExecutionFailed returns an error for story execution failures
func ErrExecutionFailed(storyID, reason string, cause error) *RalphError {
	return WrapError(ErrCodeExecutionFailed, fmt.Sprintf("story %q execution failed: %s", storyID, reason), cause)
}

// ErrStatePersistence returns an error for state save/load failures
func ErrStatePersistence(operation string, cause error) *RalphError {
	return WrapError(ErrCodeStatePersistence, fmt.Sprintf("state %s failed", operation), cause)
}

// ErrNoStoriesReady returns an error when no stories can be executed
func ErrNoStoriesReady() *RalphError {
	return NewError(ErrCodeNoStoriesReady, "no stories are ready to execute (all blocked by dependencies or already completed)")
}

// ErrAllStoriesCompleted returns an error when all stories are done
func ErrAllStoriesCompleted() *RalphError {
	return NewError(ErrCodeAllStoriesCompleted, "all stories have been completed")
}

// IsRalphError checks if an error is a RalphError
func IsRalphError(err error) bool {
	_, ok := err.(*RalphError)
	return ok
}

// GetErrorCode returns the error code if it's a RalphError, or empty string
func GetErrorCode(err error) string {
	if re, ok := err.(*RalphError); ok {
		return re.Code
	}
	return ""
}

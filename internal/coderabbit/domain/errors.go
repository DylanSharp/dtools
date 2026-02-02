package domain

import "fmt"

// ErrorCode represents domain-specific error codes
type ErrorCode string

const (
	ErrCodeGitHubAPI       ErrorCode = "github_api_error"
	ErrCodeGitHubRateLimit ErrorCode = "github_rate_limit"
	ErrCodeGitHubAuth      ErrorCode = "github_auth_error"
	ErrCodePRNotFound      ErrorCode = "pr_not_found"
	ErrCodeClaudeTimeout   ErrorCode = "claude_timeout"
	ErrCodeClaudeError     ErrorCode = "claude_error"
	ErrCodeClaudeNotFound  ErrorCode = "claude_not_found"
	ErrCodeJSONParse       ErrorCode = "json_parse_error"
	ErrCodeStateCorrupt    ErrorCode = "state_corrupt"
	ErrCodeNoComments      ErrorCode = "no_comments"
	ErrCodeInvalidConfig   ErrorCode = "invalid_config"
)

// ReviewError represents a domain-specific error
type ReviewError struct {
	Code      ErrorCode
	Message   string
	Err       error
	Retryable bool
}

// Error implements the error interface
func (e *ReviewError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *ReviewError) Unwrap() error {
	return e.Err
}

// NewError creates a new ReviewError
func NewError(code ErrorCode, message string, err error) *ReviewError {
	return &ReviewError{
		Code:      code,
		Message:   message,
		Err:       err,
		Retryable: isRetryable(code),
	}
}

// isRetryable determines if an error code is retryable
func isRetryable(code ErrorCode) bool {
	switch code {
	case ErrCodeGitHubRateLimit, ErrCodeClaudeTimeout:
		return true
	default:
		return false
	}
}

// ErrGitHubAPI creates a GitHub API error
func ErrGitHubAPI(message string, err error) *ReviewError {
	return NewError(ErrCodeGitHubAPI, message, err)
}

// ErrGitHubRateLimit creates a rate limit error
func ErrGitHubRateLimit(err error) *ReviewError {
	return NewError(ErrCodeGitHubRateLimit, "GitHub API rate limit exceeded", err)
}

// ErrGitHubAuth creates an authentication error
func ErrGitHubAuth(err error) *ReviewError {
	return NewError(ErrCodeGitHubAuth, "GitHub authentication failed", err)
}

// ErrPRNotFound creates a PR not found error
func ErrPRNotFound(prNumber int) *ReviewError {
	return NewError(ErrCodePRNotFound, fmt.Sprintf("PR #%d not found", prNumber), nil)
}

// ErrClaudeTimeout creates a Claude timeout error
func ErrClaudeTimeout(err error) *ReviewError {
	return NewError(ErrCodeClaudeTimeout, "Claude CLI timed out", err)
}

// ErrClaudeError creates a Claude error
func ErrClaudeError(message string, err error) *ReviewError {
	return NewError(ErrCodeClaudeError, message, err)
}

// ErrClaudeNotFound creates a Claude not found error
func ErrClaudeNotFound() *ReviewError {
	return NewError(ErrCodeClaudeNotFound, "Claude CLI not found in PATH", nil)
}

// ErrJSONParse creates a JSON parse error
func ErrJSONParse(message string, err error) *ReviewError {
	return NewError(ErrCodeJSONParse, message, err)
}

// ErrNoComments creates a no comments error
func ErrNoComments() *ReviewError {
	return NewError(ErrCodeNoComments, "No CodeRabbit comments found", nil)
}

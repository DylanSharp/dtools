package domain

import (
	"fmt"
	"time"
)

// Comment represents a CodeRabbit review comment
type Comment struct {
	ID           int
	FilePath     string
	LineNumber   int
	EndLine      int // For multi-line comments
	Body         string
	AIPrompt     string // Extracted "Prompt for AI Agents" section
	ThreadID     string
	Author       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	URL          string
	IsResolved   bool
	IsNit        bool
	IsOutdated   bool
	IsOutsideDiff bool
}

// HasAIPrompt returns true if the comment has an extracted AI prompt
func (c *Comment) HasAIPrompt() bool {
	return c.AIPrompt != ""
}

// EffectiveBody returns AIPrompt if available, otherwise the full body
func (c *Comment) EffectiveBody() string {
	if c.AIPrompt != "" {
		return c.AIPrompt
	}
	return c.Body
}

// Location returns a human-readable location string
func (c *Comment) Location() string {
	if c.FilePath == "" {
		return "GENERAL"
	}
	if c.LineNumber == 0 {
		return c.FilePath
	}
	return fmt.Sprintf("%s:%d", c.FilePath, c.LineNumber)
}

// CITestFailure represents a failed CI test or check
type CITestFailure struct {
	CheckName    string
	JobName      string
	AppName      string
	ErrorMessage string
	Summary      string
	LogURL       string
	Annotations  []CIAnnotation
}

// CIStatus represents the overall status of CI checks
type CIStatus struct {
	Failures     []CITestFailure
	PendingCount int      // Number of checks still running
	PendingNames []string // Names of pending checks
	PassedCount  int      // Number of checks that passed
	TotalCount   int      // Total number of checks
}

// AllComplete returns true if all checks have completed (no pending)
func (s CIStatus) AllComplete() bool {
	return s.PendingCount == 0
}

// AllPassed returns true if all checks completed successfully
func (s CIStatus) AllPassed() bool {
	return s.AllComplete() && len(s.Failures) == 0
}

// CIAnnotation represents a specific failure annotation
type CIAnnotation struct {
	Path       string
	StartLine  int
	EndLine    int
	Title      string
	Message    string
	RawDetails string
}

// ThoughtChunk represents a filtered thought from Claude's response
type ThoughtChunk struct {
	Timestamp time.Time
	Content   string
	Type      ThoughtType
	File      string // Current file being discussed
}

// ThoughtType categorizes Claude's output
type ThoughtType string

const (
	ThoughtTypeThinking    ThoughtType = "thinking"
	ThoughtTypeSuggestion  ThoughtType = "suggestion"
	ThoughtTypeAnalysis    ThoughtType = "analysis"
	ThoughtTypeCode        ThoughtType = "code"
	ThoughtTypeProgress    ThoughtType = "progress"
	ThoughtTypeComment     ThoughtType = "comment"  // CodeRabbit comment being addressed
	ThoughtTypeHeader      ThoughtType = "header"   // Section header
)

// IsDisplayable returns true if this thought should be shown in the TUI
func (t ThoughtChunk) IsDisplayable() bool {
	// Show everything except raw code chunks
	return t.Type != ThoughtTypeCode
}

package domain

import (
	"strconv"
	"time"
)

// EventType represents the type of execution event
type EventType string

const (
	EventTypeProjectStarted  EventType = "project_started"
	EventTypeProjectComplete EventType = "project_complete"
	EventTypeProjectFailed   EventType = "project_failed"
	EventTypeStoryStarted    EventType = "story_started"
	EventTypeStoryProgress   EventType = "story_progress"
	EventTypeStoryCompleted  EventType = "story_completed"
	EventTypeStoryFailed     EventType = "story_failed"
	EventTypeThought         EventType = "thought"
	EventTypeToolUse         EventType = "tool_use"
	EventTypeToolResult      EventType = "tool_result"
	EventTypeError           EventType = "error"
)

// ThoughtType categorizes thoughts for display purposes
type ThoughtType string

const (
	ThoughtTypeAnalysis   ThoughtType = "analysis"
	ThoughtTypeProgress   ThoughtType = "progress"
	ThoughtTypeSuggestion ThoughtType = "suggestion"
	ThoughtTypeCode       ThoughtType = "code"
	ThoughtTypeGeneral    ThoughtType = "general"
)

// ExecutionEvent represents a streaming execution update
type ExecutionEvent struct {
	Timestamp   time.Time         `json:"timestamp"`
	StoryID     string            `json:"story_id,omitempty"`
	Type        EventType         `json:"type"`
	ThoughtType ThoughtType       `json:"thought_type,omitempty"`
	Content     string            `json:"content"`
	File        string            `json:"file,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// NewExecutionEvent creates a new execution event
func NewExecutionEvent(eventType EventType, storyID, content string) ExecutionEvent {
	return ExecutionEvent{
		Timestamp: time.Now(),
		StoryID:   storyID,
		Type:      eventType,
		Content:   content,
		Metadata:  make(map[string]string),
	}
}

// NewThoughtEvent creates a new thought event
func NewThoughtEvent(storyID, content string, thoughtType ThoughtType) ExecutionEvent {
	return ExecutionEvent{
		Timestamp:   time.Now(),
		StoryID:     storyID,
		Type:        EventTypeThought,
		ThoughtType: thoughtType,
		Content:     content,
		Metadata:    make(map[string]string),
	}
}

// NewStoryStartedEvent creates a story started event
func NewStoryStartedEvent(story *Story) ExecutionEvent {
	return ExecutionEvent{
		Timestamp: time.Now(),
		StoryID:   story.ID,
		Type:      EventTypeStoryStarted,
		Content:   story.Title,
		Metadata: map[string]string{
			"priority": strconv.Itoa(story.Priority),
			"attempt":  strconv.Itoa(story.Attempts),
		},
	}
}

// NewStoryCompletedEvent creates a story completed event
func NewStoryCompletedEvent(story *Story) ExecutionEvent {
	event := ExecutionEvent{
		Timestamp: time.Now(),
		StoryID:   story.ID,
		Type:      EventTypeStoryCompleted,
		Content:   story.Title,
		Metadata:  make(map[string]string),
	}
	if story.Duration() > 0 {
		event.Metadata["duration"] = story.Duration().String()
	}
	return event
}

// NewStoryFailedEvent creates a story failed event
func NewStoryFailedEvent(story *Story, err string) ExecutionEvent {
	return ExecutionEvent{
		Timestamp: time.Now(),
		StoryID:   story.ID,
		Type:      EventTypeStoryFailed,
		Content:   err,
		Metadata: map[string]string{
			"title":   story.Title,
			"attempt": strconv.Itoa(story.Attempts),
		},
	}
}

// NewProjectStartedEvent creates a project started event
func NewProjectStartedEvent(project *Project) ExecutionEvent {
	return ExecutionEvent{
		Timestamp: time.Now(),
		Type:      EventTypeProjectStarted,
		Content:   project.Name,
		Metadata: map[string]string{
			"total_stories": strconv.Itoa(project.TotalStories()),
		},
	}
}

// NewProjectCompleteEvent creates a project completed event
func NewProjectCompleteEvent(project *Project) ExecutionEvent {
	event := ExecutionEvent{
		Timestamp: time.Now(),
		Type:      EventTypeProjectComplete,
		Content:   project.Name,
		Metadata: map[string]string{
			"completed": strconv.Itoa(project.CompletedStories()),
			"total":     strconv.Itoa(project.TotalStories()),
		},
	}
	if project.Duration() > 0 {
		event.Metadata["duration"] = project.Duration().String()
	}
	return event
}

// NewErrorEvent creates an error event
func NewErrorEvent(storyID, err string) ExecutionEvent {
	return ExecutionEvent{
		Timestamp: time.Now(),
		StoryID:   storyID,
		Type:      EventTypeError,
		Content:   err,
	}
}

// IsStoryEvent returns true if this event is related to story execution
func (e ExecutionEvent) IsStoryEvent() bool {
	switch e.Type {
	case EventTypeStoryStarted, EventTypeStoryProgress,
		EventTypeStoryCompleted, EventTypeStoryFailed:
		return true
	}
	return false
}

// IsProjectEvent returns true if this event is a project-level event
func (e ExecutionEvent) IsProjectEvent() bool {
	switch e.Type {
	case EventTypeProjectStarted, EventTypeProjectComplete, EventTypeProjectFailed:
		return true
	}
	return false
}

// IsThought returns true if this event is a thought
func (e ExecutionEvent) IsThought() bool {
	return e.Type == EventTypeThought
}

// IsError returns true if this event is an error
func (e ExecutionEvent) IsError() bool {
	return e.Type == EventTypeError || e.Type == EventTypeStoryFailed || e.Type == EventTypeProjectFailed
}

// WithFile adds file context to the event
func (e ExecutionEvent) WithFile(file string) ExecutionEvent {
	e.File = file
	return e
}

// WithMetadata adds metadata to the event
func (e ExecutionEvent) WithMetadata(key, value string) ExecutionEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]string)
	}
	e.Metadata[key] = value
	return e
}

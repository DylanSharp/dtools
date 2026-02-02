package ports

import (
	"context"
)

// AIProvider abstracts AI-powered review generation
type AIProvider interface {
	// StreamReview starts a review and returns a channel of stream chunks
	StreamReview(ctx context.Context, prompt string) (<-chan StreamChunk, error)

	// IsAvailable checks if the AI provider (Claude CLI) is available
	IsAvailable() bool
}

// StreamChunk represents a chunk of streaming output from Claude Code CLI
// Format: {"type":"system|assistant|result", ...}
type StreamChunk struct {
	// Type: "system", "assistant", "result", "user"
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`

	// For assistant messages
	Message *AssistantMessage `json:"message,omitempty"`

	// For result messages
	Result  string `json:"result,omitempty"`
	IsError bool   `json:"is_error,omitempty"`

	// Error info
	Error *StreamError `json:"error,omitempty"`
}

// AssistantMessage represents Claude's response
type AssistantMessage struct {
	ID      string           `json:"id"`
	Type    string           `json:"type"`
	Role    string           `json:"role"`
	Content []ContentBlock   `json:"content"`
	Usage   *TokenUsage      `json:"usage,omitempty"`
}

// ContentBlock represents a content block in the message
type ContentBlock struct {
	Type     string `json:"type"` // "text" or "thinking"
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamError represents an error in the stream
type StreamError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GetText extracts the text content from the chunk
func (c StreamChunk) GetText() string {
	// For assistant messages, extract text from content blocks
	if c.Type == "assistant" && c.Message != nil {
		var text string
		for _, block := range c.Message.Content {
			if block.Type == "text" && block.Text != "" {
				text += block.Text
			}
			if block.Type == "thinking" && block.Thinking != "" {
				text += block.Thinking
			}
		}
		return text
	}

	// For result messages, return the result
	if c.Type == "result" && c.Result != "" {
		return c.Result
	}

	return ""
}

// IsComplete returns true if this is the final result chunk
func (c StreamChunk) IsComplete() bool {
	return c.Type == "result"
}

// IsStreamError returns true if this chunk represents an error
func (c StreamChunk) IsStreamError() bool {
	return c.IsError || c.Error != nil
}

package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"

	"github.com/DylanSharp/dtools/internal/ralph/domain"
	"github.com/DylanSharp/dtools/internal/ralph/ports"
)

// ClaudeExecutor implements ports.Executor using the Claude CLI
type ClaudeExecutor struct {
	binaryPath    string
	promptBuilder *PromptBuilder
}

// NewClaudeExecutor creates a new Claude executor
func NewClaudeExecutor() *ClaudeExecutor {
	return &ClaudeExecutor{
		binaryPath:    "claude",
		promptBuilder: NewPromptBuilder(),
	}
}

// NewClaudeExecutorWithPath creates a new executor with a custom binary path
func NewClaudeExecutorWithPath(binaryPath string) *ClaudeExecutor {
	return &ClaudeExecutor{
		binaryPath:    binaryPath,
		promptBuilder: NewPromptBuilder(),
	}
}

// IsAvailable checks if the Claude CLI is available
func (e *ClaudeExecutor) IsAvailable() bool {
	_, err := exec.LookPath(e.binaryPath)
	return err == nil
}

// Execute runs a story and returns a channel of execution events
func (e *ClaudeExecutor) Execute(ctx context.Context, story *domain.Story, execCtx ports.ExecutionContext) (<-chan domain.ExecutionEvent, error) {
	if !e.IsAvailable() {
		return nil, domain.ErrClaudeNotFound()
	}

	// Build the prompt
	prompt := e.promptBuilder.BuildStoryPrompt(story, execCtx)

	// Build the Claude command with streaming JSON output
	cmd := exec.CommandContext(ctx, e.binaryPath,
		"-p",
		"--dangerously-skip-permissions",
		"--output-format", "stream-json",
		"--",
		prompt,
	)

	// Set working directory
	if execCtx.WorkDir != "" {
		cmd.Dir = execCtx.WorkDir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, domain.ErrClaudeError("failed to create stdout pipe", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, domain.ErrClaudeError("failed to create stderr pipe", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, domain.ErrClaudeError("failed to start Claude CLI", err)
	}

	events := make(chan domain.ExecutionEvent, 100)

	// Read stderr in background for error messages
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				_ = scanner.Text()
			}
		}
	}()

	// Read JSONL from stdout and convert to events
	go func() {
		defer close(events)

		// Send story started event
		events <- domain.NewStoryStartedEvent(story)

		scanner := bufio.NewScanner(stdout)
		// Increase buffer size for potentially large JSON objects
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		parser := NewStreamParser()

		for scanner.Scan() {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				// Kill the process and clean up
				cmd.Process.Kill()
				<-stderrDone // Wait for stderr goroutine
				cmd.Wait()
				events <- domain.NewErrorEvent(story.ID, "execution cancelled")
				return
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			// Parse the stream chunk
			event := parser.ParseChunk(line, story.ID)
			if event != nil {
				events <- *event
			}
		}

		// Wait for stderr goroutine
		<-stderrDone

		// Always wait for the command to finish
		cmdErr := cmd.Wait()

		if err := scanner.Err(); err != nil {
			events <- domain.NewErrorEvent(story.ID, err.Error())
		} else if cmdErr != nil {
			events <- domain.NewErrorEvent(story.ID, "command failed: "+cmdErr.Error())
		}

		// Send story completed event
		events <- domain.NewStoryCompletedEvent(story)
	}()

	return events, nil
}

// PromptBuilder constructs Claude prompts from stories
type PromptBuilder struct{}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// BuildStoryPrompt builds a prompt for story execution
func (b *PromptBuilder) BuildStoryPrompt(story *domain.Story, execCtx ports.ExecutionContext) string {
	var sb strings.Builder

	sb.WriteString("You are implementing a story from a Product Requirements Document.\n\n")

	// Project context
	if execCtx.Project != nil {
		sb.WriteString("## Project: ")
		sb.WriteString(execCtx.Project.Name)
		sb.WriteString("\n\n")

		if execCtx.Project.Description != "" {
			sb.WriteString("### Overview\n")
			sb.WriteString(execCtx.Project.Description)
			sb.WriteString("\n\n")
		}
	}

	// Story details
	sb.WriteString("## Current Story\n\n")
	sb.WriteString("**ID:** ")
	sb.WriteString(story.ID)
	sb.WriteString("\n")
	sb.WriteString("**Title:** ")
	sb.WriteString(story.Title)
	sb.WriteString("\n")
	sb.WriteString("**Priority:** ")
	sb.WriteString(strings.Repeat("!", story.Priority))
	sb.WriteString("\n\n")

	if story.Description != "" {
		sb.WriteString("### Description\n")
		sb.WriteString(story.Description)
		sb.WriteString("\n\n")
	}

	if len(story.AcceptanceCriteria) > 0 {
		sb.WriteString("### Acceptance Criteria\n")
		for _, criterion := range story.AcceptanceCriteria {
			sb.WriteString("- [ ] ")
			sb.WriteString(criterion)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if story.Notes != "" {
		sb.WriteString("### Notes\n")
		sb.WriteString(story.Notes)
		sb.WriteString("\n\n")
	}

	// Dependencies context
	if len(execCtx.CompletedStories) > 0 && len(story.DependsOn) > 0 {
		sb.WriteString("## Previously Completed Stories (for context)\n\n")
		for _, completed := range execCtx.CompletedStories {
			// Only include stories this one depends on
			for _, depID := range story.DependsOn {
				if completed.ID == depID {
					sb.WriteString("### ")
					sb.WriteString(completed.ID)
					sb.WriteString(": ")
					sb.WriteString(completed.Title)
					sb.WriteString("\n")
					if completed.Description != "" {
						sb.WriteString(completed.Description)
						sb.WriteString("\n")
					}
					sb.WriteString("\n")
					break
				}
			}
		}
	}

	// Additional context
	if execCtx.AdditionalContext != "" {
		sb.WriteString("## Additional Context\n")
		sb.WriteString(execCtx.AdditionalContext)
		sb.WriteString("\n\n")
	}

	// Instructions
	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. Read and understand the current story requirements\n")
	sb.WriteString("2. Implement the story following the acceptance criteria\n")
	sb.WriteString("3. Follow existing codebase patterns and conventions\n")
	sb.WriteString("4. Write tests for new functionality\n")
	sb.WriteString("5. Handle errors gracefully\n")
	sb.WriteString("6. Keep changes focused on the current story\n\n")

	sb.WriteString("When you have completed all acceptance criteria, clearly state that the story is complete.\n")

	return sb.String()
}

// StreamParser converts Claude stream chunks to execution events
type StreamParser struct {
	codeBlockPattern *regexp.Regexp
	filePattern      *regexp.Regexp
}

// NewStreamParser creates a new stream parser
func NewStreamParser() *StreamParser {
	return &StreamParser{
		codeBlockPattern: regexp.MustCompile("```[\\s\\S]*?```"),
		filePattern:      regexp.MustCompile(`(?:^|\s)([a-zA-Z0-9_\-./]+\.[a-zA-Z0-9]+)(?:\s|$|:)`),
	}
}

// StreamChunk represents a chunk from Claude's stream-json output
type StreamChunk struct {
	Type    string           `json:"type"`
	Subtype string           `json:"subtype,omitempty"`
	Message *AssistantMessage `json:"message,omitempty"`
	Result  string           `json:"result,omitempty"`
	IsError bool             `json:"is_error,omitempty"`
}

// AssistantMessage represents Claude's response
type AssistantMessage struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a content block in the message
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

// ParseChunk parses a JSONL line and returns an execution event
func (p *StreamParser) ParseChunk(line []byte, storyID string) *domain.ExecutionEvent {
	var chunk StreamChunk
	if err := json.Unmarshal(line, &chunk); err != nil {
		return nil
	}

	// Extract text content
	text := p.getText(&chunk)
	if text == "" {
		return nil
	}

	// Determine thought type
	thoughtType := p.classifyThought(text)

	// Extract file reference if present
	file := p.extractFile(text)

	event := domain.NewThoughtEvent(storyID, text, thoughtType)
	if file != "" {
		event = event.WithFile(file)
	}

	return &event
}

// getText extracts text content from a chunk
func (p *StreamParser) getText(chunk *StreamChunk) string {
	if chunk.Type == "assistant" && chunk.Message != nil {
		var text strings.Builder
		for _, block := range chunk.Message.Content {
			if block.Type == "text" && block.Text != "" {
				text.WriteString(block.Text)
			}
			if block.Type == "thinking" && block.Thinking != "" {
				text.WriteString(block.Thinking)
			}
		}
		return text.String()
	}

	if chunk.Type == "result" && chunk.Result != "" {
		return chunk.Result
	}

	return ""
}

// classifyThought determines the type of thought based on content
func (p *StreamParser) classifyThought(text string) domain.ThoughtType {
	lower := strings.ToLower(text)

	// Check for code patterns
	if p.codeBlockPattern.MatchString(text) {
		return domain.ThoughtTypeCode
	}

	// Check for analysis keywords
	if strings.Contains(lower, "analyzing") || strings.Contains(lower, "examining") ||
		strings.Contains(lower, "looking at") || strings.Contains(lower, "reviewing") {
		return domain.ThoughtTypeAnalysis
	}

	// Check for progress keywords
	if strings.Contains(lower, "implementing") || strings.Contains(lower, "creating") ||
		strings.Contains(lower, "adding") || strings.Contains(lower, "updating") ||
		strings.Contains(lower, "writing") {
		return domain.ThoughtTypeProgress
	}

	// Check for suggestion keywords
	if strings.Contains(lower, "suggest") || strings.Contains(lower, "recommend") ||
		strings.Contains(lower, "could") || strings.Contains(lower, "should") {
		return domain.ThoughtTypeSuggestion
	}

	return domain.ThoughtTypeGeneral
}

// extractFile attempts to extract a file path from text
func (p *StreamParser) extractFile(text string) string {
	matches := p.filePattern.FindStringSubmatch(text)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

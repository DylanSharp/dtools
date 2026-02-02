package adapters

import (
	"regexp"
	"strings"
	"time"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
	"github.com/DylanSharp/dtools/internal/coderabbit/ports"
)

// ClaudeStreamParser filters and transforms Claude JSONL output
type ClaudeStreamParser struct {
	// Patterns to detect code content
	codePatterns []*regexp.Regexp
	// Buffer for accumulating text chunks
	textBuffer strings.Builder
	// Current file being discussed
	currentFile string
}

// NewClaudeStreamParser creates a new stream parser
func NewClaudeStreamParser() *ClaudeStreamParser {
	return &ClaudeStreamParser{
		codePatterns: []*regexp.Regexp{
			// Import/export statements
			regexp.MustCompile(`^\s*(import|export|from)\s+`),
			// Function/class definitions
			regexp.MustCompile(`^\s*(function|class|const|let|var|def|async|await)\s+\w+`),
			// Common code patterns
			regexp.MustCompile(`^\s*(if|else|for|while|switch|case|return|try|catch)\s*[\(\{]?`),
			// File content with line numbers (N→)
			regexp.MustCompile(`^\s*\d+→`),
			// Package declarations
			regexp.MustCompile(`^\s*(package|module)\s+\w+`),
			// Type definitions
			regexp.MustCompile(`^\s*(type|interface|struct|enum)\s+\w+`),
			// JSON-like structures
			regexp.MustCompile(`^\s*[\{\[].*[\}\]]\s*$`),
		},
	}
}

// FilterThoughts extracts displayable thought content from stream chunks
func (p *ClaudeStreamParser) FilterThoughts(chunks <-chan ports.StreamChunk) <-chan domain.ThoughtChunk {
	filtered := make(chan domain.ThoughtChunk, 100)

	go func() {
		defer close(filtered)

		for chunk := range chunks {
			// Skip error and system chunks
			if chunk.IsStreamError() || chunk.Type == "system" {
				continue
			}

			// Extract text content
			text := chunk.GetText()
			if text == "" {
				continue
			}

			// Accumulate text and process line by line
			p.textBuffer.WriteString(text)
			buffered := p.textBuffer.String()

			// Process complete lines
			for {
				idx := strings.Index(buffered, "\n")
				if idx == -1 {
					break
				}

				line := buffered[:idx]
				buffered = buffered[idx+1:]
				p.textBuffer.Reset()
				p.textBuffer.WriteString(buffered)

				// Process the line
				if thought := p.processLine(line); thought != nil {
					filtered <- *thought
				}
			}

			// If this is the last chunk, flush the buffer
			if chunk.IsComplete() && p.textBuffer.Len() > 0 {
				remaining := p.textBuffer.String()
				if thought := p.processLine(remaining); thought != nil {
					filtered <- *thought
				}
				p.textBuffer.Reset()
			}
		}
	}()

	return filtered
}

// processLine filters a single line and returns a ThoughtChunk if displayable
func (p *ClaudeStreamParser) processLine(line string) *domain.ThoughtChunk {
	// Trim whitespace
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}

	// Check if this looks like code
	if p.isCode(trimmed) {
		return nil
	}

	// Determine thought type
	thoughtType := p.classifyThought(trimmed)

	// Extract file reference if present
	if file := p.extractFileReference(trimmed); file != "" {
		p.currentFile = file
	}

	return &domain.ThoughtChunk{
		Timestamp: time.Now(),
		Content:   trimmed,
		Type:      thoughtType,
		File:      p.currentFile,
	}
}

// isCode checks if a line looks like code
func (p *ClaudeStreamParser) isCode(line string) bool {
	// Empty or whitespace only
	if strings.TrimSpace(line) == "" {
		return false
	}

	// Very long lines are likely code
	if len(line) > 500 {
		return true
	}

	// Check against code patterns
	for _, pattern := range p.codePatterns {
		if pattern.MatchString(line) {
			return true
		}
	}

	// Lines that look like JSON objects
	if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
		return true
	}
	if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
		return true
	}

	return false
}

// classifyThought determines the type of thought based on content
func (p *ClaudeStreamParser) classifyThought(line string) domain.ThoughtType {
	lower := strings.ToLower(line)

	// Progress indicators
	if strings.Contains(lower, "analyzing") ||
		strings.Contains(lower, "reviewing") ||
		strings.Contains(lower, "checking") ||
		strings.Contains(lower, "looking at") ||
		strings.Contains(lower, "examining") {
		return domain.ThoughtTypeProgress
	}

	// Suggestions
	if strings.Contains(lower, "suggest") ||
		strings.Contains(lower, "recommend") ||
		strings.Contains(lower, "consider") ||
		strings.Contains(lower, "should") ||
		strings.Contains(lower, "could") {
		return domain.ThoughtTypeSuggestion
	}

	// Analysis
	if strings.Contains(lower, "this is") ||
		strings.Contains(lower, "the issue") ||
		strings.Contains(lower, "the problem") ||
		strings.Contains(lower, "because") ||
		strings.Contains(lower, "since") {
		return domain.ThoughtTypeAnalysis
	}

	// Default to thinking
	return domain.ThoughtTypeThinking
}

// extractFileReference extracts a file path reference from text
func (p *ClaudeStreamParser) extractFileReference(line string) string {
	// Look for file:line patterns
	filePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:in|at|file)\s+["\` + "`" + `]?([a-zA-Z0-9_\-./]+\.[a-zA-Z]+)["\` + "`" + `]?`),
		regexp.MustCompile(`([a-zA-Z0-9_\-./]+\.[a-zA-Z]+):\d+`),
		regexp.MustCompile(`\*\*([a-zA-Z0-9_\-./]+\.[a-zA-Z]+)\*\*`),
	}

	for _, pattern := range filePatterns {
		matches := pattern.FindStringSubmatch(line)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// Reset clears the parser state
func (p *ClaudeStreamParser) Reset() {
	p.textBuffer.Reset()
	p.currentFile = ""
}

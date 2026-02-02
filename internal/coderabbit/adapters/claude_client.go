package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
	"github.com/DylanSharp/dtools/internal/coderabbit/ports"
)

// ClaudeClient implements ports.AIProvider using the Claude CLI
type ClaudeClient struct {
	binaryPath string
}

// NewClaudeClient creates a new Claude CLI client
func NewClaudeClient() *ClaudeClient {
	return &ClaudeClient{
		binaryPath: "claude",
	}
}

// NewClaudeClientWithPath creates a new Claude CLI client with a custom binary path
func NewClaudeClientWithPath(binaryPath string) *ClaudeClient {
	return &ClaudeClient{
		binaryPath: binaryPath,
	}
}

// IsAvailable checks if the Claude CLI is available
func (c *ClaudeClient) IsAvailable() bool {
	_, err := exec.LookPath(c.binaryPath)
	return err == nil
}

// StreamReview starts a review and returns a channel of stream chunks
func (c *ClaudeClient) StreamReview(ctx context.Context, prompt string) (<-chan ports.StreamChunk, error) {
	if !c.IsAvailable() {
		return nil, domain.ErrClaudeNotFound()
	}

	// Build the Claude command with streaming JSON output
	cmd := exec.CommandContext(ctx, c.binaryPath,
		"-p",
		"--dangerously-skip-permissions",
		"--output-format", "stream-json",
		"--",
		prompt,
	)

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

	chunks := make(chan ports.StreamChunk, 100)

	// Read stderr in background for error messages
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// Log stderr but don't block on it
			_ = scanner.Text()
		}
	}()

	// Read JSONL from stdout
	go func() {
		defer close(chunks)
		defer cmd.Wait()

		scanner := bufio.NewScanner(stdout)
		// Increase buffer size for potentially large JSON objects
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var chunk ports.StreamChunk
			if err := json.Unmarshal(line, &chunk); err != nil {
				// Send parse error but continue
				chunks <- ports.StreamChunk{
					Type: "error",
					Error: &ports.StreamError{
						Type:    "parse_error",
						Message: err.Error(),
					},
				}
				continue
			}

			chunks <- chunk
		}

		if err := scanner.Err(); err != nil {
			chunks <- ports.StreamChunk{
				Type: "error",
				Error: &ports.StreamError{
					Type:    "scan_error",
					Message: err.Error(),
				},
			}
		}
	}()

	return chunks, nil
}

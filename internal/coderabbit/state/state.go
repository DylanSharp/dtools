package state

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
)

var (
	stateDir  = filepath.Join(os.Getenv("HOME"), ".config", "dtools")
	stateFile = filepath.Join(stateDir, "review-state.json")
	mu        sync.Mutex
)

// TrackerState holds the state for a single PR
type TrackerState struct {
	ProcessedCommentIDs []int              `json:"processedCommentIds"`
	ProcessedByHash     []string           `json:"processedByHash"`
	SeenComments        map[int]SeenInfo   `json:"seenComments"`
	LastReviewTimestamp string             `json:"lastProcessedReviewSubmittedAt,omitempty"`
}

// SeenInfo tracks when we last saw a comment and its content hash
type SeenInfo struct {
	UpdatedAt string `json:"updated_at"`
	BodyHash  string `json:"bodyHash"`
}

// TrackerData is the full state file containing all PRs
type TrackerData map[string]*TrackerState

// HashComment creates a unique hash for a comment based on file, line, and body
func HashComment(filePath string, line int, body string) string {
	if filePath == "" {
		filePath = "GENERAL"
	}
	input := fmt.Sprintf("%s|%d|%s", filePath, line, body)
	hash := sha1.Sum([]byte(input))
	return hex.EncodeToString(hash[:])
}

// GetStateKey returns the key for a PR in the state file
func GetStateKey(owner, repo string, pr int) string {
	return fmt.Sprintf("%s/%s#%d", owner, repo, pr)
}

// Load reads the state file from disk
func Load() (TrackerData, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		return make(TrackerData), nil
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state TrackerData
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return state, nil
}

// Save writes the state file to disk
func Save(data TrackerData) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(stateFile, content, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// GetOrCreate returns the state for a PR, creating it if it doesn't exist
func GetOrCreate(key string) (*TrackerState, error) {
	data, err := Load()
	if err != nil {
		return nil, err
	}

	if data[key] == nil {
		data[key] = &TrackerState{
			ProcessedCommentIDs: []int{},
			ProcessedByHash:     []string{},
			SeenComments:        make(map[int]SeenInfo),
		}
	}

	return data[key], nil
}

// IsCommentProcessed checks if a comment has already been processed
func IsCommentProcessed(state *TrackerState, comment domain.Comment) bool {
	hash := HashComment(comment.FilePath, comment.LineNumber, comment.Body)

	// Check by ID
	for _, id := range state.ProcessedCommentIDs {
		if id == comment.ID {
			return true
		}
	}

	// Check by hash (catches duplicate comments with different IDs)
	for _, h := range state.ProcessedByHash {
		if h == hash {
			return true
		}
	}

	return false
}

// HasCommentChanged checks if a comment has been updated since we last saw it
func HasCommentChanged(state *TrackerState, comment domain.Comment) bool {
	seen, exists := state.SeenComments[comment.ID]
	if !exists {
		return true
	}

	// Check if updated_at changed
	if !comment.UpdatedAt.IsZero() && seen.UpdatedAt != comment.UpdatedAt.Format("2006-01-02T15:04:05Z07:00") {
		return true
	}

	// Check if body changed
	hash := HashComment(comment.FilePath, comment.LineNumber, comment.Body)
	if seen.BodyHash != hash {
		return true
	}

	return false
}

// MarkProcessed marks comments as processed and saves state
func MarkProcessed(key string, comments []domain.Comment, reviewTimestamp string) error {
	data, err := Load()
	if err != nil {
		return err
	}

	state := data[key]
	if state == nil {
		state = &TrackerState{
			ProcessedCommentIDs: []int{},
			ProcessedByHash:     []string{},
			SeenComments:        make(map[int]SeenInfo),
		}
		data[key] = state
	}

	for _, comment := range comments {
		hash := HashComment(comment.FilePath, comment.LineNumber, comment.Body)

		// Add ID if not already present
		found := false
		for _, id := range state.ProcessedCommentIDs {
			if id == comment.ID {
				found = true
				break
			}
		}
		if !found {
			state.ProcessedCommentIDs = append(state.ProcessedCommentIDs, comment.ID)
		}

		// Add hash if not already present
		found = false
		for _, h := range state.ProcessedByHash {
			if h == hash {
				found = true
				break
			}
		}
		if !found {
			state.ProcessedByHash = append(state.ProcessedByHash, hash)
		}

		// Update seen info
		state.SeenComments[comment.ID] = SeenInfo{
			UpdatedAt: comment.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			BodyHash:  hash,
		}
	}

	if reviewTimestamp != "" {
		state.LastReviewTimestamp = reviewTimestamp
	}

	return Save(data)
}

// Reset clears the state for a PR
func Reset(key string) error {
	data, err := Load()
	if err != nil {
		return err
	}

	delete(data, key)
	return Save(data)
}

// FilterUnprocessed returns only comments that haven't been processed yet
func FilterUnprocessed(state *TrackerState, comments []domain.Comment) []domain.Comment {
	var unprocessed []domain.Comment

	for _, comment := range comments {
		// Only include comments that haven't been processed
		// Don't reprocess just because timestamp changed - CodeRabbit updates timestamps on re-review
		if !IsCommentProcessed(state, comment) {
			unprocessed = append(unprocessed, comment)
		}
	}

	return unprocessed
}

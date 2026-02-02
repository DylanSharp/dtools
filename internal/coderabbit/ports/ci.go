package ports

import (
	"context"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
)

// CIProvider abstracts CI test failure retrieval
type CIProvider interface {
	// GetTestFailures retrieves failed CI checks for a commit
	GetTestFailures(ctx context.Context, owner, repo string, commitSHA string) ([]domain.CITestFailure, error)

	// GetWorkflowRuns retrieves workflow runs for a PR
	GetWorkflowRuns(ctx context.Context, owner, repo string, prNumber int) ([]WorkflowRun, error)
}

// WorkflowRun represents a CI workflow run
type WorkflowRun struct {
	ID         int64
	Name       string
	Status     string // queued, in_progress, completed
	Conclusion string // success, failure, neutral, cancelled, skipped, timed_out, action_required
	LogURL     string
}

// IsFailed returns true if the workflow run failed
func (w WorkflowRun) IsFailed() bool {
	return w.Status == "completed" && w.Conclusion == "failure"
}

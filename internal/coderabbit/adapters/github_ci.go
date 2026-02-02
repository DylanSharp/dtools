package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
	"github.com/DylanSharp/dtools/internal/coderabbit/ports"
)

// GitHubCIAdapter implements ports.CIProvider using the gh CLI
type GitHubCIAdapter struct{}

// NewGitHubCIAdapter creates a new GitHub CI adapter
func NewGitHubCIAdapter() *GitHubCIAdapter {
	return &GitHubCIAdapter{}
}

// ghCheckRun represents a GitHub check run from the API
type ghCheckRun struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion"`
	HTMLURL     string `json:"html_url"`
	Output      struct {
		Title        string `json:"title"`
		Summary      string `json:"summary"`
		Text         string `json:"text"`
		AnnotationsCount int `json:"annotations_count"`
	} `json:"output"`
	App struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"app"`
}

// ghAnnotation represents a check run annotation
type ghAnnotation struct {
	Path            string `json:"path"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	AnnotationLevel string `json:"annotation_level"`
	Title           string `json:"title"`
	Message         string `json:"message"`
	RawDetails      string `json:"raw_details"`
}

// ghCheckRuns is the API response wrapper
type ghCheckRuns struct {
	CheckRuns []ghCheckRun `json:"check_runs"`
}

// GetTestFailures retrieves failed CI checks for a commit
func (a *GitHubCIAdapter) GetTestFailures(ctx context.Context, owner, repo, commitSHA string) ([]domain.CITestFailure, error) {
	// Get all check runs for the commit
	args := []string{
		"api",
		fmt.Sprintf("repos/%s/%s/commits/%s/check-runs", owner, repo, commitSHA),
		"--paginate",
	}

	out, err := a.runGH(ctx, args...)
	if err != nil {
		return nil, domain.ErrGitHubAPI("failed to fetch check runs", err)
	}

	var checkRuns ghCheckRuns
	if err := json.Unmarshal(out, &checkRuns); err != nil {
		return nil, domain.ErrJSONParse("failed to parse check runs", err)
	}

	var failures []domain.CITestFailure

	for _, run := range checkRuns.CheckRuns {
		// Only include failed checks
		if run.Status != "completed" || run.Conclusion != "failure" {
			continue
		}

		failure := domain.CITestFailure{
			CheckName: run.Name,
			JobName:   run.Name,
			AppName:   run.App.Name,
			Summary:   run.Output.Summary,
			LogURL:    run.HTMLURL,
		}

		// Fetch annotations if there are any
		if run.Output.AnnotationsCount > 0 {
			annotations, err := a.getAnnotations(ctx, owner, repo, run.ID)
			if err == nil {
				failure.Annotations = annotations
			}
		}

		// If no annotations, try to get the output text
		if len(failure.Annotations) == 0 && run.Output.Text != "" {
			// Truncate output text if too long
			text := run.Output.Text
			if len(text) > 5000 {
				text = text[:5000] + "\n... [truncated]"
			}
			failure.ErrorMessage = text
		}

		failures = append(failures, failure)
	}

	return failures, nil
}

// GetCIStatus retrieves the full CI status including pending, passed, and failed checks
func (a *GitHubCIAdapter) GetCIStatus(ctx context.Context, owner, repo, commitSHA string) (domain.CIStatus, error) {
	args := []string{
		"api",
		fmt.Sprintf("repos/%s/%s/commits/%s/check-runs", owner, repo, commitSHA),
		"--paginate",
	}

	out, err := a.runGH(ctx, args...)
	if err != nil {
		return domain.CIStatus{}, domain.ErrGitHubAPI("failed to fetch check runs", err)
	}

	var checkRuns ghCheckRuns
	if err := json.Unmarshal(out, &checkRuns); err != nil {
		return domain.CIStatus{}, domain.ErrJSONParse("failed to parse check runs", err)
	}

	status := domain.CIStatus{
		TotalCount: len(checkRuns.CheckRuns),
	}

	for _, run := range checkRuns.CheckRuns {
		// Check if this is a CodeRabbit check
		isCodeRabbit := strings.Contains(strings.ToLower(run.Name), "coderabbit") ||
			strings.Contains(strings.ToLower(run.App.Name), "coderabbit") ||
			strings.Contains(strings.ToLower(run.App.Slug), "coderabbit")

		if isCodeRabbit {
			status.CodeRabbitFound = true
		}

		switch run.Status {
		case "completed":
			if isCodeRabbit {
				status.CodeRabbitCompleted = true
			}
			if run.Conclusion == "failure" {
				failure := domain.CITestFailure{
					CheckName: run.Name,
					JobName:   run.Name,
					AppName:   run.App.Name,
					Summary:   run.Output.Summary,
					LogURL:    run.HTMLURL,
				}

				// Fetch annotations if there are any
				if run.Output.AnnotationsCount > 0 {
					annotations, err := a.getAnnotations(ctx, owner, repo, run.ID)
					if err == nil {
						failure.Annotations = annotations
					}
				}

				// If no annotations, try to get the output text
				if len(failure.Annotations) == 0 && run.Output.Text != "" {
					text := run.Output.Text
					if len(text) > 5000 {
						text = text[:5000] + "\n... [truncated]"
					}
					failure.ErrorMessage = text
				}

				status.Failures = append(status.Failures, failure)
			} else if run.Conclusion == "success" {
				status.PassedCount++
			}
			// Skip neutral, cancelled, skipped - they don't count as pass or fail
		case "queued", "in_progress":
			status.PendingCount++
			status.PendingNames = append(status.PendingNames, run.Name)
		}
	}

	return status, nil
}

// GetWorkflowRuns retrieves workflow runs for a PR
func (a *GitHubCIAdapter) GetWorkflowRuns(ctx context.Context, owner, repo string, prNumber int) ([]ports.WorkflowRun, error) {
	args := []string{
		"pr", "checks", fmt.Sprintf("%d", prNumber),
		"--repo", fmt.Sprintf("%s/%s", owner, repo),
		"--json", "name,state,conclusion,link",
	}

	out, err := a.runGH(ctx, args...)
	if err != nil {
		return nil, domain.ErrGitHubAPI("failed to fetch workflow runs", err)
	}

	var checks []struct {
		Name       string `json:"name"`
		State      string `json:"state"`
		Conclusion string `json:"conclusion"`
		Link       string `json:"link"`
	}

	if err := json.Unmarshal(out, &checks); err != nil {
		return nil, domain.ErrJSONParse("failed to parse workflow runs", err)
	}

	var runs []ports.WorkflowRun
	for _, check := range checks {
		runs = append(runs, ports.WorkflowRun{
			Name:       check.Name,
			Status:     check.State,
			Conclusion: check.Conclusion,
			LogURL:     check.Link,
		})
	}

	return runs, nil
}

// getAnnotations fetches annotations for a specific check run
func (a *GitHubCIAdapter) getAnnotations(ctx context.Context, owner, repo string, checkRunID int64) ([]domain.CIAnnotation, error) {
	args := []string{
		"api",
		fmt.Sprintf("repos/%s/%s/check-runs/%d/annotations", owner, repo, checkRunID),
	}

	out, err := a.runGH(ctx, args...)
	if err != nil {
		return nil, err
	}

	var ghAnnotations []ghAnnotation
	if err := json.Unmarshal(out, &ghAnnotations); err != nil {
		return nil, err
	}

	var annotations []domain.CIAnnotation
	for _, ann := range ghAnnotations {
		// Only include failure and warning level annotations
		if ann.AnnotationLevel != "failure" && ann.AnnotationLevel != "warning" {
			continue
		}

		annotations = append(annotations, domain.CIAnnotation{
			Path:       ann.Path,
			StartLine:  ann.StartLine,
			EndLine:    ann.EndLine,
			Title:      ann.Title,
			Message:    ann.Message,
			RawDetails: ann.RawDetails,
		})
	}

	return annotations, nil
}

// runGH executes a gh CLI command and returns the output
func (a *GitHubCIAdapter) runGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh command failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}
	return out, nil
}

package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/dylan/worktree-dev/internal/worktree"
)

// SelectBranchWorkflow guides the user through creating or selecting a branch
func SelectBranchWorkflow(repo *worktree.Repo) (string, error) {
	var choice string

	// First: choose between new or existing branch
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Create a new worktree").
				Options(
					huh.NewOption("Create new branch", "new"),
					huh.NewOption("Use existing branch", "existing"),
				).
				Value(&choice),
		),
	)

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return "", nil
		}
		return "", err
	}

	if choice == "new" {
		return promptNewBranch()
	}

	return selectExistingBranch(repo)
}

// promptNewBranch asks the user for a new branch name
func promptNewBranch() (string, error) {
	var branchName string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter new branch name").
				Placeholder("feature/my-feature").
				Value(&branchName).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("branch name cannot be empty")
					}
					return nil
				}),
		),
	)

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return "", nil
		}
		return "", err
	}

	return branchName, nil
}

// selectExistingBranch shows a list of branches to choose from
func selectExistingBranch(repo *worktree.Repo) (string, error) {
	local, remote, err := repo.GetBranches()
	if err != nil {
		return "", err
	}

	if len(local) == 0 && len(remote) == 0 {
		return "", fmt.Errorf("no other branches available\nCreate a new branch first or use: worktree-dev create <new-branch-name>")
	}

	// Build options list
	var options []huh.Option[string]

	for _, b := range local {
		options = append(options, huh.NewOption(fmt.Sprintf("%s (local)", b), b))
	}

	for _, b := range remote {
		options = append(options, huh.NewOption(fmt.Sprintf("%s (remote)", b), b))
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a branch").
				Options(options...).
				Value(&selected).
				Height(15), // Show more options at once
		),
	)

	err = form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return "", nil
		}
		return "", err
	}

	return selected, nil
}

package main

import (
	"fmt"
	"os"

	"github.com/dylan/worktree-dev/internal/ui"
	"github.com/dylan/worktree-dev/internal/worktree"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "worktree-dev",
	Short: "Git worktree manager with isolated Docker environments",
	Long: `worktree-dev creates git worktrees that can run docker-compose independently
without port conflicts, container name collisions, or shared volumes.

Each worktree gets:
  - Isolated Docker containers (unique COMPOSE_PROJECT_NAME)
  - Unique host ports (auto-detected from docker-compose.yml)
  - Separate volumes (fresh database per worktree)
  - A ./dev helper script for common commands`,
}

var createCmd = &cobra.Command{
	Use:   "create [branch]",
	Short: "Create a new worktree",
	Long:  "Create a new worktree. If no branch is specified, interactive mode will guide you.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := worktree.NewRepo()
		if err != nil {
			return err
		}

		var branch string
		if len(args) > 0 {
			branch = args[0]
		} else {
			// Interactive mode
			branch, err = ui.SelectBranchWorkflow(repo)
			if err != nil {
				return err
			}
			if branch == "" {
				return nil // User cancelled
			}
		}

		return repo.CreateWorktree(branch)
	},
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all worktrees",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := worktree.NewRepo()
		if err != nil {
			return err
		}
		return repo.ListWorktrees()
	},
}

var removeCmd = &cobra.Command{
	Use:     "remove [branch]",
	Aliases: []string{"rm"},
	Short:   "Remove a worktree and cleanup Docker resources",
	Long:    "Remove a worktree. If no branch specified and you're inside a worktree, removes the current one.",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := worktree.NewRepo()
		if err != nil {
			return err
		}

		var branch string
		if len(args) > 0 {
			branch = args[0]
		} else {
			// Check if we're inside a worktree
			branch = repo.CurrentWorktree()
			if branch == "" {
				return fmt.Errorf("not inside a worktree. Usage: worktree-dev remove <branch>")
			}
		}

		return repo.RemoveWorktree(branch)
	},
}

var portsCmd = &cobra.Command{
	Use:   "ports <branch>",
	Short: "Show ports that would be allocated for a branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := worktree.NewRepo()
		if err != nil {
			return err
		}
		return repo.ShowPorts(args[0])
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(portsCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

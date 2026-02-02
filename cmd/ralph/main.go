package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/DylanSharp/dtools/internal/ralph/adapters"
	"github.com/DylanSharp/dtools/internal/ralph/ports"
	"github.com/DylanSharp/dtools/internal/ralph/service"
	"github.com/DylanSharp/dtools/internal/ralph/ui"
)

//go:embed templates/*
var templateFS embed.FS

var (
	prdFile string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "ralph",
	Short: "PRD-based Claude agent loop",
	Long: `Ralph executes user stories from a Product Requirements Document (PRD)
using Claude as the AI agent. Stories are executed sequentially, respecting
dependencies between them.

Example workflow:
  1. ralph init          # Create a new PRD file
  2. Edit prd.md         # Define your stories
  3. ralph run           # Execute stories with Claude`,
}

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new ralph project",
	Long: `Create a new PRD file from template.

If no name is provided, uses the current directory name.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

var statusCmd = &cobra.Command{
	Use:   "status [prd-file]",
	Short: "Show project status",
	Long: `Display the current status of a ralph project, including:
- Total stories and completion progress
- Story status (pending, blocked, completed, failed)
- Dependency information`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

var runCmd = &cobra.Command{
	Use:   "run [prd-file]",
	Short: "Execute project stories",
	Long: `Run the ralph agent loop to execute stories from a PRD file.

Stories are executed sequentially in dependency order. Claude is used
to implement each story, and progress is displayed in a terminal UI.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProject,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all ralph projects",
	Long:  `List all ralph projects that have been initialized.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(listCmd)

	// Flags
	runCmd.Flags().StringVarP(&prdFile, "prd", "p", "prd.md", "Path to PRD file")
	statusCmd.Flags().StringVarP(&prdFile, "prd", "p", "prd.md", "Path to PRD file")
}

// runInit initializes a new ralph project
func runInit(cmd *cobra.Command, args []string) error {
	// Determine project name
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not get current directory: %w", err)
		}
		name = filepath.Base(cwd)
	}

	// Check if prd.md already exists
	prdPath := "prd.md"
	if _, err := os.Stat(prdPath); err == nil {
		return fmt.Errorf("prd.md already exists. Delete it first or use a different name")
	}

	// Load template
	template, err := templateFS.ReadFile("templates/prd_template.md")
	if err != nil {
		return fmt.Errorf("could not load template: %w", err)
	}

	// Replace placeholders
	content := strings.ReplaceAll(string(template), "{{PROJECT_NAME}}", name)

	// Write file
	if err := os.WriteFile(prdPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("could not write prd.md: %w", err)
	}

	fmt.Printf("✓ Initialized ralph project: %s\n", name)
	fmt.Printf("  Created: %s\n\n", prdPath)
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit prd.md to define your stories")
	fmt.Println("  2. Run 'ralph run' to start implementing")

	return nil
}

// runStatus shows project status
func runStatus(cmd *cobra.Command, args []string) error {
	// Get PRD path
	prdPath := prdFile
	if len(args) > 0 {
		prdPath = args[0]
	}

	// Create service
	svc, err := createService()
	if err != nil {
		return err
	}

	// Try to load existing project, or initialize from PRD
	project, err := svc.GetProject(prdPath)
	if err != nil {
		// Try to initialize from PRD
		project, err = svc.InitProject(prdPath)
		if err != nil {
			return fmt.Errorf("could not load project: %w", err)
		}
	}

	// Display status using TUI
	model := ui.NewStatusModel(project)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// runProject executes the project
func runProject(cmd *cobra.Command, args []string) error {
	// Get PRD path
	prdPath := prdFile
	if len(args) > 0 {
		prdPath = args[0]
	}

	// Create service
	svc, err := createService()
	if err != nil {
		return err
	}

	// Check Claude availability
	executor := adapters.NewClaudeExecutor()
	if !executor.IsAvailable() {
		return fmt.Errorf("Claude CLI not found. Please install Claude Code first")
	}

	// Try to load existing project, or initialize from PRD
	project, err := svc.GetProject(prdPath)
	if err != nil {
		// Try to initialize from PRD
		project, err = svc.InitProject(prdPath)
		if err != nil {
			return fmt.Errorf("could not load project: %w", err)
		}
		fmt.Printf("Initialized project: %s\n", project.Name)
	}

	// Check if already complete
	if project.IsComplete() {
		fmt.Println("✓ All stories already complete!")
		return nil
	}

	// Run TUI
	model := ui.NewModel(svc, project.ID)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Final status
	if m, ok := finalModel.(*ui.Model); ok {
		project := m.GetProject()
		if project != nil {
			fmt.Printf("\nProject: %s\n", project.Name)
			fmt.Printf("Completed: %d/%d stories\n", project.CompletedStories(), project.TotalStories())
			if project.IsComplete() {
				fmt.Println("✓ All stories complete!")
			} else if project.HasFailures() {
				fmt.Printf("✗ %d stories failed\n", project.FailedStories())
			}
		}
	}

	return nil
}

// runList lists all projects
func runList(cmd *cobra.Command, args []string) error {
	// Create repository
	repo, err := adapters.NewJSONRepository()
	if err != nil {
		return err
	}

	projects, err := repo.List()
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		fmt.Println("Run 'ralph init' to create a new project.")
		return nil
	}

	fmt.Printf("Found %d project(s):\n\n", len(projects))
	for _, p := range projects {
		status := string(p.Status)
		switch p.Status {
		case "completed":
			status = "✓ " + status
		case "failed":
			status = "✗ " + status
		case "running":
			status = "▶ " + status
		default:
			status = "○ " + status
		}

		fmt.Printf("  %s\n", p.Name)
		fmt.Printf("    Status: %s (%d/%d stories)\n", status, p.CompletedStories, p.TotalStories)
		fmt.Printf("    PRD: %s\n", p.PRDPath)
		fmt.Printf("    Updated: %s\n\n", p.UpdatedAt)
	}

	return nil
}

// createService creates the project service with all dependencies
func createService() (*service.ProjectService, error) {
	// Create adapters
	parser := adapters.NewMarkdownPRDParser(ports.DefaultPRDParseOptions())
	executor := adapters.NewClaudeExecutor()
	repo, err := adapters.NewJSONRepository()
	if err != nil {
		return nil, fmt.Errorf("could not create repository: %w", err)
	}

	// Create service
	return service.NewProjectService(parser, executor, repo), nil
}

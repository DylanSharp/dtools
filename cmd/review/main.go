package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/DylanSharp/dtools/internal/coderabbit/adapters"
	"github.com/DylanSharp/dtools/internal/coderabbit/service"
	"github.com/DylanSharp/dtools/internal/coderabbit/ui"
)

var (
	prNumber         int
	watchMode        bool
	includeNits      bool
	includeOutdated  bool
	pollInterval     int
	cooldownDuration int
	noManualConfirm  bool
	resetState       bool
	markAddressed    bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "review [pr-number]",
	Short: "Review CodeRabbit PR comments with Claude",
	Long: `Review CodeRabbit PR comments using Claude AI.

This tool fetches CodeRabbit review comments from a GitHub PR, generates
a prompt for Claude, and displays Claude's analysis in a terminal UI.

In watch mode, it continuously monitors for new comments and CI failures,
automatically triggering Claude reviews until CodeRabbit is satisfied.`,
	Example: `  # Review current branch's PR
  review

  # Review specific PR
  review 123

  # Watch mode with auto-review
  review --watch

  # Watch mode with custom settings
  review --watch --poll-interval 30 --cooldown 120`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReview,
}

func init() {
	rootCmd.Flags().IntVarP(&prNumber, "pr", "p", 0, "PR number (auto-detected if not specified)")
	rootCmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "Enable watch mode for continuous review")
	rootCmd.Flags().BoolVar(&includeNits, "include-nits", true, "Include nitpick comments")
	rootCmd.Flags().BoolVar(&includeOutdated, "include-outdated", true, "Include outdated comments")
	rootCmd.Flags().IntVar(&pollInterval, "poll-interval", 15, "Watch mode poll interval in seconds")
	rootCmd.Flags().IntVar(&cooldownDuration, "cooldown", 180, "Watch mode cooldown after review in seconds")
	rootCmd.Flags().BoolVar(&noManualConfirm, "no-manual-confirm", false, "Skip manual confirmation in watch mode")
	rootCmd.Flags().BoolVar(&resetState, "reset", false, "Reset state and re-process all comments")
	rootCmd.Flags().BoolVar(&markAddressed, "mark-addressed", true, "Mark comments as resolved on GitHub after addressing")
}

func runReview(cmd *cobra.Command, args []string) error {
	// Parse PR number from args if provided
	if len(args) > 0 {
		_, err := fmt.Sscanf(args[0], "%d", &prNumber)
		if err != nil {
			return fmt.Errorf("invalid PR number: %s", args[0])
		}
	}

	// Create adapters
	githubClient := adapters.NewGitHubCLIClient()
	ciProvider := adapters.NewGitHubCIAdapter()
	claudeClient := adapters.NewClaudeClient()

	// Check if Claude is available
	if !claudeClient.IsAvailable() {
		return fmt.Errorf("Claude CLI not found. Please install Claude Code first.")
	}

	// Create review service
	reviewService := service.NewReviewService(githubClient, ciProvider, claudeClient)

	// Auto-detect PR if not specified
	if prNumber == 0 {
		detected, err := reviewService.DetectCurrentPR(cmd.Context())
		if err != nil {
			return fmt.Errorf("could not detect PR number: %w\nUse --pr flag to specify the PR number", err)
		}
		prNumber = detected
		fmt.Printf("Detected PR #%d\n", prNumber)
	}

	// Create config
	config := service.ReviewConfig{
		PRNumber:        prNumber,
		IncludeNits:     includeNits,
		IncludeOutdated: includeOutdated,
		ResetState:      resetState,
		MarkAddressed:   markAddressed,
	}

	// Create the appropriate model
	var model tea.Model
	if watchMode {
		watchOpts := service.WatchOptions{
			PollInterval:         time.Duration(pollInterval) * time.Second,
			CooldownDuration:     time.Duration(cooldownDuration) * time.Second,
			RequireManualConfirm: !noManualConfirm,
			IncludeNits:          includeNits,
			IncludeOutdated:      includeOutdated,
		}
		model = ui.NewWatchModel(reviewService, config, watchOpts)
	} else {
		model = ui.NewModel(reviewService, config)
	}

	// Run the TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check if review was successful
	if m, ok := finalModel.(*ui.Model); ok {
		review := m.GetReview()
		if review != nil {
			fmt.Printf("\nReview complete for PR #%d\n", review.PRNumber)
			if review.Satisfied {
				fmt.Println("✓ CodeRabbit is satisfied!")
			}
		}
	}

	return nil
}

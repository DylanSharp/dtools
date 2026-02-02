package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/DylanSharp/dtools/internal/coderabbit/adapters"
	"github.com/DylanSharp/dtools/internal/coderabbit/service"
	"github.com/DylanSharp/dtools/internal/coderabbit/ui"
)

var (
	reviewPRNumber         int
	reviewWatchMode        bool
	reviewIncludeNits      bool
	reviewIncludeOutdated  bool
	reviewPollInterval     int
	reviewCooldownDuration int
	reviewNoManualConfirm  bool
	reviewResetState       bool
	reviewMarkAddressed    bool
	reviewDebug            bool
)

var reviewCmd = &cobra.Command{
	Use:   "review [pr-number]",
	Short: "Review CodeRabbit PR comments with Claude",
	Long: `Review CodeRabbit PR comments using Claude AI.

This tool fetches CodeRabbit review comments from a GitHub PR, generates
a prompt for Claude, and displays Claude's analysis in a terminal UI.

In watch mode, it continuously monitors for new comments and CI failures,
automatically triggering Claude reviews until CodeRabbit is satisfied.`,
	Example: `  # Review current branch's PR
  dtools review

  # Review specific PR
  dtools review 123

  # Watch mode with auto-review
  dtools review --watch

  # Watch mode with custom settings
  dtools review --watch --poll-interval 30 --cooldown 120`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReview,
}

func init() {
	reviewCmd.Flags().IntVarP(&reviewPRNumber, "pr", "p", 0, "PR number (auto-detected if not specified)")
	reviewCmd.Flags().BoolVarP(&reviewWatchMode, "watch", "w", false, "Enable watch mode for continuous review")
	reviewCmd.Flags().BoolVar(&reviewIncludeNits, "include-nits", true, "Include nitpick comments")
	reviewCmd.Flags().BoolVar(&reviewIncludeOutdated, "include-outdated", true, "Include outdated comments")
	reviewCmd.Flags().IntVar(&reviewPollInterval, "poll-interval", 15, "Watch mode poll interval in seconds")
	reviewCmd.Flags().IntVar(&reviewCooldownDuration, "cooldown", 180, "Watch mode cooldown after review in seconds")
	reviewCmd.Flags().BoolVar(&reviewNoManualConfirm, "no-manual-confirm", false, "Skip manual confirmation in watch mode")
	reviewCmd.Flags().BoolVar(&reviewResetState, "reset", false, "Reset state and re-process all comments")
	reviewCmd.Flags().BoolVar(&reviewMarkAddressed, "mark-addressed", true, "Mark comments as resolved on GitHub after addressing")
	reviewCmd.Flags().BoolVar(&reviewDebug, "debug", false, "Print debug info about comments without starting TUI")
	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	// Parse PR number from args if provided
	if len(args) > 0 {
		_, err := fmt.Sscanf(args[0], "%d", &reviewPRNumber)
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
	if reviewPRNumber == 0 {
		detected, err := reviewService.DetectCurrentPR(cmd.Context())
		if err != nil {
			return fmt.Errorf("could not detect PR number: %w\nUse --pr flag to specify the PR number", err)
		}
		reviewPRNumber = detected
		fmt.Printf("Detected PR #%d\n", reviewPRNumber)
	}

	// Create config
	config := service.ReviewConfig{
		PRNumber:        reviewPRNumber,
		IncludeNits:     reviewIncludeNits,
		IncludeOutdated: reviewIncludeOutdated,
		ResetState:      reviewResetState,
		MarkAddressed:   reviewMarkAddressed,
	}

	// Debug mode - print what would be processed without TUI
	if reviewDebug {
		review, err := reviewService.FetchReviewData(cmd.Context(), config)
		if err != nil {
			return fmt.Errorf("failed to fetch review data: %w", err)
		}

		fmt.Printf("\n=== DEBUG: PR #%d ===\n", review.PRNumber)
		fmt.Printf("Total comments found: %d\n", review.TotalFoundCount)
		fmt.Printf("Already addressed: %d\n", review.AlreadyAddressed)
		fmt.Printf("New comments to process: %d\n", review.NewCommentsCount)
		fmt.Printf("CI failures: %d\n", len(review.CIFailures))

		if len(review.Comments) == 0 {
			fmt.Println("\nNo comments to process - CodeRabbit should be satisfied!")
		} else {
			fmt.Printf("\nComments to process:\n")
			for i, c := range review.Comments {
				resolved := ""
				if c.IsResolved {
					resolved = " [RESOLVED - BUG!]"
				}
				fmt.Printf("  %d. ID=%d Path=%s Line=%d%s\n", i+1, c.ID, c.FilePath, c.LineNumber, resolved)
				fmt.Printf("     Body: %.100s...\n", c.Body)
			}
		}
		return nil
	}

	// Create the appropriate model
	var model tea.Model
	if reviewWatchMode {
		watchOpts := service.WatchOptions{
			PollInterval:         time.Duration(reviewPollInterval) * time.Second,
			CooldownDuration:     time.Duration(reviewCooldownDuration) * time.Second,
			RequireManualConfirm: !reviewNoManualConfirm,
			IncludeNits:          reviewIncludeNits,
			IncludeOutdated:      reviewIncludeOutdated,
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
				fmt.Println("CodeRabbit is satisfied!")
			}
		}
	}

	return nil
}

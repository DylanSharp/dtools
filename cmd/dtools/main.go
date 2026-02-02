package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dtools",
	Short: "Dylan's DevTools Kit",
	Long: `dtools (dt) - A collection of developer tools:

  worktree  Git worktree manager with isolated Docker environments
  review    CodeRabbit PR comment reviewer with Claude
  ralph     PRD-based story execution with Claude`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

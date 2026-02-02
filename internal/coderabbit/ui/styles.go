package ui

import "github.com/charmbracelet/lipgloss"

// Colors matching the existing dtools theme
var (
	// Basic colors
	Green   = lipgloss.Color("2")
	Red     = lipgloss.Color("1")
	Yellow  = lipgloss.Color("3")
	Blue    = lipgloss.Color("4")
	Cyan    = lipgloss.Color("6")
	Magenta = lipgloss.Color("5")
	White   = lipgloss.Color("15")
	Gray    = lipgloss.Color("8")
	DimGray = lipgloss.Color("240")
)

// Text styles
var (
	// Success style for positive messages
	SuccessStyle = lipgloss.NewStyle().Foreground(Green)

	// Error style for error messages
	ErrorStyle = lipgloss.NewStyle().Foreground(Red).Bold(true)

	// Warning style for warnings
	WarnStyle = lipgloss.NewStyle().Foreground(Yellow)

	// Info style for informational messages
	InfoStyle = lipgloss.NewStyle().Foreground(Blue)

	// Cyan style for highlights
	CyanStyle = lipgloss.NewStyle().Foreground(Cyan)

	// Bold style for emphasis
	BoldStyle = lipgloss.NewStyle().Bold(true)

	// Dim style for secondary text
	DimStyle = lipgloss.NewStyle().Faint(true)

	// Header style for section headers
	HeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(White).
		Background(Blue).
		Padding(0, 1)
)

// Status bar styles
var (
	// StatusBarStyle is the base style for the status bar
	StatusBarStyle = lipgloss.NewStyle().
		Foreground(White).
		Background(Gray)

	// StatusBarBrandStyle is for the tool name badge
	StatusBarBrandStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(Blue).
		Padding(0, 1)

	// StatusBarSectionStyle is for individual sections
	StatusBarSectionStyle = lipgloss.NewStyle().
		Foreground(White).
		Background(Gray).
		Padding(0, 1)

	// StatusBarDividerStyle is for dividers between sections
	StatusBarDividerStyle = lipgloss.NewStyle().
		Foreground(DimGray).
		Background(Gray)

	// StatusBarProgressStyle is for the progress indicator
	StatusBarProgressStyle = lipgloss.NewStyle().
		Foreground(Green).
		Background(Gray)

	// StatusBarWarningStyle is for warnings in the status bar
	StatusBarWarningStyle = lipgloss.NewStyle().
		Foreground(Yellow).
		Background(Gray)

	// StatusBarErrorStyle is for errors in the status bar
	StatusBarErrorStyle = lipgloss.NewStyle().
		Foreground(Red).
		Background(Gray)
)

// Thought display styles
var (
	// ThoughtStyle is the base style for thought content
	ThoughtStyle = lipgloss.NewStyle().
		Foreground(White).
		PaddingLeft(2)

	// ThoughtProgressStyle is for progress/status thoughts
	ThoughtProgressStyle = lipgloss.NewStyle().
		Foreground(Cyan).
		PaddingLeft(2)

	// ThoughtAnalysisStyle is for analysis thoughts
	ThoughtAnalysisStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		PaddingLeft(2)

	// ThoughtSuggestionStyle is for suggestion thoughts
	ThoughtSuggestionStyle = lipgloss.NewStyle().
		Foreground(Green).
		PaddingLeft(2)

	// ThoughtBulletStyle is for the bullet point
	ThoughtBulletStyle = lipgloss.NewStyle().
		Foreground(Cyan)

	// FileReferenceStyle is for file references
	FileReferenceStyle = lipgloss.NewStyle().
		Foreground(Yellow).
		Italic(true)

	// CommentStyle is for displaying CodeRabbit comments
	CommentStyle = lipgloss.NewStyle().
		Foreground(Magenta).
		PaddingLeft(2)
)

// Progress bar styles
var (
	// ProgressFilledStyle is for the filled portion
	ProgressFilledStyle = lipgloss.NewStyle().
		Foreground(Green)

	// ProgressEmptyStyle is for the empty portion
	ProgressEmptyStyle = lipgloss.NewStyle().
		Foreground(DimGray)
)

// Help styles
var (
	// HelpKeyStyle is for key bindings
	HelpKeyStyle = lipgloss.NewStyle().
		Foreground(Cyan).
		Bold(true)

	// HelpDescStyle is for key descriptions
	HelpDescStyle = lipgloss.NewStyle().
		Foreground(DimGray)

	// HelpStyle is the overall help section style
	HelpStyle = lipgloss.NewStyle().
		Foreground(DimGray).
		PaddingTop(1)
)

// Box styles
var (
	// BorderStyle is for bordered boxes
	BorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Gray).
		Padding(0, 1)

	// ActiveBorderStyle is for focused/active boxes
	ActiveBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Blue).
		Padding(0, 1)
)

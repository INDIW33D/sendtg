package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#10B981")
	errorColor     = lipgloss.Color("#EF4444")
	mutedColor     = lipgloss.Color("#6B7280")
	textColor      = lipgloss.Color("#F3F4F6")

	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// Subtitle style
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginBottom(1)

	// Input label style
	LabelStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Bold(true)

	// Success message style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	// Error message style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Help style
	HelpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// Selected item style
	SelectedStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Normal item style
	NormalStyle = lipgloss.NewStyle().
			Foreground(textColor)

	// Box style for containers
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	// Spinner style
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	// File info style
	FileInfoStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true)

	// Tab styles
	TabStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(0, 1)

	TabActiveStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1).
			Bold(true)

	// Tab separator
	TabSeparator = lipgloss.NewStyle().
			Foreground(mutedColor).
			SetString("│")

	// Search style
	SearchStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)
)

package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorPrimary  = lipgloss.Color("#7C9EF0")
	ColorAccent   = lipgloss.Color("#F0A47C")
	ColorMuted    = lipgloss.Color("#666688")
	ColorError    = lipgloss.Color("#F07C7C")
	ColorSuccess  = lipgloss.Color("#7CF09C")
	ColorBorder   = lipgloss.Color("#444466")
	ColorSelected = lipgloss.Color("#2A2A4A")
	ColorHeader   = lipgloss.Color("#EEEEFF")
	ColorBrand1   = lipgloss.Color("#FFFFFF")
	ColorBrand2   = lipgloss.Color("#5865F2")

	StyleNormal  = lipgloss.NewStyle()
	StylePrimary = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)
	StyleAccent = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)
	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)
	StyleError = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)
	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)
	StyleSelected = lipgloss.NewStyle().
			Background(ColorSelected).
			Foreground(ColorHeader).
			Bold(true)
	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)
	StyleInputPrompt = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true)
	StyleStatusKey = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)
)

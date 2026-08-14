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
	StyleSelector = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
)

// ConfigureTheme applies a complete semantic palette. Terminal mode omits
// explicit colors so the terminal's normal foreground and background inherit.
func ConfigureTheme(colors map[string]string, terminal bool) {
	ColorPrimary = themedColor(colors, terminal, "primary", "#7C9EF0")
	ColorAccent = themedColor(colors, terminal, "accent", "#F0A47C")
	ColorMuted = themedColor(colors, terminal, "muted", "#666688")
	ColorError = themedColor(colors, terminal, "error", "#F07C7C")
	ColorSuccess = themedColor(colors, terminal, "success", "#7CF09C")
	ColorBorder = themedColor(colors, terminal, "border", "#444466")
	ColorSelected = themedColor(colors, terminal, "selected_background", "#2A2A4A")
	ColorHeader = themedColor(colors, terminal, "selected_foreground", "#EEEEFF")
	ColorBrand1 = themedColor(colors, terminal, "brand_primary", "#FFFFFF")
	ColorBrand2 = themedColor(colors, terminal, "brand_secondary", "#5865F2")

	StyleNormal = themedStyle(lipgloss.NewStyle(), colors, terminal, "foreground", "")
	StylePrimary = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "primary", "#7C9EF0")
	StyleAccent = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "accent", "#F0A47C")
	StyleMuted = themedStyle(lipgloss.NewStyle(), colors, terminal, "muted", "#666688")
	StyleError = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "error", "#F07C7C")
	StyleSuccess = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "success", "#7CF09C")
	StyleSelected = themedBackground(themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "selected_foreground", "#EEEEFF"), colors, terminal, "selected_background", "#2A2A4A")
	StyleBorder = themedBorder(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()), colors, terminal, "border", "#444466")
	StyleInputPrompt = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "accent", "#F0A47C")
	StyleStatusKey = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "primary", "#7C9EF0")
	StyleSelector = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "selector", "#FFFFFF")
}

func themedColor(colors map[string]string, terminal bool, key, fallback string) lipgloss.Color {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return lipgloss.Color(value)
	}
	return lipgloss.Color("")
}

func themedStyle(style lipgloss.Style, colors map[string]string, terminal bool, key, fallback string) lipgloss.Style {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return style.Foreground(lipgloss.Color(value))
	}
	return style
}

func themedBackground(style lipgloss.Style, colors map[string]string, terminal bool, key, fallback string) lipgloss.Style {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return style.Background(lipgloss.Color(value))
	}
	return style
}

func themedBorder(style lipgloss.Style, colors map[string]string, terminal bool, key, fallback string) lipgloss.Style {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return style.BorderForeground(lipgloss.Color(value))
	}
	return style
}

func themedValue(colors map[string]string, terminal bool, key, fallback string) (string, bool) {
	if value := colors[key]; value != "" {
		return value, true
	}
	if terminal {
		return "", false
	}
	return fallback, fallback != ""
}

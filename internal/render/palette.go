package render

import "charm.land/lipgloss/v2"

// Brand palette. All styles derive from these constants.
const (
	Orange = "#F97316"
	Ink    = "#111827"
	White  = "#FFFFFF"
	Sand   = "#FED7AA"
)

var (
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(Orange))
	failStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(Orange)).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(Sand))
	skipStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(Sand))
	headingStyle = lipgloss.NewStyle().Bold(true)
	keyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(Sand))
)

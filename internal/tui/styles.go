package tui

import "github.com/charmbracelet/lipgloss"

// Shared styles for all TUI screens.
var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	labelStyle   = lipgloss.NewStyle().Bold(true)
	helpStyle    = lipgloss.NewStyle().Faint(true)
	cursorStyle  = lipgloss.NewStyle().Bold(true)
	errStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	selStyle     = lipgloss.NewStyle().Bold(true)
	headerStyle  = lipgloss.NewStyle().Bold(true).Underline(true)
	statusStyle  = lipgloss.NewStyle().Faint(true)
	posStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	negStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // red
	statBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)
)

// money renders a signed cash amount with +/- coloring.
func money(v float64) string {
	s := lipgloss.NewStyle()
	switch {
	case v > 0:
		s = posStyle
	case v < 0:
		s = negStyle
	}
	return s.Render(formatMoney(v))
}

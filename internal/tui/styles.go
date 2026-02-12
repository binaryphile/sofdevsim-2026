package tui

import (
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorSuccess   = lipgloss.Color("#10B981") // Green
	ColorWarning   = lipgloss.Color("#F59E0B") // Yellow
	ColorDanger    = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorBorder    = lipgloss.Color("#374151") // Dark gray
)

// Box styles
var (
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// Status styles
var (
	GreenStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	YellowStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	RedStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)
)

// Table styles
var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(ColorBorder)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))

	TableSelectedStyle = lipgloss.NewStyle().
				Background(ColorPrimary).
				Foreground(lipgloss.Color("#FFFFFF"))
)

// Progress bar styles
var (
	ProgressFilledStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	ProgressEmptyStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)
)

// Help styles
var (
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// RenderProgressBar creates a simple progress bar
func RenderProgressBar(percent float64, width int) string {
	filled := int(percent * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += ProgressFilledStyle.Render("█")
		} else {
			bar += ProgressEmptyStyle.Render("░")
		}
	}
	return bar
}

// FeverColor returns the appropriate style for a fever status
func FeverColor(pctUsed float64) lipgloss.Style {
	switch {
	case pctUsed < 33:
		return GreenStyle
	case pctUsed < 66:
		return YellowStyle
	default:
		return RedStyle
	}
}

// FeverLabel returns the label for a fever status
func FeverLabel(pctUsed float64) string {
	switch {
	case pctUsed < 33:
		return "GREEN"
	case pctUsed < 66:
		return "YELLOW"
	default:
		return "RED"
	}
}

// FeverEmoji returns the emoji for a FeverStatus
func FeverEmoji(status model.FeverStatus) string {
	switch status {
	case model.FeverGreen:
		return "🟢"
	case model.FeverYellow:
		return "🟡"
	default:
		return "🔴"
	}
}

// feverEmojiFromString returns emoji for string status (client mode)
func feverEmojiFromString(status string) string {
	switch status {
	case "Green":
		return "🟢"
	case "Yellow":
		return "🟡"
	default:
		return "🔴"
	}
}

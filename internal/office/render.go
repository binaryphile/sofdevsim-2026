package office

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/binaryphile/fluentfp/slice"
	"github.com/charmbracelet/lipgloss"
)

// Developer color palette (colorblind-friendly)
var DeveloperColors = []lipgloss.Color{
	lipgloss.Color("#3B82F6"), // Blue
	lipgloss.Color("#F97316"), // Orange
	lipgloss.Color("#D946EF"), // Magenta
	lipgloss.Color("#06B6D4"), // Cyan
	lipgloss.Color("#EAB308"), // Yellow
	lipgloss.Color("#22C55E"), // Green
}

// DeveloperColorNames maps color indices to human-readable names
var DeveloperColorNames = []string{
	"blue",
	"orange",
	"magenta",
	"cyan",
	"yellow",
	"green",
}

// Default developer names (diverse, inclusive)
var DefaultDeveloperNames = []string{
	"Mei",   // East Asian
	"Amir",  // Middle Eastern
	"Suki",  // Japanese
	"Jay",   // Gender-neutral English
	"Priya", // South Asian
	"Kofi",  // West African
}

// MutedStyle for error/fallback messages
var MutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

// ansiRegex matches ANSI escape sequences
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes ANSI escape codes from a string
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// Calculation: RenderDeveloperIcon returns the icon for a developer's current state
// Pure function: DeveloperAnimation → string
func RenderDeveloperIcon(anim DeveloperAnimation) string {
	var face string
	switch anim.State {
	case StateWorking:
		face = WorkingFrames[anim.CurrentFrame()]
	case StateFrustrated:
		face = FrustratedFrames[anim.CurrentFrame()]
	default:
		face = "🙂"
	}
	return face + anim.Accessory.Emoji()
}

// Calculation: RenderLateBubble returns a small "Late!" thought bubble
// Pure function: () → string
func RenderLateBubble() string {
	return "┌──────┐\n│Late! │\n└──┬───┘"
}


// Calculation: RenderCubicle renders a single cubicle with developer
// Pure function: (DeveloperAnimation, string, int) → string
func RenderCubicle(anim DeveloperAnimation, name string, width int) string {
	color := DeveloperColors[anim.ColorIndex%len(DeveloperColors)]
	style := lipgloss.NewStyle().Foreground(color)

	icon := RenderDeveloperIcon(anim)

	var lines []string

	// Show "Late!" bubble briefly when transitioning to frustrated
	if anim.LateBubbleFrames > 0 {
		bubble := RenderLateBubble()
		for _, line := range strings.Split(bubble, "\n") {
			lines = append(lines, style.Render(line))
		}
	}

	// Cubicle box
	topBorder := "┌" + strings.Repeat("─", width-2) + "┐"
	bottomBorder := "└" + strings.Repeat("─", width-2) + "┘"

	// Center name in cubicle (colored)
	nameLine := fmt.Sprintf("│%s│", centerText(style.Render(name), width-2))

	// Center icon in cubicle (emoji doesn't take ANSI color)
	iconLine := fmt.Sprintf("│%s│", centerText(icon, width-2))

	lines = append(lines, topBorder, nameLine, iconLine, bottomBorder)
	return strings.Join(lines, "\n")
}

// Calculation: RenderConferenceRoom renders the conference room with developers
// Pure function: ([]DeveloperAnimation, []string, int) → string
func RenderConferenceRoom(anims []DeveloperAnimation, names []string, width int) string {
	// renderWithColor renders a developer icon styled with their assigned color.
	renderWithColor := func(a DeveloperAnimation) string {
		color := DeveloperColors[a.ColorIndex%len(DeveloperColors)]
		return lipgloss.NewStyle().Foreground(color).Render(RenderDeveloperIcon(a))
	}
	icons := slice.From(anims).
		KeepIf(DeveloperAnimation.IsInConference).
		ToString(renderWithColor)

	iconLine := strings.Join(icons, "  ")

	var lines []string
	lines = append(lines, "┌"+strings.Repeat("─", width-2)+"┐")
	lines = append(lines, "│"+centerText("CONFERENCE ROOM", width-2)+"│")
	lines = append(lines, "│"+strings.Repeat(" ", width-2)+"│")
	lines = append(lines, "│"+centerText(iconLine, width-2)+"│")
	lines = append(lines, "│"+strings.Repeat(" ", width-2)+"│")
	lines = append(lines, "└"+strings.Repeat("─", width-2)+"┘")

	return strings.Join(lines, "\n")
}

// Calculation: RenderCubicleGrid renders a 2×3 grid of cubicles
// Pure function: (OfficeState, []string, int) → string
func RenderCubicleGrid(state OfficeState, names []string, width int) string {
	cubicleWidth := width / 3
	if cubicleWidth < 8 {
		cubicleWidth = 8
	}

	// Build 2 rows of 3 cubicles each
	var rows []string
	for row := 0; row < 2; row++ {
		var rowCubicles []string
		for col := 0; col < 3; col++ {
			idx := row*3 + col
			if idx >= len(state.Animations) {
				break
			}
			anim := state.Animations[idx]
			name := ""
			if idx < len(names) {
				name = names[idx]
			}
			// RenderCubicleCompact shows name always, icon only when not in conference
			cubicle := RenderCubicleCompact(anim, name, cubicleWidth)
			rowCubicles = append(rowCubicles, cubicle)
		}
		if len(rowCubicles) > 0 {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCubicles...))
		}
	}

	return strings.Join(rows, "\n")
}

// Calculation: RenderCubicleCompact renders a compact cubicle for grid layout
// Shows name always, icon only when developer is working (not in conference)
// Pure function: (DeveloperAnimation, string, int) → string
func RenderCubicleCompact(anim DeveloperAnimation, name string, width int) string {
	color := DeveloperColors[anim.ColorIndex%len(DeveloperColors)]
	style := lipgloss.NewStyle().Foreground(color)

	var lines []string

	// Show "Late!" bubble briefly when transitioning to frustrated
	if anim.LateBubbleFrames > 0 {
		bubble := RenderLateBubble()
		for _, line := range strings.Split(bubble, "\n") {
			lines = append(lines, style.Render(line))
		}
	}

	// Cubicle box
	innerWidth := width - 2
	topBorder := "┌" + strings.Repeat("─", innerWidth) + "┐"
	bottomBorder := "└" + strings.Repeat("─", innerWidth) + "┘"

	// Center name in cubicle (always shown)
	nameLine := "│" + centerText(style.Render(name), innerWidth) + "│"

	// Icon line: show icon unless in conference (dev is in conference room)
	var iconContent string
	if anim.State != StateConference {
		iconContent = RenderDeveloperIcon(anim)
	}
	iconLine := "│" + centerText(iconContent, innerWidth) + "│"

	lines = append(lines, topBorder, nameLine, iconLine, bottomBorder)
	return strings.Join(lines, "\n")
}

// Calculation: RenderOffice renders the complete office view
// Pure function: (OfficeState, []string, int, int) → string
func RenderOffice(state OfficeState, names []string, width, height int) string {
	if width < 40 {
		return MutedStyle.Render("Terminal too narrow for office view")
	}

	// Conference room at top
	conferenceWidth := width
	if conferenceWidth > 35 {
		conferenceWidth = 35
	}
	conference := RenderConferenceRoom(state.Animations, names, conferenceWidth)

	// Cubicle grid below
	cubicles := RenderCubicleGrid(state, names, width)

	return lipgloss.JoinVertical(lipgloss.Left, conference, cubicles)
}

// centerText centers text within a given width.
// Uses lipgloss.Width to handle ANSI-styled text correctly.
// Calculation: (string, int) → string
func centerText(text string, width int) string {
	textLen := lipgloss.Width(text) // handles ANSI escape codes
	if textLen >= width {
		return text // don't truncate styled text
	}
	padding := (width - textLen) / 2
	return strings.Repeat(" ", padding) + text + strings.Repeat(" ", width-padding-textLen)
}

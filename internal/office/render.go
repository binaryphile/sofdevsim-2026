package office

import (
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

// developerColor returns the color for a developer, handling negative indices safely.
func developerColor(colorIndex int) lipgloss.Color {
	n := len(DeveloperColors)
	idx := ((colorIndex % n) + n) % n // safe modulo for negative values
	return DeveloperColors[idx]
}

// ansiRegex matches ANSI escape sequences
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes ANSI escape codes from a string
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// Calculation: RenderDeveloperIcon returns the icon for a developer's current state
// Pure function: DeveloperAnimation → string
func RenderDeveloperIcon(anim DeveloperAnimation) string {
	// Handle sip animation (overrides normal face)
	if anim.SipPhase != SipNone && anim.Accessory != AccessoryNone {
		switch anim.SipPhase {
		case SipPreparing:
			return "😙" + anim.Accessory.Emoji()
		case SipDrinking:
			return anim.Accessory.Emoji()
		case SipRefreshed:
			return "😌" + anim.Accessory.Emoji()
		}
	}

	// Normal rendering
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

// renderFaceOnly returns just the face emoji without accessory.
// Used in cubicle view where accessory is shown on desk instead.
func renderFaceOnly(anim DeveloperAnimation) string {
	switch anim.State {
	case StateWorking:
		return WorkingFrames[anim.CurrentFrame()]
	case StateFrustrated:
		return FrustratedFrames[anim.CurrentFrame()]
	default:
		return "🙂"
	}
}

// Calculation: RenderLateBubble returns a small "Late!" thought bubble
// Pure function: () → string
func RenderLateBubble() string {
	return "┌──────┐\n│Late! │\n└──┬───┘"
}

// RenderLateBubbleInline returns a compact inline bubble for overlay rendering.
// Uses distinct characters (rounded corners) to stand out from cubicle walls.
func RenderLateBubbleInline() string {
	return "╭Late!╮"
}


// Calculation: RenderCubicle renders a single cubicle with developer
// Pure function: (DeveloperAnimation, string, int) → string
func RenderCubicle(anim DeveloperAnimation, name string, width int) string {
	color := developerColor(anim.ColorIndex)
	style := lipgloss.NewStyle().Foreground(color)

	innerWidth := width - 2

	// Cubicle box
	topBorder := "┌" + strings.Repeat("─", innerWidth) + "┐"
	bottomBorder := "└" + strings.Repeat("─", innerWidth) + "┘"

	// Name line - shows bubble overlay when frustrated, otherwise truncated name
	var nameContent string
	if anim.LateBubbleFrames > 0 {
		nameContent = style.Render(RenderLateBubbleInline())
	} else {
		displayName := truncateText(name, innerWidth-2)
		nameContent = style.Render(displayName)
	}
	nameLine := "│" + centerText(nameContent, innerWidth) + "│"

	// Center icon in cubicle (empty when dev is away)
	var icon string
	if !anim.IsAway() {
		icon = RenderDeveloperIcon(anim)
	}
	iconLine := "│" + centerText(icon, innerWidth) + "│"

	lines := []string{topBorder, nameLine, iconLine, bottomBorder}
	return strings.Join(lines, "\n")
}

// Calculation: RenderConferenceRoom renders the conference room with developers
// Pure function: ([]DeveloperAnimation, []string, int) → string
func RenderConferenceRoom(anims []DeveloperAnimation, names []string, width int) string {
	// renderWithColor renders a developer icon styled with their assigned color.
	renderWithColor := func(a DeveloperAnimation) string {
		color := developerColor(a.ColorIndex)
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
	color := developerColor(anim.ColorIndex)
	style := lipgloss.NewStyle().Foreground(color)

	// Cubicle box
	innerWidth := width - 2
	topBorder := "┌" + strings.Repeat("─", innerWidth) + "┐"
	bottomBorder := "└" + strings.Repeat("─", innerWidth) + "┘"

	// Name line - shows bubble overlay when frustrated, otherwise truncated name
	var nameContent string
	if anim.LateBubbleFrames > 0 {
		nameContent = style.Render(RenderLateBubbleInline())
	} else {
		displayName := truncateText(name, innerWidth-2)
		nameContent = style.Render(displayName)
	}
	nameLine := "│" + centerText(nameContent, innerWidth) + "│"

	// Icon line: show icon unless away (in conference or moving)
	var iconContent string
	if !anim.IsAway() {
		iconContent = RenderDeveloperIcon(anim)
	}
	iconLine := "│" + centerText(iconContent, innerWidth) + "│"

	lines := []string{topBorder, nameLine, iconLine, bottomBorder}
	return strings.Join(lines, "\n")
}

// Calculation: RenderOffice renders the complete office view
// Width >= 80: enhanced layout with detailed elements
// Width < 80: simple vertical layout
// Pure function: (OfficeState, []string, int, int) → string
func RenderOffice(state OfficeState, names []string, width, height int) string {
	if width < 40 {
		return MutedStyle.Render("Terminal too narrow for office view")
	}

	if width >= 80 {
		return renderOfficeEnhanced(state, names, width, height)
	}

	return renderOfficeSimple(state, names, width, height)
}

// renderOfficeSimple renders the simple vertical layout (original implementation).
func renderOfficeSimple(state OfficeState, names []string, width, height int) string {
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

// renderOfficeEnhanced renders the detailed side-by-side layout.
// Conference room on left, cubicle grid on right, outer frame around everything.
func renderOfficeEnhanced(state OfficeState, names []string, width, height int) string {
	// Calculate widths
	conferenceWidth := 27
	cubicleGridWidth := width - conferenceWidth - 5 // 5 for spacing and frame

	// Render conference room
	conference := renderConferenceRoomDetailed(state.Animations, conferenceWidth)

	// Render cubicle grid with hallway
	cubicles := renderCubicleGridDetailed(state, names, cubicleGridWidth)

	// Join conference and cubicles side by side
	content := lipgloss.JoinHorizontal(lipgloss.Top, conference, "  ", cubicles)

	// Wrap in outer frame using lipgloss border
	frameStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#666666")).
		Padding(0, 1)

	return frameStyle.Render(content)
}

// renderConferenceRoomDetailed renders the conference room with charts, table, chairs, door.
// Developers are positioned around the table in 3×2 seating.
// Calculation: ([]DeveloperAnimation, int) → string
func renderConferenceRoomDetailed(anims []DeveloperAnimation, width int) string {
	innerWidth := width - 2

	// Filter to devs in conference
	var conferenceDevs []DeveloperAnimation
	for _, a := range anims {
		if a.IsInConference() {
			conferenceDevs = append(conferenceDevs, a)
		}
	}

	// Render dev icons with colors
	renderIcon := func(idx int) string {
		if idx >= len(conferenceDevs) {
			return "🪑" // empty chair
		}
		a := conferenceDevs[idx]
		color := developerColor(a.ColorIndex)
		return lipgloss.NewStyle().Foreground(color).Render(RenderDeveloperIcon(a))
	}

	// Build lines
	topBorder := "┌" + strings.Repeat("─", innerWidth) + "┐"
	bottomBorder := "└" + strings.Repeat("─", innerWidth) + "┘"

	// Charts row
	chartsContent := "📊 📈 📉"
	chartsLine := "│" + centerText(chartsContent, innerWidth) + "│"

	// Empty row
	emptyLine := "│" + strings.Repeat(" ", innerWidth) + "│"

	// Top seating row (devs 0, 1, 2 or chairs)
	topSeating := renderIcon(0) + "  " + renderIcon(1) + "  " + renderIcon(2)
	topSeatingLine := "│" + centerText(topSeating, innerWidth) + "│"

	// Table row
	tableLine := "│" + centerText("╔══════╗", innerWidth) + "│"

	// Bottom seating row (devs 3, 4, 5 or chairs) + door
	bottomSeating := renderIcon(3) + " ╚══════╝ " + renderIcon(4) + "  " + renderIcon(5)
	// Put door at the end
	bottomSeatingWithDoor := bottomSeating + " 🚪"
	bottomSeatingLine := "│" + padRight(bottomSeatingWithDoor, innerWidth) + "│"

	lines := []string{
		topBorder,
		chartsLine,
		emptyLine,
		topSeatingLine,
		tableLine,
		bottomSeatingLine,
		emptyLine,
		bottomBorder,
	}

	return strings.Join(lines, "\n")
}

// renderCubicleGridDetailed renders a 2×3 grid of detailed cubicles with hallway.
// Row 0: devs 0-2 with doors on bottom (facing hallway)
// HALLWAY label
// Row 1: devs 3-5 with doors on top (facing hallway)
// Calculation: (OfficeState, []string, int) → string
func renderCubicleGridDetailed(state OfficeState, names []string, width int) string {
	cubicleWidth := width / 3
	if cubicleWidth < 9 {
		cubicleWidth = 9
	}

	// Build row 0 (devs 0-2, doors on bottom)
	var row0Cubicles []string
	for col := 0; col < 3; col++ {
		idx := col
		if idx >= len(state.Animations) {
			break
		}
		anim := state.Animations[idx]
		name := ""
		if idx < len(names) {
			name = names[idx]
		}
		cubicle := renderCubicleDetailed(anim, name, cubicleWidth, false) // doorOnTop=false
		row0Cubicles = append(row0Cubicles, cubicle)
	}
	row0 := lipgloss.JoinHorizontal(lipgloss.Top, row0Cubicles...)

	// Hallway label
	hallwayWidth := width
	hallway := centerText("HALLWAY", hallwayWidth)

	// Build row 1 (devs 3-5, doors on top)
	var row1Cubicles []string
	for col := 0; col < 3; col++ {
		idx := 3 + col
		if idx >= len(state.Animations) {
			break
		}
		anim := state.Animations[idx]
		name := ""
		if idx < len(names) {
			name = names[idx]
		}
		cubicle := renderCubicleDetailed(anim, name, cubicleWidth, true) // doorOnTop=true
		row1Cubicles = append(row1Cubicles, cubicle)
	}
	row1 := lipgloss.JoinHorizontal(lipgloss.Top, row1Cubicles...)

	return lipgloss.JoinVertical(lipgloss.Left, row0, hallway, row1)
}

// padRight pads a string to the given width on the right.
func padRight(text string, width int) string {
	textLen := lipgloss.Width(text)
	if textLen >= width {
		return text
	}
	return text + strings.Repeat(" ", width-textLen)
}

// renderCubicleDetailed renders a detailed cubicle with desk, trash, door.
// doorOnTop: true for row 1 (door faces hallway above), false for row 0 (door faces hallway below).
// Calculation: (DeveloperAnimation, string, int, bool) → string
func renderCubicleDetailed(anim DeveloperAnimation, name string, width int, doorOnTop bool) string {
	color := developerColor(anim.ColorIndex)
	style := lipgloss.NewStyle().Foreground(color)

	innerWidth := width - 2
	topBorder := "┌" + strings.Repeat("─", innerWidth) + "┐"
	bottomBorder := "└" + strings.Repeat("─", innerWidth) + "┘"
	doorBorder := "└" + centerText("🚪", innerWidth) + "┘"
	doorBorderTop := "┌" + centerText("🚪", innerWidth) + "┐"

	// Name line - shows bubble overlay when frustrated, otherwise name
	// Truncate long names to fit cubicle width
	var nameContent string
	if anim.LateBubbleFrames > 0 {
		nameContent = style.Render(RenderLateBubbleInline())
	} else {
		displayName := truncateText(name, innerWidth-2)
		nameContent = style.Render(displayName)
	}
	nameLine := "│" + centerText(nameContent, innerWidth) + "│"

	// Face + trash line (face only if dev is in cubicle)
	var faceContent string
	if !anim.IsAway() {
		face := renderFaceOnly(anim)
		faceContent = face + " 🗑️"
	} else {
		faceContent = "🗑️" // just trash when dev is away
	}
	faceLine := "│" + centerText(faceContent, innerWidth) + "│"

	// Desk line (accessory on desk only when dev is in cubicle)
	var deskContent string
	if !anim.IsAway() && anim.Accessory != AccessoryNone {
		deskContent = "🖥️ " + anim.Accessory.Emoji()
	} else {
		deskContent = "🖥️"
	}
	deskLine := "│" + centerText(deskContent, innerWidth) + "│"

	var lines []string
	if doorOnTop {
		// Row 1: door on top (facing hallway above)
		lines = []string{doorBorderTop, nameLine, faceLine, deskLine, bottomBorder}
	} else {
		// Row 0: door on bottom (facing hallway below)
		lines = []string{topBorder, nameLine, faceLine, deskLine, doorBorder}
	}

	return strings.Join(lines, "\n")
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

// truncateText truncates plain text to fit within maxWidth, adding "…" if truncated.
// Uses rune-aware truncation to handle Unicode correctly.
func truncateText(text string, maxWidth int) string {
	if lipgloss.Width(text) <= maxWidth {
		return text
	}
	// Truncate rune by rune until it fits with ellipsis
	runes := []rune(text)
	for i := len(runes) - 1; i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return "…"
}

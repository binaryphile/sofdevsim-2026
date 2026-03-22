package office

import (
	"regexp"
	"sort"
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
	"MsPac",  // Ms. Pac-Man (1982, Midway)
	"Qbert",  // Q*bert (1982, Gottlieb)
	"Samus",  // Metroid (1986, Nintendo)
	"Athena", // Athena (1986, SNK)
	"Mappy",  // Mappy (1983, Namco)
	"Pengo",  // Pengo (1982, Sega)
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

// RenderLateBubbleInline returns a compact inline speech indicator.
// Text stands alone with a single arch connector pointing down to the speaker.
func RenderLateBubbleInline() string {
	return "╭Late!"
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
	for row := 0; row < 2; row++ { // justified:IX
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

	// Render dev icons at fixed seat positions. Devs keep their assigned seat;
	// absent devs show a chair instead of shifting everyone down.
	const seatWidth = 4
	renderIcon := func(idx int) string {
		if idx >= len(anims) || !anims[idx].IsInConference() {
			return padRight("🪑", seatWidth)
		}
		a := anims[idx]
		color := developerColor(a.ColorIndex)
		icon := lipgloss.NewStyle().Foreground(color).Render(RenderDeveloperIcon(a))
		return padRight(icon, seatWidth)
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

	// TT-shaped table: overhanging tabletop with legs below
	tableLine := "│" + centerText("══╦═══════╦══", innerWidth) + "│"

	// Table legs: ║ aligned with ╦ connectors
	// ╦ positions within "══╦═══════╦══" (13 chars): ╦ at offset 2 and 10
	tableStr := "══╦═══════╦══"
	tableWidth := lipgloss.Width(tableStr)
	tablePad := (innerWidth - tableWidth) / 2
	legsContent := strings.Repeat(" ", tablePad+2) + "║" + strings.Repeat(" ", 7) + "║"
	legsLine := "│" + padRight(legsContent, innerWidth) + "┤" // post above opening

	// Bottom seating row (devs 3, 4, 5 or chairs) — opening in right wall
	bottomSeating := renderIcon(3) + "  " + renderIcon(4) + "  " + renderIcon(5)
	bottomSeatingLine := "│" + centerText(bottomSeating, innerWidth) + " "

	// Trash + post below opening (bottom-left corner, away from door and charts)
	trashPostLine := "│🗑\uFE0F" + strings.Repeat(" ", innerWidth-2) + "┤"

	lines := []string{
		topBorder,
		chartsLine,
		emptyLine,
		topSeatingLine,
		tableLine,
		legsLine,
		bottomSeatingLine,
		trashPostLine,
		bottomBorder,
	}

	return strings.Join(lines, "\n")
}

// renderCubicleGridDetailed renders a 2×3 grid of detailed cubicles with hallway.
// Row 0: devs 0-2 with doors on bottom (facing hallway)
// Hallway between cubicle rows (implicit — no label)
// Row 1: devs 3-5 with doors on top (facing hallway)
// Calculation: (OfficeState, []string, int) → string
func renderCubicleGridDetailed(state OfficeState, names []string, width int) string {
	cubicleWidth := width / 3
	if cubicleWidth < 9 {
		cubicleWidth = 9
	}

	// Build row 0 (devs 0-2, doors on bottom)
	var row0Cubicles []string
	for col := 0; col < 3; col++ { // justified:IX
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

	// 3-line hallway between cubicle rows
	hallwayWidth := width

	// Collect walking devs for hallway rendering
	type walker struct {
		pos  int    // display-width x position
		icon string // colored face+accessory (ANSI-wrapped)
	}
	var walkers []walker
	for i, anim := range state.Animations { // justified:CF
		if anim.State == StateMovingToConference {
			col := i % 3
			startX := col*cubicleWidth + cubicleWidth/2
			currentX := startX - int(float64(startX)*anim.Progress)
			color := developerColor(anim.ColorIndex)
			icon := lipgloss.NewStyle().Foreground(color).Render(RenderDeveloperIcon(anim))
			walkers = append(walkers, walker{pos: currentX, icon: icon})
		} else if anim.State == StateMovingToCubicle {
			col := i % 3
			endX := col*cubicleWidth + cubicleWidth/2
			currentX := int(float64(endX) * anim.Progress)
			color := developerColor(anim.ColorIndex)
			icon := lipgloss.NewStyle().Foreground(color).Render(RenderDeveloperIcon(anim))
			walkers = append(walkers, walker{pos: currentX, icon: icon})
		}
	}

	// Check for frustrated row-1 dev with active Late! bubble
	var bubbleCol int = -1
	for col := 0; col < 3; col++ { // justified:IX
		idx := 3 + col
		if idx < len(state.Animations) && state.Animations[idx].LateBubbleFrames > 0 {
			bubbleCol = col
			break
		}
	}

	var hallway string
	if len(walkers) > 0 {
		// Render walking devs on middle hallway line
		sort.Slice(walkers, func(i, j int) bool { return walkers[i].pos < walkers[j].pos })
		var parts []string
		cursor := 0
		for _, w := range walkers { // justified:CF
			if w.pos > cursor {
				parts = append(parts, strings.Repeat(" ", w.pos-cursor))
				cursor = w.pos
			}
			parts = append(parts, w.icon)
			cursor += lipgloss.Width(w.icon)
		}
		if cursor < hallwayWidth {
			parts = append(parts, strings.Repeat(" ", hallwayWidth-cursor))
		}
		middleLine := strings.Join(parts, "")
		emptyLine := strings.Repeat(" ", hallwayWidth)
		hallway = strings.Join([]string{emptyLine, middleLine, emptyLine}, "\n")
	} else if bubbleCol >= 0 {
		// Render Late! bubble in hallway, positioned above the frustrated dev's column
		bubbleLines := strings.Split(RenderLateBubble(), "\n") // 3 lines: top, middle, bottom
		offset := bubbleCol * cubicleWidth
		var hallwayLines []string
		for _, bLine := range bubbleLines {
			line := strings.Repeat(" ", offset) + bLine
			hallwayLines = append(hallwayLines, padRight(line, hallwayWidth))
		}
		hallway = strings.Join(hallwayLines, "\n")
	} else {
		emptyHallway := strings.Repeat(" ", hallwayWidth)
		hallway = strings.Join([]string{emptyHallway, emptyHallway, emptyHallway}, "\n")
	}

	// Build row 1 (devs 3-5, doors on top)
	var row1Cubicles []string
	for col := 0; col < 3; col++ { // justified:IX
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

	// Entryway borders with embedded name: └── Mei ┤ ├────────┘
	entryWidth := lipgloss.Width("┤ ├") // 3 display chars
	leftDash := (innerWidth - entryWidth) / 2
	rightDash := innerWidth - entryWidth - leftDash

	// Name embedded in door border, right of entryway: └────┤ ├ Mei ────┘
	maxNameW := rightDash - 1 // room for " name" on right of entryway
	var doorBorder, doorBorderTop string
	if maxNameW >= 1 && name != "" {
		displayName := truncateText(name, maxNameW)
		coloredName := style.Render(displayName)
		nameW := lipgloss.Width(displayName)
		trailingDash := rightDash - nameW - 1
		if trailingDash < 0 {
			trailingDash = 0
		}
		leftSection := strings.Repeat("─", leftDash) + "┤ ├"
		rightSection := " " + coloredName + strings.Repeat("─", trailingDash)
		doorBorder = "└" + leftSection + rightSection + "┘"
		doorBorderTop = "┌" + leftSection + rightSection + "┐"
	} else {
		doorBorder = "└" + strings.Repeat("─", leftDash) + "┤ ├" + strings.Repeat("─", rightDash) + "┘"
		doorBorderTop = "┌" + strings.Repeat("─", leftDash) + "┤ ├" + strings.Repeat("─", rightDash) + "┐"
	}

	// Face line: dev face when present, chair when away, LateBubble when frustrated
	var faceContent string
	if anim.LateBubbleFrames > 0 {
		faceContent = style.Render(RenderLateBubbleInline())
	} else if !anim.IsAway() {
		faceContent = renderFaceOnly(anim)
	} else {
		faceContent = "🪑"
	}

	// Monitor content (always on back wall)
	// No VS16 (\uFE0F) on monitor — causes duplicate glyph in some terminals.
	// Pad to consistent width when dev is away so centering doesn't shift.
	var monitorContent string
	if anim.Accessory != AccessoryNone {
		if !anim.IsAway() {
			monitorContent = "🖥 " + anim.Accessory.Emoji()
		} else {
			monitorContent = "🖥   " // same width as "🖥 ☕"
		}
	} else {
		monitorContent = "🖥"
	}

	// All developers centered in their cubicles.

	// Trash corner placement per developer.
	// 0=top-left, 1=top-right, 2=bottom-left, 3=bottom-right
	// Chosen to avoid right-then-left mirroring at shared cubicle walls.
	trashCorners := []int{0, 2, 1, 3, 0, 1}
	trashCorner := trashCorners[anim.ColorIndex%len(trashCorners)]
	trashOnLeft := trashCorner%2 == 0
	trashOnBackWall := (trashCorner <= 1) != doorOnTop

	// Center both lines in full width, then overlay trash at edge.
	// Keeps chair aligned with monitor regardless of trash placement.
	centeredMonitor := centerText(monitorContent, innerWidth)
	centeredFace := centerText(faceContent, innerWidth)

	var backWallLine, faceLine string
	if trashOnBackWall && trashOnLeft {
		backWallLine = "│🗑\uFE0F" + centeredMonitor[2:] + "│"
	} else if trashOnBackWall {
		backWallLine = "│" + centeredMonitor[:len(centeredMonitor)-3] + "🗑\uFE0F │"
	} else {
		backWallLine = "│" + centeredMonitor + "│"
	}

	if !trashOnBackWall && trashOnLeft {
		faceLine = "│🗑\uFE0F" + centeredFace[2:] + "│"
	} else if !trashOnBackWall {
		faceLine = "│" + centeredFace[:len(centeredFace)-3] + "🗑\uFE0F │"
	} else {
		faceLine = "│" + centeredFace + "│"
	}

	emptyLine := "│" + strings.Repeat(" ", innerWidth) + "│"

	var lines []string
	if doorOnTop {
		// Row 1: door on top → desk against far wall, face in front of desk
		lines = []string{doorBorderTop, emptyLine, emptyLine, faceLine, backWallLine, bottomBorder}
	} else {
		// Row 0: door on bottom → desk against far wall, face in front of desk
		lines = []string{topBorder, backWallLine, faceLine, emptyLine, emptyLine, doorBorder}
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

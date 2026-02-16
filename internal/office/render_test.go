package office

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// containsFaceEmoji returns true if the string contains any developer face emoji.
func containsFaceEmoji(s string) bool {
	for _, face := range WorkingFrames { // justified:CF
		if strings.Contains(s, face) {
			return true
		}
	}
	for _, face := range FrustratedFrames { // justified:CF
		if strings.Contains(s, face) {
			return true
		}
	}
	// Default neutral face (used for idle/conference)
	return strings.Contains(s, "🙂")
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello", "hello"},
		{"single color", "\x1b[31mred\x1b[0m", "red"},
		{"multiple codes", "\x1b[1m\x1b[32mbold green\x1b[0m", "bold green"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.input)
			if got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderDeveloperIcon(t *testing.T) {
	tests := []struct {
		name string
		anim DeveloperAnimation
		want string
	}{
		{"idle", DeveloperAnimation{State: StateIdle}, "🙂"},
		{"conference", DeveloperAnimation{State: StateConference}, "🙂"},
		{"working frame 0", DeveloperAnimation{State: StateWorking, Frame: 0}, "😊"},
		{"working frame 2", DeveloperAnimation{State: StateWorking, Frame: 2}, "😁"},
		{"frustrated frame 1", DeveloperAnimation{State: StateFrustrated, Frame: 1}, "😠"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderDeveloperIcon(tt.anim)
			if got != tt.want {
				t.Errorf("RenderDeveloperIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderDeveloperIcon_WithAccessory(t *testing.T) {
	tests := []struct {
		name       string
		accessory  Accessory
		state      AnimationState
		wantSuffix string
	}{
		{"coffee working", AccessoryCoffee, StateWorking, "☕"},
		{"soda idle", AccessorySoda, StateIdle, "🥤"},
		{"none working", AccessoryNone, StateWorking, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := DeveloperAnimation{State: tt.state, Accessory: tt.accessory}
			got := RenderDeveloperIcon(anim)
			if tt.wantSuffix != "" && !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("RenderDeveloperIcon() = %q, want suffix %q", got, tt.wantSuffix)
			}
			if tt.wantSuffix == "" && (strings.Contains(got, "☕") || strings.Contains(got, "🥤")) {
				t.Errorf("RenderDeveloperIcon() = %q, want no accessory", got)
			}
		})
	}
}

func TestRenderDeveloperIcon_SipPhases(t *testing.T) {
	tests := []struct {
		name      string
		sipPhase  SipPhase
		accessory Accessory
		want      string
	}{
		{"preparing coffee", SipPreparing, AccessoryCoffee, "😙☕"},
		{"drinking coffee", SipDrinking, AccessoryCoffee, "☕"},
		{"refreshed coffee", SipRefreshed, AccessoryCoffee, "😌☕"},
		{"preparing soda", SipPreparing, AccessorySoda, "😙🥤"},
		{"drinking soda", SipDrinking, AccessorySoda, "🥤"},
		{"refreshed soda", SipRefreshed, AccessorySoda, "😌🥤"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := DeveloperAnimation{
				State:     StateConference,
				Accessory: tt.accessory,
				SipPhase:  tt.sipPhase,
			}
			got := RenderDeveloperIcon(anim)
			if got != tt.want {
				t.Errorf("RenderDeveloperIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderCubicleCompact_LateBubble(t *testing.T) {
	// Use BecomeFrustrated to get the Late bubble
	anim := NewDeveloperAnimation("dev-1", 0).BecomeFrustrated()

	cubicle := RenderCubicleCompact(anim, "MsPac", 10)
	plain := StripANSI(cubicle)

	if !strings.Contains(plain, "Late!") {
		t.Error("Compact cubicle should show Late! bubble when LateBubbleFrames > 0")
	}

	// After countdown, bubble should disappear
	for i := 0; i < 10; i++ { // justified:IX
		anim = anim.NextFrame()
	}
	cubicle = RenderCubicleCompact(anim, "MsPac", 10)
	plain = StripANSI(cubicle)

	if strings.Contains(plain, "Late!") {
		t.Error("Late! bubble should disappear after countdown")
	}
}

func TestRenderConferenceRoom(t *testing.T) {
	anims := []DeveloperAnimation{
		{DevID: "dev-1", State: StateConference, ColorIndex: 0},
		{DevID: "dev-2", State: StateConference, ColorIndex: 1},
		{DevID: "dev-3", State: StateWorking, ColorIndex: 2}, // not in conference
	}
	names := []string{"MsPac", "Qbert", "Samus"}

	room := RenderConferenceRoom(anims, names, 35)
	plain := StripANSI(room)

	if !strings.Contains(plain, "CONFERENCE ROOM") {
		t.Error("Should contain title")
	}
	// Should show 2 neutral faces (dev-1 and dev-2 are in conference)
	iconCount := strings.Count(plain, "🙂")
	if iconCount != 2 {
		t.Errorf("Should show 2 conference icons (🙂), got %d", iconCount)
	}
}

func TestRenderOffice_NarrowTerminal(t *testing.T) {
	state := NewOfficeState([]string{"dev-1"})
	names := []string{"MsPac"}

	result := RenderOffice(state, names, 30, 24)
	plain := StripANSI(result)

	if !strings.Contains(plain, "too narrow") {
		t.Error("Should show 'too narrow' message for width < 40")
	}
}

func TestRenderCubicleGrid(t *testing.T) {
	devIDs := []string{"dev-1", "dev-2", "dev-3", "dev-4", "dev-5", "dev-6"}
	state := NewOfficeState(devIDs)
	names := DefaultDeveloperNames

	grid := RenderCubicleGrid(state, names, 60)
	plain := StripANSI(grid)

	// Should contain all 6 developer names
	for _, name := range names { // justified:AS
		if !strings.Contains(plain, name) {
			t.Errorf("Grid should contain %s", name)
		}
	}
}

func TestCenterText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  int // expected total length
	}{
		{"short text", "hi", 10, 10},
		{"exact width", "hello", 5, 5},
		{"text too long", "hello world", 5, 11}, // don't truncate
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := centerText(tt.text, tt.width)
			if len(got) != tt.want {
				t.Errorf("centerText(%q, %d) len = %d, want %d", tt.text, tt.width, len(got), tt.want)
			}
		})
	}
}

// TestCubicleFaceVisibility verifies the invariant: face visible iff !IsAway()
// across all three cubicle renderers and all animation states.
func TestCubicleFaceVisibility(t *testing.T) {
	allStates := []struct {
		state AnimationState
		name  string
	}{
		{StateIdle, "idle"},
		{StateWorking, "working"},
		{StateFrustrated, "frustrated"},
		{StateConference, "conference"},
		{StateMovingToConference, "movingToConference"},
		{StateMovingToCubicle, "movingToCubicle"},
	}

	renderers := []struct {
		name   string
		render func(DeveloperAnimation) string
	}{
		{"RenderCubicle", func(a DeveloperAnimation) string {
			return StripANSI(RenderCubicle(a, "MsPac", 12))
		}},
		{"RenderCubicleCompact", func(a DeveloperAnimation) string {
			return StripANSI(RenderCubicleCompact(a, "MsPac", 10))
		}},
		{"renderCubicleDetailed", func(a DeveloperAnimation) string {
			return StripANSI(renderCubicleDetailed(a, "MsPac", 9, false))
		}},
	}

	for _, r := range renderers { // justified:SM
		for _, s := range allStates {
			t.Run(r.name+"/"+s.name, func(t *testing.T) {
				anim := DeveloperAnimation{State: s.state, ColorIndex: 0}
				result := r.render(anim)
				hasFace := containsFaceEmoji(result)
				wantFace := !anim.IsAway()
				if hasFace != wantFace {
					t.Errorf("face visible = %v, want %v\n%s", hasFace, wantFace, result)
				}
			})
		}
	}
}

func TestRenderCubicle_LateBubble(t *testing.T) {
	// Use BecomeFrustrated to get the Late bubble
	anim := NewDeveloperAnimation("dev-1", 0).BecomeFrustrated()

	cubicle := RenderCubicle(anim, "MsPac", 12)
	plain := StripANSI(cubicle)

	if !strings.Contains(plain, "Late!") {
		t.Error("Should show Late! bubble when LateBubbleFrames > 0")
	}

	// After 10 frames, bubble should disappear
	for i := 0; i < 10; i++ { // justified:IX
		anim = anim.NextFrame()
	}
	cubicle = RenderCubicle(anim, "MsPac", 12)
	plain = StripANSI(cubicle)

	if strings.Contains(plain, "Late!") {
		t.Error("Late! bubble should disappear after countdown")
	}
}

func TestRenderOffice_ValidWidth(t *testing.T) {
	devIDs := []string{"dev-1", "dev-2", "dev-3", "dev-4", "dev-5", "dev-6"}
	state := NewOfficeState(devIDs)
	names := DefaultDeveloperNames

	result := RenderOffice(state, names, 60, 24)
	plain := StripANSI(result)

	if !strings.Contains(plain, "CONFERENCE ROOM") {
		t.Error("Should contain conference room")
	}
	for _, name := range names { // justified:AS
		if !strings.Contains(plain, name) {
			t.Errorf("Should contain developer name %s", name)
		}
	}
}

// Cycle 1: Detailed cubicle with desk, trash, door
func TestRenderCubicleDetailed(t *testing.T) {
	tests := []struct {
		name      string
		anim      DeveloperAnimation
		devName   string
		doorOnTop bool
		wantIn    []string
	}{
		{
			"working dev, door on bottom",
			DeveloperAnimation{State: StateWorking, ColorIndex: 0},
			"MsPac",
			false,
			[]string{"MsPac", "🖥", "🗑", "┤"},
		},
		{
			"working dev, door on top",
			DeveloperAnimation{State: StateWorking, ColorIndex: 0},
			"Athena",
			true,
			[]string{"Athena", "🖥", "🗑", "┤"},
		},
		{
			"in conference, cubicle empty",
			DeveloperAnimation{State: StateConference, ColorIndex: 0},
			"MsPac",
			false,
			[]string{"MsPac", "🖥", "🗑", "┤"}, // Structure still present, just no face
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderCubicleDetailed(tt.anim, tt.devName, 18, tt.doorOnTop)
			plain := StripANSI(result)
			for _, want := range tt.wantIn { // justified:AS
				if !strings.Contains(plain, want) {
					t.Errorf("renderCubicleDetailed() missing %q\ngot:\n%s", want, plain)
				}
			}
		})
	}
}

func TestRenderCubicleDetailed_LineOrder(t *testing.T) {
	anim := DeveloperAnimation{State: StateWorking, ColorIndex: 0}

	t.Run("door on bottom: desk at back wall, face near door", func(t *testing.T) {
		// ColorIndex=0, doorOnTop=false: trashCorner=0 (TL) → trash on back wall, left
		// 6 lines: topBorder(0), backWall(1), empty(2), empty(3), face(4), doorBorder(5)
		result := renderCubicleDetailed(anim, "MsPac", 18, false)
		plain := StripANSI(result)
		lines := strings.Split(plain, "\n")
		if !strings.Contains(lines[1], "🗑") || !strings.Contains(lines[1], "🖥") {
			t.Errorf("line 1 (back wall) should have trash and monitor\ngot: %q", lines[1])
		}
		if !containsFaceEmoji(lines[4]) {
			t.Errorf("line 4 should have face\ngot: %q", lines[4])
		}
		if !strings.Contains(lines[5], "MsPac") {
			t.Errorf("door border should contain name\ngot: %q", lines[5])
		}
		if !strings.Contains(lines[5], "┤") {
			t.Errorf("door border should have ┤ ├ posts\ngot: %q", lines[5])
		}
	})

	t.Run("door on top: face near door, desk at back wall", func(t *testing.T) {
		// ColorIndex=0, doorOnTop=true: trashCorner=0 (TL) → trash on face line, left
		// 6 lines: doorBorder(0), face(1), empty(2), empty(3), backWall(4), bottomBorder(5)
		result := renderCubicleDetailed(anim, "Athena", 18, true)
		plain := StripANSI(result)
		lines := strings.Split(plain, "\n")
		if !strings.Contains(lines[0], "Athena") {
			t.Errorf("door border should contain name\ngot: %q", lines[0])
		}
		if !strings.Contains(lines[0], "┤") {
			t.Errorf("door border should have ┤ ├ posts\ngot: %q", lines[0])
		}
		if !strings.Contains(lines[1], "🗑") {
			t.Errorf("line 1 (face) should have trash in corner\ngot: %q", lines[1])
		}
		if !strings.Contains(lines[4], "🖥") {
			t.Errorf("line 4 (back wall) should have monitor\ngot: %q", lines[4])
		}
	})
}

// Cycle 5: Enhanced layout with moving developer
func TestRenderMovingDeveloper(t *testing.T) {
	state := NewOfficeState([]string{"d0", "d1"})
	// d0 is moving to cubicle (cubicle should be empty)
	state = state.StartDeveloperMovingToCubicle("d0", Position{X: 50, Y: 6}, testTime)
	state.Animations[0].Progress = 0.5 // halfway
	// d1 is in conference
	state = state.SetDeveloperState("d1", StateConference)
	names := []string{"MsPac", "Qbert"}

	result := renderOfficeEnhanced(state, names, 80, 12)
	plain := StripANSI(result)

	// d1 should be visible in conference room
	if !strings.Contains(plain, "🙂") {
		t.Errorf("Conference dev should be visible\ngot:\n%s", plain)
	}

	// d0's cubicle should show name but no face (away)
	if !strings.Contains(plain, "MsPac") {
		t.Errorf("Moving dev's cubicle should still show name\ngot:\n%s", plain)
	}

	// Should have outer frame (rounded corners from lipgloss.RoundedBorder)
	lines := strings.Split(plain, "\n")
	if len(lines) > 0 && !strings.HasPrefix(lines[0], "╭") {
		t.Errorf("Should have outer frame, first line: %q", lines[0])
	}
}

// Cycle 4: Cubicle grid with hallway between rows
func TestRenderCubicleGridDetailed(t *testing.T) {
	devIDs := []string{"d0", "d1", "d2", "d3", "d4", "d5"}
	state := NewOfficeState(devIDs)
	// Set some devs to working (in cubicles)
	state = state.SetDeveloperState("d0", StateWorking)
	state = state.SetDeveloperState("d1", StateWorking)
	names := []string{"MsPac", "Qbert", "Samus", "Athena", "Mappy", "Pengo"}

	result := renderCubicleGridDetailed(state, names, 54)
	plain := StripANSI(result)

	// Should have entryway posts (┤ ├) between cubicle rows
	if !strings.Contains(plain, "┤") || !strings.Contains(plain, "├") {
		t.Errorf("Should contain entryway posts between cubicle rows\ngot:\n%s", plain)
	}

	// Hallway should be 3 lines between cubicle rows
	// Find the entryway borders: row0 ends with └──┤ ├──┘, row1 starts with ┌──┤ ├──┐
	lines := strings.Split(plain, "\n")
	var row0End, row1Start int
	for i, line := range lines { // justified:IX
		if strings.Contains(line, "┘") && strings.Contains(line, "┤") && row0End == 0 {
			row0End = i
		}
		if strings.Contains(line, "┐") && strings.Contains(line, "├") {
			row1Start = i
			break
		}
	}
	hallwayLines := row1Start - row0End - 1
	if hallwayLines != 3 {
		t.Errorf("Expected 3 hallway lines between cubicle rows, got %d", hallwayLines)
	}

	// Should have all 6 developer names
	for _, name := range names { // justified:AS
		if !strings.Contains(plain, name) {
			t.Errorf("Should contain developer name %q", name)
		}
	}

	// Should have entryway posts (6 cubicles × 2 posts = 12 ┤ chars)
	postCount := strings.Count(plain, "┤")
	if postCount < 6 {
		t.Errorf("Expected at least 6 entryway posts, got %d", postCount)
	}
}

func TestRenderCubicleGridDetailed_LateBubble(t *testing.T) {
	devIDs := []string{"d0", "d1", "d2", "d3", "d4", "d5"}
	names := []string{"MsPac", "Qbert", "Samus", "Athena", "Mappy", "Pengo"}

	t.Run("frustrated row-1 dev shows bubble in hallway", func(t *testing.T) {
		state := NewOfficeState(devIDs)
		// d3 (Athena) in row 1 becomes frustrated → Late! bubble in hallway
		state = state.SetDeveloperState("d3", StateFrustrated)

		result := renderCubicleGridDetailed(state, names, 54)
		plain := StripANSI(result)

		// Check for hallway bubble (full 3-line box), not just inline ╭Late!
		// Find hallway section between cubicle rows
		lines := strings.Split(plain, "\n")
		var hallwayStart, hallwayEnd int
		for i, line := range lines {
			if strings.Contains(line, "┴") && hallwayStart == 0 {
				hallwayStart = i + 1
			}
			if strings.Contains(line, "┬") {
				hallwayEnd = i
				break
			}
		}
		hallwaySection := strings.Join(lines[hallwayStart:hallwayEnd], "\n")
		if !strings.Contains(hallwaySection, "Late!") {
			t.Errorf("Should show Late! bubble in hallway when row-1 dev is frustrated\nhallway:\n%s\nfull:\n%s", hallwaySection, plain)
		}
	})

	t.Run("no frustrated devs means no bubble", func(t *testing.T) {
		state := NewOfficeState(devIDs)
		state = state.SetDeveloperState("d3", StateWorking)

		result := renderCubicleGridDetailed(state, names, 54)
		plain := StripANSI(result)

		// Hallway should be present but no Late! bubble
		if strings.Contains(plain, "Late!") {
			t.Errorf("Should not show Late! bubble when no dev is frustrated\ngot:\n%s", plain)
		}
	})

	t.Run("frustrated row-0 dev does not show bubble in hallway", func(t *testing.T) {
		state := NewOfficeState(devIDs)
		// d0 (MsPac) in row 0 is frustrated — bubble should be inline in cubicle, not hallway
		state = state.SetDeveloperState("d0", StateFrustrated)

		result := renderCubicleGridDetailed(state, names, 54)
		plain := StripANSI(result)

		// Find hallway section (between ┴ door borders and ┬ door borders)
		lines := strings.Split(plain, "\n")
		var hallwayStart, hallwayEnd int
		for i, line := range lines {
			if strings.Contains(line, "┴") && hallwayStart == 0 {
				hallwayStart = i + 1
			}
			if strings.Contains(line, "┬") {
				hallwayEnd = i
				break
			}
		}
		hallwaySection := strings.Join(lines[hallwayStart:hallwayEnd], "\n")
		if strings.Contains(hallwaySection, "Late!") {
			t.Errorf("Row-0 frustrated dev should not show Late! bubble in hallway\nhallway:\n%s", hallwaySection)
		}
	})
}

// findHallwaySection extracts the 3-line hallway between cubicle rows from rendered grid.
// Row 0 bottom border contains └ and ┘, row 1 top border contains ┌ and ┐.
func findHallwaySection(plain string) string {
	lines := strings.Split(plain, "\n")
	var row0End, row1Start int
	for i, line := range lines {
		if strings.Contains(line, "┘") && strings.Contains(line, "┤") && row0End == 0 {
			row0End = i
		}
		if strings.Contains(line, "┐") && strings.Contains(line, "├") {
			row1Start = i
			break
		}
	}
	if row0End == 0 || row1Start == 0 || row1Start <= row0End+1 {
		return ""
	}
	return strings.Join(lines[row0End+1:row1Start], "\n")
}

func TestRenderCubicleGridDetailed_WalkingDevInHallway(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	devIDs := []string{"d0", "d1", "d2", "d3", "d4", "d5"}
	names := []string{"MsPac", "Qbert", "Samus", "Athena", "Mappy", "Pengo"}

	t.Run("walking dev appears in hallway", func(t *testing.T) {
		state := NewOfficeState(devIDs)
		target := Position{X: 5, Y: 5}
		state = state.StartDeveloperMovingToConference("d1", target, now)

		result := renderCubicleGridDetailed(state, names, 54)
		plain := StripANSI(result)
		hallway := findHallwaySection(plain)

		// Walking dev should show face emoji in hallway (default face: 🙂)
		if !strings.Contains(hallway, "🙂") {
			t.Errorf("Walking dev should appear in hallway with face\nhallway:\n%s\nfull:\n%s", hallway, plain)
		}
	})

	t.Run("walking dev not shown in cubicle", func(t *testing.T) {
		state := NewOfficeState(devIDs)
		target := Position{X: 5, Y: 5}
		state = state.StartDeveloperMovingToConference("d0", target, now)

		// d0 is IsAway() = true during StateMovingToConference
		anim := state.Animations[0]
		if !anim.IsAway() {
			t.Fatal("StateMovingToConference dev should be IsAway()")
		}
	})

	t.Run("walkers take priority over Late bubble", func(t *testing.T) {
		state := NewOfficeState(devIDs)
		target := Position{X: 5, Y: 5}
		// d3 frustrated with active bubble, but d1 walking (takes priority)
		state = state.SetDeveloperState("d3", StateFrustrated)
		state.Animations[3].LateBubbleFrames = 10 // active Late! bubble
		state = state.StartDeveloperMovingToConference("d1", target, now)

		result := renderCubicleGridDetailed(state, names, 54)
		plain := StripANSI(result)
		hallway := findHallwaySection(plain)

		if strings.Contains(hallway, "Late!") {
			t.Errorf("Walking devs should take priority over Late! bubble\nhallway:\n%s", hallway)
		}
		if !strings.Contains(hallway, "🙂") {
			t.Errorf("Should show walking dev face in hallway\nhallway:\n%s", hallway)
		}
	})
}

// Cycle 3: Conference room with charts, table, chairs, door
func TestRenderConferenceRoomDetailed(t *testing.T) {
	anims := []DeveloperAnimation{
		{State: StateConference, ColorIndex: 0},
		{State: StateConference, ColorIndex: 1, Accessory: AccessoryCoffee},
		{State: StateConference, ColorIndex: 2},
	}

	result := renderConferenceRoomDetailed(anims, 25)
	plain := StripANSI(result)

	// Structure elements
	wantElements := []string{
		"📊", "📈", "📉", // charts
		"╦", // TT table connector (tabletop joins to legs)
		"║", // table legs
		"🪑", // chairs (empty seats)
	}
	for _, want := range wantElements { // justified:AS
		if !strings.Contains(plain, want) {
			t.Errorf("Conference room missing %q\ngot:\n%s", want, plain)
		}
	}

	// Table should NOT have old box-style bottom
	if strings.Contains(plain, "╚") {
		t.Errorf("Conference room should not have ╚ (old table bottom)\ngot:\n%s", plain)
	}

	// Should have developer faces (3 devs in conference)
	faceCount := strings.Count(plain, "🙂")
	coffeeCount := strings.Count(plain, "☕")
	if faceCount < 2 { // At least 2 neutral faces (dev 0 and 2), dev 1 might show with coffee
		t.Errorf("Expected at least 2 neutral faces, got %d", faceCount)
	}
	if coffeeCount < 1 {
		t.Errorf("Expected at least 1 coffee (dev 1 has accessory), got %d", coffeeCount)
	}
}

// TestCubicleAccessoryOnDesk verifies: accessory on desk iff !IsAway() && has accessory.
// Only applies to renderCubicleDetailed (the only renderer with a desk line).
func TestCubicleAccessoryOnDesk(t *testing.T) {
	allStates := []struct {
		state AnimationState
		name  string
	}{
		{StateIdle, "idle"},
		{StateWorking, "working"},
		{StateFrustrated, "frustrated"},
		{StateConference, "conference"},
		{StateMovingToConference, "movingToConference"},
		{StateMovingToCubicle, "movingToCubicle"},
	}

	for _, s := range allStates { // justified:AS
		t.Run(s.name, func(t *testing.T) {
			anim := DeveloperAnimation{
				State:      s.state,
				ColorIndex: 1,
				Accessory:  AccessoryCoffee,
			}
			result := renderCubicleDetailed(anim, "Qbert", 18, false)
			lines := strings.Split(result, "\n")
			deskLine := StripANSI(lines[1]) // desk is line 1 (back wall, door on bottom)
			hasAccessory := strings.Contains(deskLine, "☕")
			wantAccessory := !anim.IsAway()
			if hasAccessory != wantAccessory {
				t.Errorf("accessory on desk = %v, want %v\ndesk line: %q", hasAccessory, wantAccessory, deskLine)
			}
		})
	}
}

// Cycle 0: Validate emoji widths for enhanced layout
func TestEmojiWidths(t *testing.T) {
	tests := []struct {
		name  string
		emoji string
		want  int
	}{
		{"neutral face", "🙂", 2},
		{"trash can", "🗑", 1},
		{"desktop", "🖥", 1},
		{"coffee", "☕", 2},
		{"door", "🚪", 2},
		{"chair", "🪑", 2},
		{"chart bar", "📊", 2},
		{"chart up", "📈", 2},
		{"chart down", "📉", 2},
		{"soda", "🥤", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lipgloss.Width(tt.emoji)
			if got != tt.want {
				t.Errorf("lipgloss.Width(%s) = %d, want %d", tt.emoji, got, tt.want)
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		want     string
	}{
		{"short text", "MsPac", 10, "MsPac"},
		{"exact width", "Hello", 5, "Hello"},
		{"needs truncation", "Bartholomew", 8, "Barthol…"},
		{"unicode", "田中太郎", 6, "田中…"},
		{"very narrow", "Hello", 2, "H…"},
		{"width 1", "Hello", 1, "…"},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateText(tt.text, tt.maxWidth)
			if got != tt.want {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.text, tt.maxWidth, got, tt.want)
			}
		})
	}
}

// Cycle 6: Full enhanced composition + width dispatch
func TestRenderOfficeEnhanced(t *testing.T) {
	devIDs := []string{"dev-1", "dev-2", "dev-3", "dev-4", "dev-5", "dev-6"}
	state := NewOfficeState(devIDs)
	state.Animations[0].State = StateConference
	state.Animations[1].State = StateWorking
	names := DefaultDeveloperNames

	result := renderOfficeEnhanced(state, names, 100, 12)
	plain := StripANSI(result)

	// Outer frame (lipgloss rounded border)
	lines := strings.Split(plain, "\n")
	if len(lines) == 0 {
		t.Fatal("Expected non-empty output")
	}
	if !strings.Contains(lines[0], "╭") {
		t.Errorf("First line should contain rounded border top-left (╭), got: %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], "╰") {
		t.Errorf("Last line should contain rounded border bottom-left (╰), got: %q", lines[len(lines)-1])
	}

	// Conference elements
	for _, want := range []string{"📊", "🪑", "╦"} { // justified:AS
		if !strings.Contains(plain, want) {
			t.Errorf("Enhanced layout should contain %s", want)
		}
	}

	// Cubicle elements
	for _, want := range []string{"🖥", "🗑"} { // justified:AS
		if !strings.Contains(plain, want) {
			t.Errorf("Enhanced layout should contain %s", want)
		}
	}
}

func TestRenderOffice_WidthDispatch(t *testing.T) {
	devIDs := []string{"dev-1", "dev-2", "dev-3"}
	state := NewOfficeState(devIDs)
	names := DefaultDeveloperNames[:3]

	// Narrow: simple layout (no conference details)
	narrow := StripANSI(RenderOffice(state, names, 60, 12))
	if strings.Contains(narrow, "📊") {
		t.Error("Simple layout (width=60) should not contain chart emoji")
	}

	// Wide: enhanced layout
	wide := StripANSI(RenderOffice(state, names, 100, 12))
	if !strings.Contains(wide, "📊") {
		t.Error("Enhanced layout (width=100) should contain chart emoji")
	}
}

func TestDeveloperColor(t *testing.T) {
	tests := []struct {
		name       string
		colorIndex int
		wantIdx    int // expected index into DeveloperColors
	}{
		{"zero", 0, 0},
		{"positive", 2, 2},
		{"wraps at 6", 6, 0},
		{"negative one", -1, 5},
		{"very negative", -100, 2}, // -100 % 6 = -4, (-4 + 6) % 6 = 2
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := developerColor(tt.colorIndex)
			want := DeveloperColors[tt.wantIdx]
			if got != want {
				t.Errorf("developerColor(%d) = %v, want %v (index %d)", tt.colorIndex, got, want, tt.wantIdx)
			}
		})
	}
}

package office

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// containsFaceEmoji returns true if the string contains any developer face emoji.
func containsFaceEmoji(s string) bool {
	for _, face := range WorkingFrames {
		if strings.Contains(s, face) {
			return true
		}
	}
	for _, face := range FrustratedFrames {
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

	cubicle := RenderCubicleCompact(anim, "Mei", 10)
	plain := StripANSI(cubicle)

	if !strings.Contains(plain, "Late!") {
		t.Error("Compact cubicle should show Late! bubble when LateBubbleFrames > 0")
	}

	// After countdown, bubble should disappear
	for i := 0; i < 10; i++ {
		anim = anim.NextFrame()
	}
	cubicle = RenderCubicleCompact(anim, "Mei", 10)
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
	names := []string{"Mei", "Amir", "Suki"}

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
	names := []string{"Mei"}

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
	for _, name := range names {
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
			return StripANSI(RenderCubicle(a, "Mei", 12))
		}},
		{"RenderCubicleCompact", func(a DeveloperAnimation) string {
			return StripANSI(RenderCubicleCompact(a, "Mei", 10))
		}},
		{"renderCubicleDetailed", func(a DeveloperAnimation) string {
			return StripANSI(renderCubicleDetailed(a, "Mei", 9, false))
		}},
	}

	for _, r := range renderers {
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

	cubicle := RenderCubicle(anim, "Mei", 12)
	plain := StripANSI(cubicle)

	if !strings.Contains(plain, "Late!") {
		t.Error("Should show Late! bubble when LateBubbleFrames > 0")
	}

	// After 10 frames, bubble should disappear
	for i := 0; i < 10; i++ {
		anim = anim.NextFrame()
	}
	cubicle = RenderCubicle(anim, "Mei", 12)
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
	for _, name := range names {
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
			"Mei",
			false,
			[]string{"Mei", "🖥️", "🗑️", "🚪"},
		},
		{
			"working dev, door on top",
			DeveloperAnimation{State: StateWorking, ColorIndex: 0},
			"Jay",
			true,
			[]string{"Jay", "🖥️", "🗑️", "🚪"},
		},
		{
			"in conference, cubicle empty",
			DeveloperAnimation{State: StateConference, ColorIndex: 0},
			"Mei",
			false,
			[]string{"Mei", "🖥️", "🗑️", "🚪"}, // Structure still present, just no face
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderCubicleDetailed(tt.anim, tt.devName, 9, tt.doorOnTop)
			plain := StripANSI(result)
			for _, want := range tt.wantIn {
				if !strings.Contains(plain, want) {
					t.Errorf("renderCubicleDetailed() missing %q\ngot:\n%s", want, plain)
				}
			}
		})
	}
}

// Cycle 5: Enhanced layout with moving developer
func TestRenderMovingDeveloper(t *testing.T) {
	state := NewOfficeState([]string{"d0", "d1"})
	// d0 is moving to cubicle (cubicle should be empty)
	state = state.StartDeveloperMovingToCubicle("d0", Position{X: 50, Y: 6}, testTime)
	state.Animations[0].Progress = 0.5 // halfway
	// d1 is in conference
	state = state.SetDeveloperState("d1", StateConference)
	names := []string{"Mei", "Amir"}

	result := renderOfficeEnhanced(state, names, 80, 12)
	plain := StripANSI(result)

	// d1 should be visible in conference room
	if !strings.Contains(plain, "🙂") {
		t.Errorf("Conference dev should be visible\ngot:\n%s", plain)
	}

	// d0's cubicle should show name but no face (away)
	if !strings.Contains(plain, "Mei") {
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
	names := []string{"Mei", "Amir", "Suki", "Jay", "Priya", "Kofi"}

	result := renderCubicleGridDetailed(state, names, 45)
	plain := StripANSI(result)

	// Should have HALLWAY label
	if !strings.Contains(plain, "HALLWAY") {
		t.Errorf("Should contain HALLWAY label\ngot:\n%s", plain)
	}

	// Should have all 6 developer names
	for _, name := range names {
		if !strings.Contains(plain, name) {
			t.Errorf("Should contain developer name %q", name)
		}
	}

	// Should have doors (🚪)
	doorCount := strings.Count(plain, "🚪")
	if doorCount < 6 {
		t.Errorf("Expected 6 doors, got %d", doorCount)
	}
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
		"╔", "╚", // table
		"🪑", // chairs
		"🚪", // door
	}
	for _, want := range wantElements {
		if !strings.Contains(plain, want) {
			t.Errorf("Conference room missing %q\ngot:\n%s", want, plain)
		}
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

	for _, s := range allStates {
		t.Run(s.name, func(t *testing.T) {
			anim := DeveloperAnimation{
				State:      s.state,
				ColorIndex: 1,
				Accessory:  AccessoryCoffee,
			}
			result := renderCubicleDetailed(anim, "Amir", 11, false)
			lines := strings.Split(result, "\n")
			deskLine := StripANSI(lines[3]) // desk is line 3
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
		{"trash can", "🗑️", 2},
		{"desktop", "🖥️", 2},
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
		{"short text", "Mei", 10, "Mei"},
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

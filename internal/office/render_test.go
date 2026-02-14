package office

import (
	"strings"
	"testing"
)

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

func TestRenderCubicleCompact_IdleState(t *testing.T) {
	anim := NewDeveloperAnimation("dev-1", 0) // Starts idle

	cubicle := RenderCubicleCompact(anim, "Mei", 10)
	plain := StripANSI(cubicle)

	if !strings.Contains(plain, "Mei") {
		t.Error("Should contain developer name")
	}
	// Idle state shows neutral face
	if !strings.Contains(plain, "🙂") {
		t.Errorf("Idle state should show neutral face 🙂, got:\n%s", plain)
	}
}

func TestRenderCubicleCompact_WorkingState(t *testing.T) {
	anim := DeveloperAnimation{
		DevID:      "dev-1",
		State:      StateWorking,
		ColorIndex: 0,
		Frame:      0,
	}

	cubicle := RenderCubicleCompact(anim, "Mei", 10)
	plain := StripANSI(cubicle)

	if !strings.Contains(plain, "Mei") {
		t.Error("Should contain developer name")
	}
	// Working state cycles through happy faces (😊😄😁🙂😀)
	hasWorkingIcon := strings.Contains(plain, "😊") || strings.Contains(plain, "😄") ||
		strings.Contains(plain, "😁") || strings.Contains(plain, "🙂") || strings.Contains(plain, "😀")
	if !hasWorkingIcon {
		t.Error("Working state should show happy face icon")
	}
	if !strings.Contains(plain, "┌") || !strings.Contains(plain, "└") {
		t.Error("Should have box borders")
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

func TestRenderCubicleCompact_ConferenceState(t *testing.T) {
	anim := DeveloperAnimation{
		DevID:      "dev-1",
		State:      StateConference,
		ColorIndex: 0,
	}

	cubicle := RenderCubicleCompact(anim, "Mei", 10)
	plain := StripANSI(cubicle)

	if !strings.Contains(plain, "Mei") {
		t.Error("Should contain developer name even in conference")
	}
	// Icon line should be empty (just spaces) when in conference
	lines := strings.Split(plain, "\n")
	iconLine := lines[2] // third line is icon line
	iconContent := strings.Trim(iconLine, "│ ")
	if iconContent != "" {
		t.Errorf("Conference state should have empty icon line, got %q", iconContent)
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

func TestRenderCubicle_Frustrated(t *testing.T) {
	anim := DeveloperAnimation{
		DevID:      "dev-1",
		State:      StateFrustrated,
		ColorIndex: 0,
		Frame:      0,
	}

	cubicle := RenderCubicle(anim, "Mei", 12)
	plain := StripANSI(cubicle)

	if !strings.Contains(plain, "Mei") {
		t.Error("Should contain developer name")
	}
	// Frustrated state shows unhappy emoji (frame 0 = 😤)
	if !strings.Contains(plain, "😤") {
		t.Error("Frustrated state should show frustrated emoji")
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

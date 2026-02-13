package tui

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
	"github.com/binaryphile/sofdevsim-2026/internal/office"
)

func init() {
	// Force ASCII color profile for consistent test output
	lipgloss.SetColorProfile(termenv.Ascii)
}

// Step 2: Layout Calculations - Table-driven tests for pure functions

func TestCubicleLayout(t *testing.T) {
	tests := []struct {
		n    int
		want int // number of positions returned
	}{
		{1, 1},
		{3, 3},
		{6, 6},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("n=%d", tt.n), func(t *testing.T) {
			got := CubicleLayout(tt.n)
			if len(got) != tt.want {
				t.Errorf("CubicleLayout(%d) returned %d positions, want %d", tt.n, len(got), tt.want)
			}
		})
	}
}

func TestCubicleLayout_Positions(t *testing.T) {
	// 6 devs should be arranged in 2 rows x 3 columns
	positions := CubicleLayout(6)

	// Row 1: positions 0, 1, 2 should have same Y
	if positions[0].Y != positions[1].Y || positions[1].Y != positions[2].Y {
		t.Error("First row positions should have same Y coordinate")
	}

	// Row 2: positions 3, 4, 5 should have same Y
	if positions[3].Y != positions[4].Y || positions[4].Y != positions[5].Y {
		t.Error("Second row positions should have same Y coordinate")
	}

	// Row 2 should be below Row 1
	if positions[3].Y <= positions[0].Y {
		t.Error("Second row should be below first row")
	}

	// Columns should have increasing X
	if positions[0].X >= positions[1].X || positions[1].X >= positions[2].X {
		t.Error("Columns should have increasing X coordinates")
	}
}

func TestConferencePosition(t *testing.T) {
	tests := []struct {
		idx   int
		total int
	}{
		{0, 1},
		{0, 3},
		{1, 3},
		{2, 3},
		{0, 6},
		{5, 6},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("idx=%d/total=%d", tt.idx, tt.total), func(t *testing.T) {
			pos := ConferencePosition(tt.idx, tt.total)
			// Position should be valid (non-negative)
			if pos.X < 0 || pos.Y < 0 {
				t.Errorf("ConferencePosition(%d, %d) = %v, want non-negative coordinates", tt.idx, tt.total, pos)
			}
		})
	}
}

func TestConferencePosition_Spacing(t *testing.T) {
	// 3 devs in conference should be evenly spaced
	pos0 := ConferencePosition(0, 3)
	pos1 := ConferencePosition(1, 3)
	pos2 := ConferencePosition(2, 3)

	// All should have same Y (horizontal line)
	if pos0.Y != pos1.Y || pos1.Y != pos2.Y {
		t.Error("Conference positions should have same Y coordinate")
	}

	// X should be increasing
	if pos0.X >= pos1.X || pos1.X >= pos2.X {
		t.Error("Conference positions should have increasing X coordinates")
	}

	// Spacing should be equal
	spacing1 := pos1.X - pos0.X
	spacing2 := pos2.X - pos1.X
	if spacing1 != spacing2 {
		t.Errorf("Conference spacing should be equal: got %d and %d", spacing1, spacing2)
	}
}

func TestLerp(t *testing.T) {
	from := Position{X: 0, Y: 0}
	to := Position{X: 100, Y: 50}

	tests := []struct {
		t    float64
		want Position
	}{
		{0.0, Position{X: 0, Y: 0}},
		{1.0, Position{X: 100, Y: 50}},
		{0.5, Position{X: 50, Y: 25}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("t=%.1f", tt.t), func(t *testing.T) {
			got := Lerp(from, to, tt.t)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Lerp(%v, %v, %.1f) = %v, want %v", from, to, tt.t, got, tt.want)
			}
		})
	}
}

// Step 3: State Transitions - Event-driven state machine tests

func TestNewDeveloperAnimation(t *testing.T) {
	anim := NewDeveloperAnimation("dev-1", 0)

	if anim.DevID != "dev-1" {
		t.Errorf("DevID = %q, want %q", anim.DevID, "dev-1")
	}
	if anim.State != StateIdle {
		t.Errorf("Initial state = %v, want StateIdle", anim.State)
	}
	if anim.ColorIndex != 0 {
		t.Errorf("ColorIndex = %d, want 0", anim.ColorIndex)
	}
}

func TestDeveloperAnimation_FrameOffset_VisualVariety(t *testing.T) {
	// Developers should display different frames even when raw Frame is same
	// This prevents unison cycling (all devs showing same icon)
	anim0 := NewDeveloperAnimation("dev-0", 0) // FrameOffset = 0
	anim1 := NewDeveloperAnimation("dev-1", 1) // FrameOffset = 1
	anim2 := NewDeveloperAnimation("dev-2", 2) // FrameOffset = 2

	// All have Frame=0 initially
	if anim0.Frame != 0 || anim1.Frame != 0 || anim2.Frame != 0 {
		t.Fatal("All animations should start with Frame=0")
	}

	// But CurrentFrame() should differ due to offset
	if anim0.CurrentFrame() == anim1.CurrentFrame() {
		t.Error("dev-0 and dev-1 should have different CurrentFrame()")
	}
	if anim1.CurrentFrame() == anim2.CurrentFrame() {
		t.Error("dev-1 and dev-2 should have different CurrentFrame()")
	}

	// Verify offsets are deterministic based on colorIndex
	if anim0.FrameOffset != 0 {
		t.Errorf("dev-0 FrameOffset = %d, want 0", anim0.FrameOffset)
	}
	if anim1.FrameOffset != 1 {
		t.Errorf("dev-1 FrameOffset = %d, want 1", anim1.FrameOffset)
	}
	if anim2.FrameOffset != 2 {
		t.Errorf("dev-2 FrameOffset = %d, want 2", anim2.FrameOffset)
	}

	// CurrentFrame should be (Frame + FrameOffset) % len(WorkingFrames)
	expectedFrames := []int{0, 1, 2}
	actualFrames := []int{anim0.CurrentFrame(), anim1.CurrentFrame(), anim2.CurrentFrame()}
	for i, expected := range expectedFrames {
		if actualFrames[i] != expected {
			t.Errorf("dev-%d CurrentFrame() = %d, want %d", i, actualFrames[i], expected)
		}
	}
}

func TestDeveloperAnimation_WithState(t *testing.T) {
	anim := NewDeveloperAnimation("dev-1", 0)

	// Immutable: returns new value
	anim2 := anim.WithState(StateConference)

	if anim.State != StateIdle {
		t.Error("Original should remain unchanged")
	}
	if anim2.State != StateConference {
		t.Errorf("New state = %v, want StateConference", anim2.State)
	}
}

func TestDeveloperAnimation_WithPosition(t *testing.T) {
	anim := NewDeveloperAnimation("dev-1", 0)
	newPos := Position{X: 50, Y: 10}

	anim2 := anim.WithPosition(newPos)

	if anim2.Position != newPos {
		t.Errorf("Position = %v, want %v", anim2.Position, newPos)
	}
}

func TestDeveloperAnimation_WithTarget(t *testing.T) {
	anim := NewDeveloperAnimation("dev-1", 0)
	target := Position{X: 100, Y: 20}

	anim2 := anim.WithTarget(target)

	if anim2.Target != target {
		t.Errorf("Target = %v, want %v", anim2.Target, target)
	}
}

// Step 4: Frame Cycling

func TestDeveloperAnimation_NextFrame(t *testing.T) {
	tests := []struct {
		startFrame int
		wantFrame  int
	}{
		{0, 1},
		{1, 2},
		{4, 0}, // wraps around
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("frame=%d", tt.startFrame), func(t *testing.T) {
			anim := DeveloperAnimation{Frame: tt.startFrame}
			anim = anim.NextFrame()
			if anim.Frame != tt.wantFrame {
				t.Errorf("NextFrame() from %d = %d, want %d", tt.startFrame, anim.Frame, tt.wantFrame)
			}
		})
	}
}

// TestLerp_EdgeCases tests boundary conditions for interpolation
func TestLerp_EdgeCases(t *testing.T) {
	from := Position{X: 0, Y: 0}
	to := Position{X: 100, Y: 50}

	tests := []struct {
		name string
		t    float64
		want Position
	}{
		{"negative t clamps to start", -0.5, Position{X: -50, Y: -25}}, // Lerp doesn't clamp - documents behavior
		{"t > 1 extrapolates", 1.5, Position{X: 150, Y: 75}},           // Lerp doesn't clamp - documents behavior
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Lerp(from, to, tt.t)
			if got != tt.want {
				t.Errorf("Lerp(%v, %v, %.1f) = %v, want %v", from, to, tt.t, got, tt.want)
			}
		})
	}
}

// TestDeveloperAnimation_Movement tests movement interpolation
func TestDeveloperAnimation_Movement(t *testing.T) {
	anim := DeveloperAnimation{
		State:    StateIdle,
		Position: Position{X: 10, Y: 5},
	}

	// Start moving to target (cubicle)
	target := Position{X: 50, Y: 10}
	anim = anim.StartMovingToCubicle(target, testTime)

	if anim.State != StateMovingToCubicle {
		t.Errorf("After StartMovingToCubicle: State = %v, want StateMovingToCubicle", anim.State)
	}
	if anim.Progress != 0.0 {
		t.Errorf("After StartMovingToCubicle: Progress = %f, want 0.0", anim.Progress)
	}

	// Advance past MovementDuration (500ms) to complete movement
	completedTime := testTime.Add(office.MovementDuration + time.Millisecond)
	anim = anim.AdvanceMovement(completedTime)

	if anim.State != StateWorking {
		t.Errorf("After movement complete: State = %v, want StateWorking", anim.State)
	}
	if anim.Position != target {
		t.Errorf("After movement complete: Position = %v, want %v", anim.Position, target)
	}
}

// TestCubicleLayout_EdgeCases tests boundary conditions
func TestCubicleLayout_EdgeCases(t *testing.T) {
	t.Run("zero devs returns empty", func(t *testing.T) {
		positions := CubicleLayout(0)
		if len(positions) != 0 {
			t.Errorf("CubicleLayout(0) = %d positions, want 0", len(positions))
		}
	})

	t.Run("7+ devs still produces positions", func(t *testing.T) {
		// Layout continues the grid pattern for any count
		positions := CubicleLayout(7)
		if len(positions) != 7 {
			t.Errorf("CubicleLayout(7) = %d positions, want 7", len(positions))
		}
		// 7th position should be row 2, col 0 (third row)
		if positions[6].Y <= positions[3].Y {
			t.Error("7th position should be in third row (Y > row 2)")
		}
	})
}

// Step 5: ASCII Rendering - String content checks

func TestRenderDeveloperIcon(t *testing.T) {
	tests := []struct {
		state AnimationState
		frame int
		want  string
	}{
		{StateIdle, 0, "🙂"},
		{StateConference, 0, "🙂"},
		{StateWorking, 0, "😊"},
		{StateWorking, 2, "😁"},
		{StateWorking, 4, "😀"},
		{StateFrustrated, 0, "😤"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("state=%d/frame=%d", tt.state, tt.frame), func(t *testing.T) {
			anim := DeveloperAnimation{State: tt.state, Frame: tt.frame}
			got := RenderDeveloperIcon(anim)
			if got != tt.want {
				t.Errorf("RenderDeveloperIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// Step 6: OfficeState Manager

func TestNewOfficeState(t *testing.T) {
	devIDs := []string{"dev-1", "dev-2", "dev-3"}
	state := NewOfficeState(devIDs)

	if len(state.Animations) != 3 {
		t.Errorf("Animations count = %d, want 3", len(state.Animations))
	}

	for i, anim := range state.Animations {
		// Developers start at cubicles (Idle), move to conference via events
		if anim.State != StateIdle {
			t.Errorf("Animation %d: state = %v, want StateIdle", i, anim.State)
		}
		if anim.ColorIndex != i {
			t.Errorf("Animation %d: colorIndex = %d, want %d", i, anim.ColorIndex, i)
		}
	}
}

func TestOfficeState_GetAnimationOption(t *testing.T) {
	devIDs := []string{"dev-1", "dev-2", "dev-3"}
	state := NewOfficeState(devIDs)

	anim, ok := state.GetAnimationOption("dev-2").Get()
	if !ok {
		t.Fatal("GetAnimationOption should find dev-2")
	}
	if anim.DevID != "dev-2" {
		t.Errorf("DevID = %q, want %q", anim.DevID, "dev-2")
	}
	if anim.ColorIndex != 1 {
		t.Errorf("ColorIndex = %d, want 1", anim.ColorIndex)
	}

	if state.GetAnimationOption("nonexistent").IsOk() {
		t.Error("GetAnimationOption should not find nonexistent dev")
	}
}

func TestOfficeState_SetDeveloperState(t *testing.T) {
	devIDs := []string{"dev-1", "dev-2", "dev-3"}
	state := NewOfficeState(devIDs)

	// Immutable: returns new state
	state2 := state.SetDeveloperState("dev-2", StateWorking)

	// Original unchanged (starts at cubicle/Idle)
	anim1 := state.GetAnimationOption("dev-2").OrZero()
	if anim1.State != StateIdle {
		t.Error("Original state should be unchanged")
	}

	// New state has change
	anim2 := state2.GetAnimationOption("dev-2").OrZero()
	if anim2.State != StateWorking {
		t.Errorf("New state = %v, want StateWorking", anim2.State)
	}

	// Other devs unchanged (still at cubicle/Idle)
	anim3 := state2.GetAnimationOption("dev-1").OrZero()
	if anim3.State != StateIdle {
		t.Error("Other devs should be unchanged")
	}
}

func TestOfficeState_AdvanceFrames(t *testing.T) {
	devIDs := []string{"dev-1", "dev-2", "dev-3"}
	state := NewOfficeState(devIDs)

	// Set one dev to Working
	state = state.SetDeveloperState("dev-1", StateWorking)

	// Advance frames
	state = state.AdvanceFrames(testTime)

	// Working dev should advance
	anim1 := state.GetAnimationOption("dev-1").OrZero()
	if anim1.Frame != 1 {
		t.Errorf("Working dev frame = %d, want 1", anim1.Frame)
	}

	// Idle dev should not advance (devs start at cubicles)
	anim2 := state.GetAnimationOption("dev-2").OrZero()
	if anim2.Frame != 0 {
		t.Errorf("Idle dev frame = %d, want 0", anim2.Frame)
	}
}

func TestRenderOffice_MinWidth(t *testing.T) {
	devIDs := []string{"dev-1"}
	state := NewOfficeState(devIDs)
	names := []string{"Mei"}

	output := RenderOffice(state, names, 30, 20) // too narrow (min is 40)

	if output == "" {
		t.Error("RenderOffice should return non-empty string")
	}
	// Should indicate terminal is too narrow
	if !containsSubstring(output, "narrow") {
		t.Errorf("RenderOffice with narrow width should mention 'narrow', got: %q", output)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && containsSubstringHelper(s, substr)))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Step 7: App Integration - ONE integration test per Khorikov

// Step 8: Client Mode - verify degraded mode works

func TestApp_OfficeAnimation_ClientMode(t *testing.T) {
	// Client mode: positions snap, no smooth movement
	// This test verifies that client mode initializes office state correctly
	// even without animation frame messages

	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	app := NewAppWithClient(client, createResp.Simulation)

	// Set window size
	m, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	app = m.(*App)

	// Devs should appear in correct positions (no animation frames needed)
	output := app.View()

	// Client mode developers should appear in the conference room during planning
	if !containsSubstring(output, "CONFERENCE") {
		t.Error("Client mode: should show conference room in planning view")
	}
}

// Step 7: App Integration - ONE integration test per Khorikov

func TestApp_OfficeAnimation_Integration(t *testing.T) {
	app := NewAppWithSeed(42)

	// Helper to send messages
	send := func(msg tea.Msg) {
		m, _ := app.Update(msg)
		app = m.(*App)
	}

	// Set window size (required for rendering)
	send(tea.WindowSizeMsg{Width: 100, Height: 40})

	// 1. Initial: devs should be in cubicles (idle), conference room visible but empty
	output := app.View()
	if !containsSubstring(output, "CONFERENCE") {
		t.Error("Planning view should show conference room")
	}
	// Cubicle grid shows all dev names (Mei is first)
	if !containsSubstring(output, "Mei") {
		t.Error("Initial view should show developer names in cubicles")
	}

	// 2. Assign ticket → dev transitions from idle to moving to working
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	// 3. Advance animation frames (movement animation completes)
	for i := 0; i < 5; i++ {
		send(animationTickMsg{})
	}

	// 4. Start sprint → working animation begins
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	send(animationTickMsg{})
	output = app.View()
	// Working animation uses ○◔◑◕● - check for any working frame
	hasWorkingIcon := containsAny(output, WorkingFrames)
	if !hasWorkingIcon {
		t.Error("After sprint start: should show working animation icon")
	}
}

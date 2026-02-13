package tui

import (
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
)

// Data: Position represents a screen coordinate (value type)
type Position struct {
	X, Y int
}

// Data: AnimationState enum
type AnimationState int

const (
	StateIdle AnimationState = iota
	StateConference
	StateMoving
	StateWorking
	StateFrustrated
)

// Data: DeveloperAnimation is a value type (use value receivers)
type DeveloperAnimation struct {
	DevID       string
	State       AnimationState
	Position    Position // Current screen position
	Target      Position // Destination (for Moving state)
	Frame       int      // Current animation frame
	ColorIndex  int      // 0-5 for palette lookup
	Progress    float64  // 0.0-1.0 for movement interpolation
	FrameOffset int      // Offset for visual variety (devs don't cycle in unison)
}

// Data: OfficeState holds all developer animations
type OfficeState struct {
	Animations []DeveloperAnimation
}

// Working animation frames
var WorkingFrames = []string{"○", "◔", "◑", "◕", "●"}

// Frustration bubble text cycles
var FrustrationText = []string{"!@#", "$%^", "&*!", "#$%"}

// Layout constants
const (
	CubicleWidth   = 10
	CubicleHeight  = 4
	CubicleSpacing = 2
	CubicleStartX  = 40 // Right side of screen
	CubicleStartY  = 2

	ConferenceX      = 5
	ConferenceY      = 3
	ConferenceWidth  = 30
	ConferenceHeight = 5
)

// Calculation: CubicleLayout returns positions for n developers (1-6)
// Pure function: int → []Position
func CubicleLayout(n int) []Position {
	positions := make([]Position, n)
	for i := 0; i < n; i++ {
		row := i / 3 // 0 or 1
		col := i % 3 // 0, 1, or 2
		positions[i] = Position{
			X: CubicleStartX + col*(CubicleWidth+CubicleSpacing),
			Y: CubicleStartY + row*(CubicleHeight+CubicleSpacing),
		}
	}
	return positions
}

// Calculation: ConferencePosition returns center position in conference room
// Pure function: (int, int) → Position
func ConferencePosition(devIndex, total int) Position {
	if total == 0 {
		return Position{X: ConferenceX + ConferenceWidth/2, Y: ConferenceY + 2}
	}
	// Evenly space developers horizontally in conference room
	spacing := ConferenceWidth / (total + 1)
	return Position{
		X: ConferenceX + spacing*(devIndex+1),
		Y: ConferenceY + 2,
	}
}

// Calculation: Lerp linearly interpolates between two positions
// Pure function: (Position, Position, float64) → Position
func Lerp(from, to Position, t float64) Position {
	return Position{
		X: from.X + int(float64(to.X-from.X)*t),
		Y: from.Y + int(float64(to.Y-from.Y)*t),
	}
}

// NewDeveloperAnimation creates a new developer animation in Idle state.
// Calculation: (string, int) → DeveloperAnimation
func NewDeveloperAnimation(devID string, colorIndex int) DeveloperAnimation {
	return DeveloperAnimation{
		DevID:       devID,
		State:       StateIdle,
		ColorIndex:  colorIndex,
		FrameOffset: colorIndex % len(WorkingFrames), // Deterministic offset for visual variety
	}
}

// Value receiver methods for state transitions (return new value)

// WithState returns a new DeveloperAnimation with the given state
func (d DeveloperAnimation) WithState(s AnimationState) DeveloperAnimation {
	d.State = s
	return d
}

// WithPosition returns a new DeveloperAnimation with the given position
func (d DeveloperAnimation) WithPosition(p Position) DeveloperAnimation {
	d.Position = p
	return d
}

// WithTarget returns a new DeveloperAnimation with the given target
func (d DeveloperAnimation) WithTarget(t Position) DeveloperAnimation {
	d.Target = t
	return d
}

// StartMoving begins movement animation from current position to target
func (d DeveloperAnimation) StartMoving(target Position) DeveloperAnimation {
	d.Target = target
	d.State = StateMoving
	d.Progress = 0.0
	return d
}

// NextFrame advances the animation frame, wrapping at the end
func (d DeveloperAnimation) NextFrame() DeveloperAnimation {
	d.Frame = (d.Frame + 1) % len(WorkingFrames)
	return d
}

// CurrentFrame returns the display frame with offset applied (for visual variety).
// Calculation: DeveloperAnimation → int
func (d DeveloperAnimation) CurrentFrame() int {
	return (d.Frame + d.FrameOffset) % len(WorkingFrames)
}

// ShouldBecomeFrustrated returns true if developer should transition to frustrated.
// Predicate: avoids duplicate DevBecameFrustrated events.
func (d DeveloperAnimation) ShouldBecomeFrustrated(actualDays, estimatedDays float64) bool {
	return d.State != StateFrustrated && actualDays > estimatedDays
}

// ShouldStartWorking returns true if developer should transition to working.
// Predicate: for detecting when dev needs DevStartedWorking event.
func (d DeveloperAnimation) ShouldStartWorking() bool {
	return d.State != StateWorking && d.State != StateMoving && d.State != StateFrustrated
}

// IsActive returns true if developer is actively working (not idle/conference).
// Predicate: for SprintEnded to identify devs needing DevEnteredConference event.
func (d DeveloperAnimation) IsActive() bool {
	return d.State == StateWorking || d.State == StateFrustrated
}

// NewOfficeState creates a new OfficeState with animations for all developers.
// Developers start at conference room positions.
// Calculation: []string → OfficeState
func NewOfficeState(devIDs []string) OfficeState {
	anims := make([]DeveloperAnimation, len(devIDs))
	for i, id := range devIDs {
		anim := NewDeveloperAnimation(id, i)
		// Set initial position in conference room
		anim.Position = ConferencePosition(i, len(devIDs))
		anims[i] = anim
	}
	return OfficeState{Animations: anims}
}

// GetAnimationOption returns the animation for a specific developer.
// Calculation: (OfficeState, string) → option.Basic[DeveloperAnimation]
func (s OfficeState) GetAnimationOption(devID string) option.Basic[DeveloperAnimation] {
	// hasDevID returns true if the animation belongs to the specified developer.
	hasDevID := func(a DeveloperAnimation) bool { return a.DevID == devID }
	return slice.From(s.Animations).Find(hasDevID)
}

// GetActiveAnimationOption returns the animation only if it's active.
// Calculation: (OfficeState, string) → option.Basic[DeveloperAnimation]
func (s OfficeState) GetActiveAnimationOption(devID string) option.Basic[DeveloperAnimation] {
	return s.GetAnimationOption(devID).KeepOkIf(DeveloperAnimation.IsActive)
}

// SetDeveloperState returns a new OfficeState with the developer's state changed
func (s OfficeState) SetDeveloperState(devID string, state AnimationState) OfficeState {
	newAnims := make([]DeveloperAnimation, len(s.Animations))
	copy(newAnims, s.Animations)
	for i, anim := range newAnims {
		if anim.DevID == devID {
			newAnims[i] = anim.WithState(state)
			break
		}
	}
	return OfficeState{Animations: newAnims}
}

// StartDeveloperMoving returns a new OfficeState with the developer moving to target
func (s OfficeState) StartDeveloperMoving(devID string, target Position) OfficeState {
	newAnims := make([]DeveloperAnimation, len(s.Animations))
	copy(newAnims, s.Animations)
	for i, anim := range newAnims {
		if anim.DevID == devID {
			newAnims[i] = anim.StartMoving(target)
			break
		}
	}
	return OfficeState{Animations: newAnims}
}

// AdvanceFrames advances animation frames for all working/frustrated developers
// and interpolates movement for Moving developers
func (s OfficeState) AdvanceFrames() OfficeState {
	// advanceFrame advances animation based on current state.
	advanceFrame := func(anim DeveloperAnimation) DeveloperAnimation {
		switch anim.State {
		case StateWorking, StateFrustrated:
			return anim.NextFrame()
		case StateMoving:
			return anim.AdvanceMovement()
		default:
			return anim
		}
	}
	return OfficeState{Animations: slice.From(s.Animations).Convert(advanceFrame)}
}

// AdvanceMovement interpolates position toward target.
// Movement completes in ~500ms (5 frames at 100ms).
func (d DeveloperAnimation) AdvanceMovement() DeveloperAnimation {
	d.Progress += 0.2 // 5 steps to complete
	if d.Progress >= 1.0 {
		d.Progress = 1.0
		d.Position = d.Target
		d.State = StateWorking // Arrived at cubicle
	} else {
		d.Position = Lerp(d.Position, d.Target, d.Progress)
	}
	return d
}

package office

import (
	"time"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
)

// Data: Position represents a screen coordinate (value type)
type Position struct {
	X, Y int
}

// Data: Accessory represents a developer's drink
type Accessory int

const (
	AccessoryNone Accessory = iota
	AccessoryCoffee // ☕
	AccessorySoda   // 🥤
)

// Emoji returns the emoji representation of the accessory.
func (a Accessory) Emoji() string {
	switch a {
	case AccessoryCoffee:
		return "☕"
	case AccessorySoda:
		return "🥤"
	default:
		return ""
	}
}

// Data: SipPhase tracks drink animation state
type SipPhase int

const (
	SipNone SipPhase = iota
	SipPreparing // 😙 kissy lips
	SipDrinking  // drink emoji only
	SipRefreshed // 😌 relieved face
)

// Time constants (frame-rate independent)
const (
	SipPhaseDuration   = 500 * time.Millisecond // Each phase lasts 500ms
	MovementDuration   = 500 * time.Millisecond // Movement completes in 500ms
	LateBubbleDuration = 1 * time.Second        // "Late!" bubble visible for 1s
)

// Data: AnimationState enum
type AnimationState int

const (
	StateIdle AnimationState = iota
	StateConference
	StateMovingToConference
	StateMovingToCubicle
	StateWorking
	StateFrustrated
)

// String returns the string representation of AnimationState.
func (s AnimationState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateConference:
		return "conference"
	case StateMovingToConference:
		return "movingToConference"
	case StateMovingToCubicle:
		return "movingToCubicle"
	case StateWorking:
		return "working"
	case StateFrustrated:
		return "frustrated"
	default:
		return "unknown"
	}
}

// Data: DeveloperAnimation is a value type (use value receivers)
type DeveloperAnimation struct {
	DevID           string
	State           AnimationState
	Position        Position // Current screen position
	Target          Position // Destination (for Moving state)
	Frame           int      // Current animation frame
	ColorIndex      int      // 0-5 for palette lookup
	Progress        float64  // 0.0-1.0 for movement interpolation
	FrameOffset      int       // Offset for visual variety (devs don't cycle in unison)
	LateBubbleFrames int       // Countdown for "Late!" bubble (0 = hidden)
	Accessory        Accessory // Developer's drink (coffee/soda)
	SipPhase         SipPhase  // Current sip animation phase
	SipStartTime     time.Time // When current sip phase started
	MovementStart    time.Time // When movement started (for time-based interpolation)
	LateBubbleStart  time.Time // When "Late!" bubble appeared
}

// Data: OfficeState holds all developer animations
type OfficeState struct {
	Animations []DeveloperAnimation
}

// Data: StaggeredAnimator tracks which developer animates next (value type)
type StaggeredAnimator struct {
	LastChangedIndex int // -1 = start before dev 0
	TicksSinceChange int // For tracking pauses
}

// NextToAnimate returns (newAnimator, devIndex, shouldAnimate).
// devIndex=-1 and shouldAnimate=false when pausing.
// shouldPause is injected by caller for testability.
func (s StaggeredAnimator) NextToAnimate(devCount int, shouldPause bool) (StaggeredAnimator, int, bool) {
	if shouldPause {
		return StaggeredAnimator{
			LastChangedIndex: s.LastChangedIndex,
			TicksSinceChange: s.TicksSinceChange + 1,
		}, -1, false
	}
	nextIndex := (s.LastChangedIndex + 1) % devCount
	return StaggeredAnimator{
		LastChangedIndex: nextIndex,
		TicksSinceChange: 0,
	}, nextIndex, true
}

// Working animation frames - happy/silly faces for on-schedule work
var WorkingFrames = []string{"😊", "😄", "😁", "🙂", "😀"}

// Frustrated animation frames - unhappy faces for over-estimate work
var FrustratedFrames = []string{"😤", "😠", "😡", "😩", "😖"}

// NewDeveloperAnimation creates a new developer animation in Idle state.
// Calculation: (string, int) → DeveloperAnimation
func NewDeveloperAnimation(devID string, colorIndex int) DeveloperAnimation {
	var accessory Accessory
	switch colorIndex {
	case 1: // Amir
		accessory = AccessoryCoffee
	case 3: // Jay
		accessory = AccessorySoda
	}
	return DeveloperAnimation{
		DevID:       devID,
		State:       StateIdle,
		ColorIndex:  colorIndex,
		FrameOffset: colorIndex % len(WorkingFrames), // Deterministic offset for visual variety
		Accessory:   accessory,
	}
}

// Value receiver methods for state transitions (return new value)

// WithState returns a new DeveloperAnimation with the given state
func (d DeveloperAnimation) WithState(s AnimationState) DeveloperAnimation {
	d.State = s
	return d
}

// BecomeFrustrated transitions to frustrated state with "Late!" bubble.
// Bubble shows for ~1 second (10 frames at 100ms).
func (d DeveloperAnimation) BecomeFrustrated() DeveloperAnimation {
	d.State = StateFrustrated
	d.LateBubbleFrames = 10
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

// WithAccessory returns a new DeveloperAnimation with the given accessory.
func (d DeveloperAnimation) WithAccessory(a Accessory) DeveloperAnimation {
	d.Accessory = a
	return d
}

// StartMovingToCubicle begins movement animation toward a cubicle.
func (d DeveloperAnimation) StartMovingToCubicle(target Position, now time.Time) DeveloperAnimation {
	d.Target = target
	d.State = StateMovingToCubicle
	d.Progress = 0.0
	d.MovementStart = now
	return d
}

// StartMovingToConference begins movement animation toward the conference room.
func (d DeveloperAnimation) StartMovingToConference(target Position, now time.Time) DeveloperAnimation {
	d.Target = target
	d.State = StateMovingToConference
	d.Progress = 0.0
	d.MovementStart = now
	return d
}

// NextFrame advances the animation frame, wrapping at the end.
// Also decrements LateBubbleFrames if active.
func (d DeveloperAnimation) NextFrame() DeveloperAnimation {
	d.Frame = (d.Frame + 1) % len(WorkingFrames)
	if d.LateBubbleFrames > 0 {
		d.LateBubbleFrames--
	}
	return d
}

// CurrentFrame returns the display frame with offset applied (for visual variety).
// Calculation: DeveloperAnimation → int
func (d DeveloperAnimation) CurrentFrame() int {
	return (d.Frame + d.FrameOffset) % len(WorkingFrames)
}

// StartSip initiates sip animation if developer has an accessory.
// No-op if AccessoryNone.
func (d DeveloperAnimation) StartSip(now time.Time) DeveloperAnimation {
	if d.Accessory == AccessoryNone {
		return d
	}
	d.SipPhase = SipPreparing
	d.SipStartTime = now
	return d
}

// AdvanceSip cycles sip animation based on elapsed time.
// Transitions: Preparing → Drinking → Refreshed → None.
// No-op if SipPhase is SipNone or insufficient time has elapsed.
func (d DeveloperAnimation) AdvanceSip(now time.Time) DeveloperAnimation {
	if d.SipPhase == SipNone {
		return d
	}
	elapsed := now.Sub(d.SipStartTime)
	if elapsed < SipPhaseDuration {
		return d
	}
	d.SipStartTime = now
	switch d.SipPhase {
	case SipPreparing:
		d.SipPhase = SipDrinking
	case SipDrinking:
		d.SipPhase = SipRefreshed
	case SipRefreshed:
		d.SipPhase = SipNone
	}
	return d
}

// ShouldBecomeFrustrated returns true if developer should transition to frustrated.
// Predicate: avoids duplicate DevBecameFrustrated events.
func (d DeveloperAnimation) ShouldBecomeFrustrated(actualDays, estimatedDays float64) bool {
	return d.State != StateFrustrated && actualDays > estimatedDays
}

// ShouldStartWorking returns true if developer should transition to working.
// Predicate: for detecting when dev needs DevStartedWorking event.
func (d DeveloperAnimation) ShouldStartWorking() bool {
	return d.State != StateWorking &&
		d.State != StateMovingToConference &&
		d.State != StateMovingToCubicle &&
		d.State != StateFrustrated
}

// IsActive returns true if developer is actively working (not idle/conference).
// Predicate: for SprintEnded to identify devs needing DevEnteredConference event.
func (d DeveloperAnimation) IsActive() bool {
	return d.State == StateWorking || d.State == StateFrustrated
}

// IsInConference returns true if developer is in conference room.
func (d DeveloperAnimation) IsInConference() bool {
	return d.State == StateConference
}

// IsAway returns true if developer is not physically in their cubicle.
// This includes being in conference or moving between locations.
func (d DeveloperAnimation) IsAway() bool {
	return d.State == StateConference ||
		d.State == StateMovingToConference ||
		d.State == StateMovingToCubicle
}

// AdvanceMovement interpolates position toward target using time-based animation.
// Movement completes after MovementDuration (500ms).
func (d DeveloperAnimation) AdvanceMovement(now time.Time) DeveloperAnimation {
	elapsed := now.Sub(d.MovementStart)
	d.Progress = float64(elapsed) / float64(MovementDuration)
	if d.Progress >= 1.0 {
		d.Progress = 1.0
		d.Position = d.Target
		if d.State == StateMovingToConference {
			d.State = StateConference
		} else {
			d.State = StateWorking
		}
	} else {
		d.Position = Lerp(d.Position, d.Target, d.Progress)
	}
	return d
}

// NewOfficeState creates a new OfficeState with animations for all developers.
// Developers start at their cubicles (StateIdle). Use DevEnteredConference events
// to move them to the conference room when a sprint starts.
// Calculation: []string → OfficeState
func NewOfficeState(devIDs []string) OfficeState {
	cubiclePositions := CubicleLayout(len(devIDs))
	anims := make([]DeveloperAnimation, len(devIDs))
	for i, id := range devIDs {
		anim := NewDeveloperAnimation(id, i)
		anim.State = StateIdle
		anim.Position = cubiclePositions[i]
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

// SetDeveloperState returns a new OfficeState with the developer's state changed.
// Uses BecomeFrustrated for StateFrustrated to show "Late!" bubble.
func (s OfficeState) SetDeveloperState(devID string, state AnimationState) OfficeState {
	// applyState applies state change to matching developer, passes others through.
	applyState := func(anim DeveloperAnimation) DeveloperAnimation {
		if anim.DevID != devID {
			return anim
		}
		if state == StateFrustrated {
			return anim.BecomeFrustrated()
		}
		return anim.WithState(state)
	}
	return OfficeState{Animations: slice.From(s.Animations).Convert(applyState)}
}

// StartDeveloperMovingToCubicle returns a new OfficeState with the developer moving to cubicle.
func (s OfficeState) StartDeveloperMovingToCubicle(devID string, target Position, now time.Time) OfficeState {
	// startMoving applies movement to matching developer, passes others through.
	startMoving := func(anim DeveloperAnimation) DeveloperAnimation {
		if anim.DevID != devID {
			return anim
		}
		return anim.StartMovingToCubicle(target, now)
	}
	return OfficeState{Animations: slice.From(s.Animations).Convert(startMoving)}
}

// StartDeveloperMovingToConference returns a new OfficeState with the developer moving to conference.
func (s OfficeState) StartDeveloperMovingToConference(devID string, target Position, now time.Time) OfficeState {
	// startMoving applies movement to matching developer, passes others through.
	startMoving := func(anim DeveloperAnimation) DeveloperAnimation {
		if anim.DevID != devID {
			return anim
		}
		return anim.StartMovingToConference(target, now)
	}
	return OfficeState{Animations: slice.From(s.Animations).Convert(startMoving)}
}

// AdvanceFrames advances animation for developers.
// devIdxToAdvance: -1 = pause (no faces advance), >=0 = only that dev's face advances.
// Movement interpolation always advances for all moving devs regardless of devIdxToAdvance.
// LateBubbleFrames decrements for ALL working/frustrated devs (not just staggered one).
func (s OfficeState) AdvanceFrames(now time.Time, devIdxToAdvance int) OfficeState {
	anims := make([]DeveloperAnimation, len(s.Animations))
	for i, anim := range s.Animations {
		switch anim.State {
		case StateWorking, StateFrustrated:
			if i == devIdxToAdvance {
				anims[i] = anim.NextFrame()
			} else {
				// Still decrement LateBubbleFrames for non-staggered devs
				if anim.LateBubbleFrames > 0 {
					anim.LateBubbleFrames--
				}
				anims[i] = anim
			}
		case StateMovingToConference, StateMovingToCubicle:
			anims[i] = anim.AdvanceMovement(now)
		default:
			anims[i] = anim
		}
	}
	return OfficeState{Animations: anims}
}

// ClearBubbles returns a new OfficeState with all LateBubbleFrames zeroed.
func (s OfficeState) ClearBubbles() OfficeState {
	anims := make([]DeveloperAnimation, len(s.Animations))
	for i, anim := range s.Animations {
		anim.LateBubbleFrames = 0
		anims[i] = anim
	}
	return OfficeState{Animations: anims}
}

// startDevSip returns a new OfficeState with the developer's sip animation started.
func (s OfficeState) startDevSip(devID string, now time.Time) OfficeState {
	// apply starts sip for matching developer, passes others through.
	apply := func(anim DeveloperAnimation) DeveloperAnimation {
		if anim.DevID != devID {
			return anim
		}
		return anim.StartSip(now)
	}
	return OfficeState{Animations: slice.From(s.Animations).Convert(apply)}
}

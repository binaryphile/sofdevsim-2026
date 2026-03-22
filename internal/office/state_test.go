package office

import (
	"testing"
	"time"
)

// testTime is a fixed time for deterministic tests.
var testTime = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func TestShouldBecomeFrustrated(t *testing.T) {
	tests := []struct {
		name      string
		state     AnimationState
		actual    float64
		estimated float64
		want      bool
	}{
		{"idle, over estimate", StateIdle, 3.0, 2.0, true},
		{"working, over estimate", StateWorking, 3.0, 2.0, true},
		{"already frustrated", StateFrustrated, 3.0, 2.0, false}, // key case: don't re-trigger
		{"under estimate", StateWorking, 1.0, 2.0, false},
		{"at estimate", StateWorking, 2.0, 2.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := DeveloperAnimation{State: tt.state}
			got := anim.ShouldBecomeFrustrated(tt.actual, tt.estimated)
			if got != tt.want {
				t.Errorf("ShouldBecomeFrustrated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldStartWorking(t *testing.T) {
	tests := []struct {
		name  string
		state AnimationState
		want  bool
	}{
		{"idle", StateIdle, true},
		{"conference", StateConference, true},
		{"already working", StateWorking, false},              // key case: don't re-trigger
		{"movingToConference", StateMovingToConference, false}, // still in transit
		{"movingToCubicle", StateMovingToCubicle, false},       // still in transit
		{"frustrated", StateFrustrated, false},                 // don't downgrade from frustrated
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := DeveloperAnimation{State: tt.state}
			got := anim.ShouldStartWorking()
			if got != tt.want {
				t.Errorf("ShouldStartWorking() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsActive(t *testing.T) {
	tests := []struct {
		name  string
		state AnimationState
		want  bool
	}{
		{"working", StateWorking, true},
		{"frustrated", StateFrustrated, true},
		{"idle", StateIdle, false},
		{"conference", StateConference, false},
		{"movingToConference", StateMovingToConference, false},
		{"movingToCubicle", StateMovingToCubicle, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := DeveloperAnimation{State: tt.state}
			got := anim.IsActive()
			if got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAway(t *testing.T) {
	tests := []struct {
		name  string
		state AnimationState
		want  bool
	}{
		{"conference", StateConference, true},
		{"movingToConference", StateMovingToConference, true},
		{"movingToCubicle", StateMovingToCubicle, true},
		{"working", StateWorking, false},
		{"frustrated", StateFrustrated, false},
		{"idle", StateIdle, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := DeveloperAnimation{State: tt.state}
			got := anim.IsAway()
			if got != tt.want {
				t.Errorf("IsAway() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccessory_Emoji(t *testing.T) {
	tests := []struct {
		a    Accessory
		want string
	}{
		{AccessoryNone, ""},
		{AccessoryCoffee, "☕"},
		{AccessorySoda, "🥤"},
	}
	for _, tt := range tests {
		if got := tt.a.Emoji(); got != tt.want {
			t.Errorf("Accessory(%d).Emoji() = %q, want %q", tt.a, got, tt.want)
		}
	}
}

func TestNewDeveloperAnimation_Accessories(t *testing.T) {
	tests := []struct {
		colorIndex int
		want       Accessory
	}{
		{0, AccessoryNone},   // Mei - no accessory
		{1, AccessoryCoffee}, // Amir - coffee
		{2, AccessoryNone},   // Suki - no accessory
		{3, AccessorySoda},   // Jay - soda
		{4, AccessoryNone},   // Priya - no accessory
		{5, AccessoryNone},   // Kofi - no accessory
	}
	for _, tt := range tests {
		anim := NewDeveloperAnimation("dev", tt.colorIndex)
		if anim.Accessory != tt.want {
			t.Errorf("colorIndex %d: Accessory = %v, want %v", tt.colorIndex, anim.Accessory, tt.want)
		}
	}
}

func TestStaggeredAnimator_RoundRobin(t *testing.T) {
	s := StaggeredAnimator{LastChangedIndex: -1}
	for i := 0; i < 6; i++ { // justified:SM
		var idx int
		var ok bool
		s, idx, ok = s.NextToAnimate(3, false)
		if !ok || idx != i%3 {
			t.Errorf("tick %d: idx=%d ok=%v, want idx=%d ok=true", i, idx, ok, i%3)
		}
	}
}

func TestStaggeredAnimator_Pauses(t *testing.T) {
	s := StaggeredAnimator{LastChangedIndex: 0}
	s, idx, ok := s.NextToAnimate(3, true)
	if ok || idx != -1 {
		t.Errorf("pause: idx=%d ok=%v, want idx=-1 ok=false", idx, ok)
	}
	if s.TicksSinceChange != 1 {
		t.Errorf("TicksSinceChange=%d, want 1", s.TicksSinceChange)
	}
}

func TestAdvanceMovement_ToConference(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	d := DeveloperAnimation{
		State:         StateMovingToConference,
		MovementStart: start,
	}
	// Advance past MovementDuration
	d = d.AdvanceMovement(start.Add(MovementDuration + time.Millisecond))
	if d.State != StateConference {
		t.Errorf("State = %v, want StateConference", d.State)
	}
}

func TestAdvanceMovement_ToCubicle(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	d := DeveloperAnimation{
		State:         StateMovingToCubicle,
		MovementStart: start,
	}
	d = d.AdvanceMovement(start.Add(MovementDuration + time.Millisecond))
	if d.State != StateWorking {
		t.Errorf("State = %v, want StateWorking", d.State)
	}
}

func TestAdvanceMovement_MidProgress(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	d := DeveloperAnimation{
		State:         StateMovingToCubicle,
		Position:      Position{X: 0, Y: 0},
		Target:        Position{X: 100, Y: 50},
		MovementStart: start,
	}

	// Advance to 50% of MovementDuration
	d = d.AdvanceMovement(start.Add(MovementDuration / 2))

	if d.State != StateMovingToCubicle {
		t.Errorf("Mid-progress: State = %v, want StateMovingToCubicle", d.State)
	}
	if d.Progress < 0.49 || d.Progress > 0.51 {
		t.Errorf("Mid-progress: Progress = %f, want ~0.5", d.Progress)
	}
	// Position should be interpolated (Lerp)
	if d.Position.X < 40 || d.Position.X > 60 {
		t.Errorf("Mid-progress: Position.X = %d, want ~50", d.Position.X)
	}
}

func TestStartMovingToConference(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	d := DeveloperAnimation{
		State:    StateWorking,
		Position: Position{X: 50, Y: 10},
	}
	target := Position{X: 5, Y: 5}

	d = d.StartMovingToConference(target, start)

	if d.State != StateMovingToConference {
		t.Errorf("State = %v, want StateMovingToConference", d.State)
	}
	if d.Target != target {
		t.Errorf("Target = %v, want %v", d.Target, target)
	}
	if d.Progress != 0.0 {
		t.Errorf("Progress = %f, want 0.0", d.Progress)
	}
	if d.MovementStart != start {
		t.Errorf("MovementStart = %v, want %v", d.MovementStart, start)
	}
}

func TestStartSip(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	anim := DeveloperAnimation{Accessory: AccessoryCoffee}
	anim = anim.StartSip(now)

	if anim.SipPhase != SipPreparing {
		t.Errorf("SipPhase = %v, want SipPreparing", anim.SipPhase)
	}
	if anim.SipStartTime != now {
		t.Errorf("SipStartTime = %v, want %v", anim.SipStartTime, now)
	}
}

func TestStartSip_NoAccessory(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	anim := DeveloperAnimation{Accessory: AccessoryNone}
	anim = anim.StartSip(now)
	if anim.SipPhase != SipNone {
		t.Errorf("SipPhase = %v, want SipNone (no accessory)", anim.SipPhase)
	}
}

func TestAdvanceSip_FullCycle(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	anim := DeveloperAnimation{
		Accessory:    AccessoryCoffee,
		SipPhase:     SipPreparing,
		SipStartTime: start,
	}

	// Preparing → Drinking
	now := start.Add(SipPhaseDuration + time.Millisecond)
	anim = anim.AdvanceSip(now)
	if anim.SipPhase != SipDrinking {
		t.Errorf("after 1st advance: SipPhase = %v, want SipDrinking", anim.SipPhase)
	}

	// Drinking → Refreshed
	now = now.Add(SipPhaseDuration + time.Millisecond)
	anim = anim.AdvanceSip(now)
	if anim.SipPhase != SipRefreshed {
		t.Errorf("after 2nd advance: SipPhase = %v, want SipRefreshed", anim.SipPhase)
	}

	// Refreshed → None
	now = now.Add(SipPhaseDuration + time.Millisecond)
	anim = anim.AdvanceSip(now)
	if anim.SipPhase != SipNone {
		t.Errorf("after 3rd advance: SipPhase = %v, want SipNone", anim.SipPhase)
	}
}

func TestAdvanceSip_NoOpWhenNone(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	anim := DeveloperAnimation{SipPhase: SipNone}
	anim = anim.AdvanceSip(now)
	if anim.SipPhase != SipNone {
		t.Errorf("SipPhase = %v, want SipNone (no-op)", anim.SipPhase)
	}
}

// Cycle 1: AdvanceFrames with devIdxToAdvance parameter
func TestAdvanceFrames_StaggeredSingleDev(t *testing.T) {
	state := NewOfficeState([]string{"dev-0", "dev-1", "dev-2"})
	state = state.SetDeveloperState("dev-0", StateWorking)
	state = state.SetDeveloperState("dev-1", StateWorking)
	state = state.SetDeveloperState("dev-2", StateWorking)

	state = state.AdvanceFrames(testTime, 1)

	if state.Animations[0].Frame != 0 {
		t.Error("dev-0 should not advance")
	}
	if state.Animations[1].Frame != 1 {
		t.Error("dev-1 should advance")
	}
	if state.Animations[2].Frame != 0 {
		t.Error("dev-2 should not advance")
	}
}

// Cycle 2: Pause (devIdx=-1) means no faces advance
func TestAdvanceFrames_PauseNoFaces(t *testing.T) {
	state := NewOfficeState([]string{"dev-0", "dev-1"})
	state = state.SetDeveloperState("dev-0", StateWorking)
	state = state.SetDeveloperState("dev-1", StateWorking)

	state = state.AdvanceFrames(testTime, -1)

	if state.Animations[0].Frame != 0 {
		t.Error("dev-0 should not advance during pause")
	}
	if state.Animations[1].Frame != 0 {
		t.Error("dev-1 should not advance during pause")
	}
}

// Cycle 3: Movement still advances even during pause
func TestAdvanceFrames_MovementStillAdvances(t *testing.T) {
	state := NewOfficeState([]string{"dev-0"})
	state = state.StartDeveloperMovingToCubicle("dev-0", Position{50, 2}, testTime)

	// Advance past MovementDuration with pause (-1)
	state = state.AdvanceFrames(testTime.Add(600*time.Millisecond), -1)

	if state.Animations[0].State != StateWorking {
		t.Errorf("State = %v, want StateWorking (should arrive even during pause)", state.Animations[0].State)
	}
}

// Cycle 4: LateBubbleFrames decrements for ALL devs, not just staggered one
func TestAdvanceFrames_LateBubbleDecrementsForAll(t *testing.T) {
	state := NewOfficeState([]string{"dev-0", "dev-1"})
	// SetDeveloperState calls BecomeFrustrated for StateFrustrated, setting LateBubbleFrames=10
	state = state.SetDeveloperState("dev-0", StateFrustrated)
	state = state.SetDeveloperState("dev-1", StateFrustrated)

	// Only dev-0 face advances, but both should decrement LateBubbleFrames
	state = state.AdvanceFrames(testTime, 0)

	anim1 := state.GetAnimationOption("dev-1").OrZero()
	if anim1.LateBubbleFrames != 9 {
		t.Errorf("dev-1 LateBubbleFrames = %d, want 9 (should decrement even though face didn't advance)", anim1.LateBubbleFrames)
	}
}

func TestAdvanceFrames_AdvancesSipForConferenceDev(t *testing.T) {
	state := NewOfficeState([]string{"dev-0", "dev-1"})
	// dev-1 (colorIndex 1) has coffee accessory
	state = state.SetDeveloperState("dev-1", StateConference)

	// Start a sip on dev-1
	anim := state.Animations[1].StartSip(testTime)
	state.Animations[1] = anim

	if state.Animations[1].SipPhase != SipPreparing {
		t.Fatalf("precondition: expected SipPreparing, got %v", state.Animations[1].SipPhase)
	}

	// Advance past SipPhaseDuration
	state = state.AdvanceFrames(testTime.Add(SipPhaseDuration+time.Millisecond), -1)

	if state.Animations[1].SipPhase != SipDrinking {
		t.Errorf("SipPhase = %v, want SipDrinking (AdvanceFrames should advance sip for conference devs)", state.Animations[1].SipPhase)
	}
}

func TestClearBubbles(t *testing.T) {
	anim := NewDeveloperAnimation("dev-1", 0).BecomeFrustrated()
	if anim.LateBubbleFrames != 10 {
		t.Fatalf("precondition: expected 10, got %d", anim.LateBubbleFrames)
	}
	state := OfficeState{Animations: []DeveloperAnimation{anim}}
	state = state.ClearBubbles()
	if state.Animations[0].LateBubbleFrames != 0 {
		t.Errorf("expected 0 after ClearBubbles, got %d", state.Animations[0].LateBubbleFrames)
	}
}

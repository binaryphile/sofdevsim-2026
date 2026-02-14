package office

import (
	"testing"
	"time"
)

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
	for i := 0; i < 6; i++ {
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

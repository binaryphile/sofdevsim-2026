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

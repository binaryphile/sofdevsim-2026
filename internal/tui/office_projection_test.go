package tui

import (
	"testing"
	"time"

	"github.com/binaryphile/fluentfp/slice"
)

// testTime is a fixed time for deterministic tests.
var testTime = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func TestOfficeProjection_Empty(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1", "dev-2"})
	state := proj.State()

	if len(state.Animations) != 2 {
		t.Errorf("Animations count = %d, want 2", len(state.Animations))
	}

	anim, ok := state.GetAnimationOption("dev-1").Get()
	if !ok {
		t.Fatal("dev-1 not found")
	}
	// Developers start at their cubicles (Idle), move to conference via events
	if anim.State != StateIdle {
		t.Errorf("Initial state = %v, want StateIdle", anim.State)
	}
}

func TestOfficeProjection_AssignedMoving(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevAssignedToTicket{
		DevID:    "dev-1",
		TicketID: "TKT-1",
		Target:   Position{X: 50, Y: 2},
	}, 1, testTime)

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateMovingToCubicle {
		t.Errorf("State = %v, want StateMovingToCubicle", anim.State)
	}
	if anim.Target.X != 50 || anim.Target.Y != 2 {
		t.Errorf("Target = %v, want {50, 2}", anim.Target)
	}
}

func TestOfficeProjection_MovementComplete(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	now := testTime
	proj = proj.Record(DevAssignedToTicket{
		DevID:    "dev-1",
		TicketID: "TKT-1",
		Target:   Position{X: 50, Y: 2},
	}, 1, now)

	// Advance time past MovementDuration (500ms)
	now = now.Add(600 * time.Millisecond)
	proj = proj.Record(AnimationFrameAdvanced{}, 1, now)

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateWorking {
		t.Errorf("After movement duration: State = %v, want StateWorking", anim.State)
	}
}

func TestOfficeProjection_StartedWorking(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"}, 1, testTime)

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateWorking {
		t.Errorf("State = %v, want StateWorking", anim.State)
	}
}

func TestOfficeProjection_Frustrated(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"}, 1, testTime)
	proj = proj.Record(DevBecameFrustrated{DevID: "dev-1", TicketID: "TKT-1"}, 2, testTime)

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateFrustrated {
		t.Errorf("State = %v, want StateFrustrated", anim.State)
	}
}

func TestOfficeProjection_Completed(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"}, 1, testTime)
	proj = proj.Record(DevCompletedTicket{DevID: "dev-1", TicketID: "TKT-1"}, 5, testTime)

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	// After completing ticket, dev goes to Idle (not Conference - that's for sprint end)
	if anim.State != StateIdle {
		t.Errorf("State = %v, want StateIdle", anim.State)
	}
}

func TestOfficeProjection_EnteredConference(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"}, 1, testTime)
	proj = proj.Record(DevEnteredConference{DevID: "dev-1"}, 10, testTime)

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateConference {
		t.Errorf("State = %v, want StateConference", anim.State)
	}
}

func TestOfficeProjection_FrameAdvance(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"}, 1, testTime)

	// Advance 3 frames
	for i := 0; i < 3; i++ {
		proj = proj.Record(AnimationFrameAdvanced{}, 1, testTime)
	}

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.Frame != 3 {
		t.Errorf("Frame = %d, want 3", anim.Frame)
	}
}

func TestOfficeProjection_Events(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevAssignedToTicket{DevID: "dev-1", TicketID: "TKT-1", Target: Position{50, 2}}, 1, testTime)
	proj = proj.Record(AnimationFrameAdvanced{}, 1, testTime)
	proj = proj.Record(DevBecameFrustrated{DevID: "dev-1", TicketID: "TKT-1"}, 2, testTime)

	events := proj.Events()
	if len(events) != 3 {
		t.Errorf("Events() len = %d, want 3", len(events))
	}
	if _, ok := events[0].(DevAssignedToTicket); !ok {
		t.Error("First event should be DevAssignedToTicket")
	}
	if _, ok := events[1].(AnimationFrameAdvanced); !ok {
		t.Error("Second event should be AnimationFrameAdvanced")
	}
	if _, ok := events[2].(DevBecameFrustrated); !ok {
		t.Error("Third event should be DevBecameFrustrated")
	}
}

func TestOfficeProjection_ImmutableRecord(t *testing.T) {
	proj1 := NewOfficeProjection([]string{"dev-1"})
	proj2 := proj1.Record(DevStartedWorking{DevID: "dev-1"}, 1, testTime)

	// Original unchanged (starts at cubicle/Idle)
	state1 := proj1.State()
	anim1 := state1.GetAnimationOption("dev-1").OrZero()
	if anim1.State != StateIdle {
		t.Error("Original projection should remain unchanged")
	}

	// New has change
	state2 := proj2.State()
	anim2 := state2.GetAnimationOption("dev-1").OrZero()
	if anim2.State != StateWorking {
		t.Error("New projection should have working state")
	}
}

func TestOfficeProjection_MultipleDevsConference(t *testing.T) {
	devIDs := []string{"dev-1", "dev-2", "dev-3"}

	// Replicate app.go initialization pattern
	recordConferenceEntry := func(proj OfficeProjection, id string) OfficeProjection {
		return proj.Record(DevEnteredConference{DevID: id}, 0, testTime)
	}
	proj := slice.Fold(devIDs, NewOfficeProjection(devIDs), recordConferenceEntry)

	state := proj.State()

	if len(state.Animations) != 3 {
		t.Fatalf("Animations count = %d, want 3", len(state.Animations))
	}

	for _, anim := range state.Animations {
		if anim.State != StateConference {
			t.Errorf("%s: State = %v, want StateConference", anim.DevID, anim.State)
		}
	}
}

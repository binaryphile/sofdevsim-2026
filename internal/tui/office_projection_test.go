package tui

import (
	"testing"

	"github.com/binaryphile/fluentfp/slice"
)

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
	})

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateMoving {
		t.Errorf("State = %v, want StateMoving", anim.State)
	}
	if anim.Target.X != 50 || anim.Target.Y != 2 {
		t.Errorf("Target = %v, want {50, 2}", anim.Target)
	}
}

func TestOfficeProjection_MovementComplete(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevAssignedToTicket{
		DevID:    "dev-1",
		TicketID: "TKT-1",
		Target:   Position{X: 50, Y: 2},
	})

	// 5 frame advances complete movement (Progress += 0.2 each)
	for i := 0; i < 5; i++ {
		proj = proj.Record(AnimationFrameAdvanced{})
	}

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateWorking {
		t.Errorf("After 5 advances: State = %v, want StateWorking", anim.State)
	}
}

func TestOfficeProjection_StartedWorking(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"})

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateWorking {
		t.Errorf("State = %v, want StateWorking", anim.State)
	}
}

func TestOfficeProjection_Frustrated(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"})
	proj = proj.Record(DevBecameFrustrated{DevID: "dev-1", TicketID: "TKT-1"})

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateFrustrated {
		t.Errorf("State = %v, want StateFrustrated", anim.State)
	}
}

func TestOfficeProjection_Completed(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"})
	proj = proj.Record(DevCompletedTicket{DevID: "dev-1", TicketID: "TKT-1"})

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateIdle {
		t.Errorf("State = %v, want StateIdle", anim.State)
	}
}

func TestOfficeProjection_EnteredConference(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"})
	proj = proj.Record(DevEnteredConference{DevID: "dev-1"})

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.State != StateConference {
		t.Errorf("State = %v, want StateConference", anim.State)
	}
}

func TestOfficeProjection_FrameAdvance(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevStartedWorking{DevID: "dev-1"})

	// Advance 3 frames
	for i := 0; i < 3; i++ {
		proj = proj.Record(AnimationFrameAdvanced{})
	}

	state := proj.State()
	anim := state.GetAnimationOption("dev-1").OrZero()

	if anim.Frame != 3 {
		t.Errorf("Frame = %d, want 3", anim.Frame)
	}
}

func TestOfficeProjection_Events(t *testing.T) {
	proj := NewOfficeProjection([]string{"dev-1"})
	proj = proj.Record(DevAssignedToTicket{DevID: "dev-1", TicketID: "TKT-1", Target: Position{50, 2}})
	proj = proj.Record(AnimationFrameAdvanced{})
	proj = proj.Record(DevBecameFrustrated{DevID: "dev-1", TicketID: "TKT-1"})

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
	proj2 := proj1.Record(DevStartedWorking{DevID: "dev-1"})

	// Original unchanged
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
		return proj.Record(DevEnteredConference{DevID: id})
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

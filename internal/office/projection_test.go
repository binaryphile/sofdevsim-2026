package office

import (
	"testing"
	"time"
)

func TestRecord_DevStartedSip(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	// dev-1 at index 1 has colorIndex=1 (Amir) who gets coffee accessory
	proj := NewOfficeProjection([]string{"dev-0", "dev-1"})

	proj = proj.Record(DevStartedSip{DevID: "dev-1"}, 0, now)

	anim, _ := proj.State().GetAnimationOption("dev-1").Get()
	if anim.SipPhase != SipPreparing {
		t.Errorf("SipPhase = %v, want SipPreparing", anim.SipPhase)
	}
	if anim.SipStartTime != now {
		t.Errorf("SipStartTime = %v, want %v", anim.SipStartTime, now)
	}
}

func TestRecord_DevStartedMovingToConference(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	proj := NewOfficeProjection([]string{"dev-0", "dev-1"})
	target := Position{X: 5, Y: 5}

	proj = proj.Record(DevStartedMovingToConference{DevID: "dev-0", Target: target}, 0, now)

	anim, ok := proj.State().GetAnimationOption("dev-0").Get()
	if !ok {
		t.Fatal("dev-0 not found in projection")
	}
	if anim.State != StateMovingToConference {
		t.Errorf("State = %v, want StateMovingToConference", anim.State)
	}
	if anim.Target != target {
		t.Errorf("Target = %v, want %v", anim.Target, target)
	}
	if anim.MovementStart != now {
		t.Errorf("MovementStart = %v, want %v", anim.MovementStart, now)
	}
}

func TestRecord_DevStartedMovingToConference_Transition(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	proj := NewOfficeProjection([]string{"dev-0"})
	target := Position{X: 5, Y: 5}

	proj = proj.Record(DevStartedMovingToConference{DevID: "dev-0", Target: target}, 1, now)

	transitions := proj.Transitions()
	if len(transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(transitions))
	}
	tr := transitions[0]
	if tr.FromState != StateIdle.String() {
		t.Errorf("FromState = %q, want %q", tr.FromState, StateIdle.String())
	}
	if tr.ToState != StateMovingToConference.String() {
		t.Errorf("ToState = %q, want %q", tr.ToState, StateMovingToConference.String())
	}
	if tr.Reason != "opening animation" {
		t.Errorf("Reason = %q, want %q", tr.Reason, "opening animation")
	}
}

func TestRecord_DevStartedSip_NoAccessory(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	proj := NewOfficeProjection([]string{"dev-0"}) // index 0 = no accessory

	proj = proj.Record(DevStartedSip{DevID: "dev-0"}, 0, now)

	anim, _ := proj.State().GetAnimationOption("dev-0").Get()
	if anim.SipPhase != SipNone {
		t.Errorf("SipPhase = %v, want SipNone (no accessory)", anim.SipPhase)
	}
}

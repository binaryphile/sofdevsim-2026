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

func TestRecord_DevStartedSip_NoAccessory(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	proj := NewOfficeProjection([]string{"dev-0"}) // index 0 = no accessory

	proj = proj.Record(DevStartedSip{DevID: "dev-0"}, 0, now)

	anim, _ := proj.State().GetAnimationOption("dev-0").Get()
	if anim.SipPhase != SipNone {
		t.Errorf("SipPhase = %v, want SipNone (no accessory)", anim.SipPhase)
	}
}

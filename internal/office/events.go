package office

// Data: OfficeEvent is the sealed interface for animation events.
// All events are immutable value types with past-tense naming.
// These are session-scoped (not persisted) - used for debugging animation state.
type OfficeEvent interface {
	officeEvent() // sealed marker
}

// DevAssignedToTicket: developer starts moving from conference to cubicle.
type DevAssignedToTicket struct {
	DevID    string
	TicketID string
	Target   Position
}

func (DevAssignedToTicket) officeEvent() {}

// DevStartedWorking: developer began working (after arriving or resuming).
type DevStartedWorking struct {
	DevID string
}

func (DevStartedWorking) officeEvent() {}

// DevBecameFrustrated: developer's ticket exceeded estimate (ActualDays > EstimatedDays).
type DevBecameFrustrated struct {
	DevID    string
	TicketID string
}

func (DevBecameFrustrated) officeEvent() {}

// DevCompletedTicket: developer finished ticket, returns to idle.
type DevCompletedTicket struct {
	DevID    string
	TicketID string
}

func (DevCompletedTicket) officeEvent() {}

// DevEnteredConference: developer returned to conference room (sprint ended or idle).
type DevEnteredConference struct {
	DevID string
}

func (DevEnteredConference) officeEvent() {}

// AnimationFrameAdvanced: 100ms tick for animation frame updates.
// DevIdxToAdvance specifies which developer's face advances this tick.
// -1 = pause (no faces advance), >=0 = that developer index only.
type AnimationFrameAdvanced struct {
	DevIdxToAdvance int
}

func (AnimationFrameAdvanced) officeEvent() {}

// DevStartedSip: developer began sip animation (has accessory, in valid state).
type DevStartedSip struct {
	DevID string
}

func (DevStartedSip) officeEvent() {}

// BubblesExpired: clears all Late! bubble countdowns.
// Fired at the start of each simulation tick in the REST API path,
// so bubbles persist for exactly one tick.
type BubblesExpired struct{}

func (BubblesExpired) officeEvent() {}

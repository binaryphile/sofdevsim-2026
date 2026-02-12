package tui

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
type AnimationFrameAdvanced struct{}

func (AnimationFrameAdvanced) officeEvent() {}

package tui

// Data: OfficeProjection accumulates animation events (ephemeral, session-scoped).
// State is computed on Record(), stored, and returned by State() - matches engine.Projection pattern.
type OfficeProjection struct {
	events []OfficeEvent
	state  OfficeState // memoized: computed on Record(), returned by State()
}

// Calculation: NewOfficeProjection creates a projection with developer list.
func NewOfficeProjection(devIDs []string) OfficeProjection {
	return OfficeProjection{state: NewOfficeState(devIDs)}
}

// Calculation: Record returns a NEW projection with the event applied.
// Computes new state immediately (compute-on-write pattern).
// Value semantics - does not mutate the receiver.
func (p OfficeProjection) Record(evt OfficeEvent) OfficeProjection {
	newEvents := make([]OfficeEvent, len(p.events)+1)
	copy(newEvents, p.events)
	newEvents[len(p.events)] = evt
	return OfficeProjection{
		events: newEvents,
		state:  applyOfficeEvent(p.state, evt), // compute on write
	}
}

// Calculation: State returns the current OfficeState (memoized).
func (p OfficeProjection) State() OfficeState {
	return p.state
}

// Calculation: Events returns the event history for debugging.
func (p OfficeProjection) Events() []OfficeEvent {
	return p.events
}

// Calculation: applyOfficeEvent applies one event to state.
// Pure function: (OfficeState, OfficeEvent) → OfficeState
func applyOfficeEvent(state OfficeState, evt OfficeEvent) OfficeState {
	switch e := evt.(type) {
	case DevAssignedToTicket:
		return state.StartDeveloperMoving(e.DevID, e.Target)
	case DevStartedWorking:
		return state.SetDeveloperState(e.DevID, StateWorking)
	case DevBecameFrustrated:
		return state.SetDeveloperState(e.DevID, StateFrustrated)
	case DevCompletedTicket:
		return state.SetDeveloperState(e.DevID, StateIdle)
	case DevEnteredConference:
		return state.SetDeveloperState(e.DevID, StateConference)
	case AnimationFrameAdvanced:
		return state.AdvanceFrames()
	default:
		return state
	}
}

package office

import "time"

// Data: StateTransition records a developer state change with timing context.
type StateTransition struct {
	DevID     string    `json:"devId"`
	FromState string    `json:"fromState"`
	ToState   string    `json:"toState"`
	Tick      int       `json:"tick"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
}

// Data: OfficeProjection accumulates animation events (ephemeral, session-scoped).
// State is computed on Record(), stored, and returned by State() - matches engine.Projection pattern.
type OfficeProjection struct {
	events      []OfficeEvent
	state       OfficeState       // memoized: computed on Record(), returned by State()
	transitions []StateTransition // state change history with timing
	currentTick int               // current simulation tick
}

// Calculation: NewOfficeProjection creates a projection with developer list.
func NewOfficeProjection(devIDs []string) OfficeProjection {
	return OfficeProjection{state: NewOfficeState(devIDs)}
}

// Calculation: Record returns a NEW projection with the event applied.
// Computes new state immediately (compute-on-write pattern).
// Value semantics - does not mutate the receiver.
// now is injected for time-based animations (caller passes time.Now() in production).
func (p OfficeProjection) Record(evt OfficeEvent, tick int, now time.Time) OfficeProjection {
	newEvents := make([]OfficeEvent, len(p.events)+1)
	copy(newEvents, p.events)
	newEvents[len(p.events)] = evt

	newState := applyOfficeEvent(p.state, evt, now)
	newTransitions := p.detectTransitions(evt, newState, tick, now)

	return OfficeProjection{
		events:      newEvents,
		state:       newState,
		transitions: newTransitions,
		currentTick: tick,
	}
}

// Calculation: detectTransitions identifies state changes from an event.
// Returns new transitions slice with any new transitions appended.
// now is injected for testability (no internal time.Now() calls).
func (p OfficeProjection) detectTransitions(evt OfficeEvent, newState OfficeState, tick int, now time.Time) []StateTransition {
	var newTrans *StateTransition

	switch e := evt.(type) {
	case DevAssignedToTicket:
		if before, ok := p.state.GetAnimationOption(e.DevID).Get(); ok {
			if after, ok := newState.GetAnimationOption(e.DevID).Get(); ok {
				if before.State != after.State {
					newTrans = &StateTransition{
						DevID:     e.DevID,
						FromState: before.State.String(),
						ToState:   after.State.String(),
						Tick:      tick,
						Timestamp: now,
						Reason:    "assigned to " + e.TicketID,
					}
				}
			}
		}
	case DevStartedWorking:
		if before, ok := p.state.GetAnimationOption(e.DevID).Get(); ok {
			newTrans = &StateTransition{
				DevID:     e.DevID,
				FromState: before.State.String(),
				ToState:   StateWorking.String(),
				Tick:      tick,
				Timestamp: now,
			}
		}
	case DevBecameFrustrated:
		if before, ok := p.state.GetAnimationOption(e.DevID).Get(); ok {
			newTrans = &StateTransition{
				DevID:     e.DevID,
				FromState: before.State.String(),
				ToState:   StateFrustrated.String(),
				Tick:      tick,
				Timestamp: now,
				Reason:    "ticket " + e.TicketID + " exceeded estimate",
			}
		}
	case DevCompletedTicket:
		if before, ok := p.state.GetAnimationOption(e.DevID).Get(); ok {
			newTrans = &StateTransition{
				DevID:     e.DevID,
				FromState: before.State.String(),
				ToState:   StateIdle.String(),
				Tick:      tick,
				Timestamp: now,
				Reason:    "completed " + e.TicketID,
			}
		}
	case DevEnteredConference:
		if before, ok := p.state.GetAnimationOption(e.DevID).Get(); ok {
			newTrans = &StateTransition{
				DevID:     e.DevID,
				FromState: before.State.String(),
				ToState:   StateConference.String(),
				Tick:      tick,
				Timestamp: now,
			}
		}
	}

	if newTrans == nil {
		return p.transitions
	}

	result := make([]StateTransition, len(p.transitions)+1)
	copy(result, p.transitions)
	result[len(p.transitions)] = *newTrans
	return result
}

// Calculation: State returns the current OfficeState (memoized).
func (p OfficeProjection) State() OfficeState {
	return p.state
}

// Calculation: Events returns the event history for debugging.
func (p OfficeProjection) Events() []OfficeEvent {
	return p.events
}

// Calculation: Transitions returns the state change history.
func (p OfficeProjection) Transitions() []StateTransition {
	return p.transitions
}

// Calculation: CurrentTick returns the last recorded tick.
func (p OfficeProjection) CurrentTick() int {
	return p.currentTick
}

// Calculation: applyOfficeEvent applies one event to state.
// Pure function: (OfficeState, OfficeEvent, time.Time) → OfficeState
func applyOfficeEvent(state OfficeState, evt OfficeEvent, now time.Time) OfficeState {
	switch e := evt.(type) {
	case DevAssignedToTicket:
		return state.StartDeveloperMovingToCubicle(e.DevID, e.Target, now)
	case DevStartedWorking:
		return state.SetDeveloperState(e.DevID, StateWorking)
	case DevBecameFrustrated:
		return state.SetDeveloperState(e.DevID, StateFrustrated)
	case DevCompletedTicket:
		return state.SetDeveloperState(e.DevID, StateIdle)
	case DevEnteredConference:
		return state.SetDeveloperState(e.DevID, StateConference)
	case AnimationFrameAdvanced:
		return state.AdvanceFrames(now, e.DevIdxToAdvance)
	case DevStartedSip:
		return state.startDevSip(e.DevID, now)
	default:
		return state
	}
}

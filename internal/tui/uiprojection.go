package tui

// Data: UIProjection accumulates input events (ephemeral, session-scoped).
// This is a read model that computes UIState via pure fold over events.
type UIProjection struct {
	events []InputEvent // nil-safe: range handles nil slice (0 iterations)
}

// Calculation: () → UIProjection (empty projection).
// Returns a new projection with no events recorded.
func NewUIProjection() UIProjection {
	return UIProjection{}
}

// Calculation: UIProjection × InputEvent → UIProjection (value semantics, immutable).
// Returns a NEW projection with the event appended. Does not mutate the receiver.
// Note: O(n) slice copy. For typical sessions (<100 events), this is trivial.
func (p UIProjection) Record(evt InputEvent) UIProjection {
	return UIProjection{events: append(p.events, evt)}
}

// Calculation: UIProjection → UIState (pure fold over events).
// Computes current UI state by replaying all events from the beginning.
// Boundary: nil events slice handled safely by range (iterates 0 times).
// Idempotent: same events always produce same state.
func (p UIProjection) State() UIState {
	state := NewUIState()
	for _, evt := range p.events {
		switch e := evt.(type) {
		case ViewSwitched:
			state.CurrentView = e.To
			state.SelectedTicket = "" // Clear selection on view change
			state.ErrorMessage = ""   // Clear error on navigation
		case LessonPanelToggled:
			state.LessonVisible = !state.LessonVisible
		case TicketSelected:
			state.SelectedTicket = e.ID
		case SprintStartAttempted:
			state.ErrorMessage = errorFromOutcome(e.Outcome)
		case TickAttempted:
			state.ErrorMessage = errorFromOutcome(e.Outcome)
		case AssignmentAttempted:
			state.ErrorMessage = errorFromOutcome(e.Outcome)
			if _, ok := e.Outcome.(Succeeded); ok {
				state.SelectedTicket = "" // Clear selection on success
			}
		}
	}
	return state
}

// Calculation: Outcome → string (extracts error message from Failed, empty for Succeeded).
func errorFromOutcome(o Outcome) string {
	if f, ok := o.(Failed); ok {
		return f.Reason
	}
	return ""
}

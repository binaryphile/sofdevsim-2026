package tui

// Data: UIState represents UI state derived from input event stream.
// This is a read model - computed by folding over input events.
type UIState struct {
	CurrentView    View   // Reuses existing tui.View type
	SelectedTicket string // ID of currently selected ticket, empty if none
	LessonVisible  bool   // Whether lesson panel is shown
	ErrorMessage   string // Error from last failed action, empty if none
}

// Calculation: () → UIState (default state).
// Returns the initial UI state before any events are processed.
func NewUIState() UIState {
	return UIState{CurrentView: ViewPlanning}
}

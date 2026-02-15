package tui

// Data: Outcome sum type for input events (sealed interface pattern).
// Represents the result of a user action - either success or failure with details.
type Outcome interface{ sealed() }

// Data: successful outcome - action completed without error.
type Succeeded struct{}

func (Succeeded) sealed() {}

// Data: failed outcome with category and reason.
// Category enables programmatic handling; Reason is human-readable.
type Failed struct {
	Category FailureCategory
	Reason   string
}

func (Failed) sealed() {}

// Data: failure category enum for classifying errors.
type FailureCategory int

const (
	// BusinessRule indicates a domain rule violation (e.g., sprint already active).
	BusinessRule FailureCategory = iota
	// NotFound indicates the requested resource doesn't exist.
	NotFound
	// Conflict indicates a state conflict (e.g., developer already busy).
	Conflict
)

// Data: InputEvent sealed interface (all input events implement this).
// Input events are past-tense facts recorded AFTER outcome is known.
type InputEvent interface{ inputEvent() }

// Data: SprintStartAttempted records a sprint start attempt with its outcome.
type SprintStartAttempted struct{ Outcome Outcome }

// Data: TickAttempted records a simulation tick attempt with its outcome.
type TickAttempted struct{ Outcome Outcome }

// Data: ViewSwitched records a view change (always succeeds).
type ViewSwitched struct{ To View }

// Data: LessonPanelToggled records lesson panel visibility toggle (always succeeds).
type LessonPanelToggled struct{}

// Data: TicketSelected records a ticket selection (always succeeds).
type TicketSelected struct{ ID string }

// Data: AssignmentAttempted records a ticket assignment attempt with its outcome.
type AssignmentAttempted struct {
	TicketID string
	Outcome  Outcome
}

// Data: EventDeduplicated records when a self-event was received via subscription
// and skipped (projection already applied it locally).
type EventDeduplicated struct {
	EventType string // e.g. "SprintStarted", "Ticked"
}

// Marker methods for sealed interface pattern.
func (SprintStartAttempted) inputEvent() {}
func (TickAttempted) inputEvent()        {}
func (ViewSwitched) inputEvent()         {}
func (LessonPanelToggled) inputEvent()   {}
func (TicketSelected) inputEvent()       {}
func (AssignmentAttempted) inputEvent()  {}
func (EventDeduplicated) inputEvent()    {}

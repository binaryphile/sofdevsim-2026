package events

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// eventCounter provides unique sequential IDs for events.
var eventCounter uint64

// spanCounter provides unique sequential IDs for spans.
var spanCounter uint64

// EventIDGenerator is the function used to generate event IDs.
// Override this in tests to use deterministic IDs.
// Use SetEventIDGenerator for safe test setup with automatic cleanup.
var EventIDGenerator = defaultEventIDGenerator

// defaultEventIDGenerator generates unique sequential event IDs.
func defaultEventIDGenerator(eventType string) string {
	seq := atomic.AddUint64(&eventCounter, 1)
	return fmt.Sprintf("%s-%d", eventType, seq)
}

// SetEventIDGenerator sets a custom generator and returns a restore function.
// Use with t.Cleanup for safe test isolation:
//
//	t.Cleanup(events.SetEventIDGenerator(func(eventType string) string {
//	    return "deterministic-" + eventType
//	}))
func SetEventIDGenerator(gen func(string) string) func() {
	original := EventIDGenerator
	EventIDGenerator = gen
	return func() { EventIDGenerator = original }
}

// TraceContext holds tracing information for a request flowing through the system.
// Use this to correlate events and measure spans.
type TraceContext struct {
	TraceID      string // correlates all events from same request
	SpanID       string // this operation's span
	ParentSpanID string // parent span (empty if root)
}

// NewTraceContext creates a new trace context with fresh IDs.
func NewTraceContext() TraceContext {
	return TraceContext{
		TraceID: NextTraceID(),
		SpanID:  NextSpanID(),
	}
}

// NewChildSpan creates a child span within the same trace.
func (tc TraceContext) NewChildSpan() TraceContext {
	return TraceContext{
		TraceID:      tc.TraceID,
		SpanID:       NextSpanID(),
		ParentSpanID: tc.SpanID,
	}
}

// IsEmpty returns true if the trace context has no trace ID.
func (tc TraceContext) IsEmpty() bool {
	return tc.TraceID == ""
}

// ApplyTrace applies a trace context to an event, returning a new event with trace fields set.
// This is the exported helper for using the polymorphic withTrace interface method.
func ApplyTrace(evt Event, tc TraceContext) Event {
	return evt.withTrace(tc.TraceID, tc.SpanID, tc.ParentSpanID)
}

// nextEventID generates a unique event ID using the configured generator.
func nextEventID(eventType string) string {
	return EventIDGenerator(eventType)
}

// NextSpanID generates a unique span ID for tracing.
func NextSpanID() string {
	seq := atomic.AddUint64(&spanCounter, 1)
	return fmt.Sprintf("span-%d", seq)
}

// NextTraceID generates a unique trace ID for request correlation.
func NextTraceID() string {
	seq := atomic.AddUint64(&spanCounter, 1)
	return fmt.Sprintf("trace-%d", seq)
}

// Event represents a domain event that occurred in a simulation.
// All events are immutable value types.
//
// Per CQRS/Event Sourcing patterns (see cqrs-event-sourcing-guide.md):
// - EventID: unique identifier (ES guide §11: part of StoredEvent)
// - OccurrenceTime: simulation tick when event occurred (domain time)
// - DetectionTime: wall clock when recorded (audit trail)
// - CausedBy: causation ID linking to parent event (ES guide §11: Metadata)
//
// We keep both OccurrenceTime and DetectionTime because simulations have
// two time dimensions: simulation ticks (domain) and wall clock (audit).
//
// Tracing fields (OpenTelemetry-style):
// - TraceID: correlates all events from a single request
// - SpanID: identifies this specific operation
// - ParentSpanID: links to parent span for timing hierarchy
type Event interface {
	SimulationID() string
	EventID() string
	EventType() string
	OccurrenceTime() int      // simulation tick when event occurred
	DetectionTime() time.Time // wall clock when event was detected
	CausedBy() string         // EventID of causing event (empty if root)

	// Tracing - measure spans through the system
	TraceID() string      // correlates events from same request
	SpanID() string       // this operation's span
	ParentSpanID() string // parent span (empty if root)

	// withTrace returns a copy with tracing fields set.
	// This method enables type-safe tracing without type switches.
	withTrace(traceID, spanID, parentSpanID string) Event

	// EventVersion returns the schema version for upcasting (per CQRS Guide §11).
	// Used by Upcaster to dispatch transforms via "Type:vN" key format.
	EventVersion() int
}

// Header contains common fields for all events.
// Embed this in concrete event types.
type Header struct {
	ID         string    // unique event ID
	SimID      string    // simulation this event belongs to
	Type       string    // event type name
	OccurredAt int       // simulation tick
	DetectedAt time.Time // wall clock time
	CausedByID string    // ID of causing event

	// Tracing
	Trace      string // trace ID for request correlation
	Span       string // this span's ID
	ParentSpan string // parent span's ID

	// Schema version for upcasting (per CQRS Guide §11)
	Version int
}

func (h Header) EventID() string       { return h.ID }
func (h Header) SimulationID() string  { return h.SimID }
func (h Header) EventType() string     { return h.Type }
func (h Header) OccurrenceTime() int   { return h.OccurredAt }
func (h Header) DetectionTime() time.Time { return h.DetectedAt }
func (h Header) CausedBy() string      { return h.CausedByID }
func (h Header) TraceID() string       { return h.Trace }
func (h Header) SpanID() string        { return h.Span }
func (h Header) ParentSpanID() string  { return h.ParentSpan }
func (h Header) EventVersion() int     { return h.Version }

// SimConfig holds simulation configuration captured in SimulationCreated.
type SimConfig struct {
	TeamSize     int
	SprintLength int
	Seed         int64
	Policy       model.SizingPolicy // Sizing policy for ticket decomposition
	BufferPct    float64            // Buffer percentage (default 0.2 = 20%)
}

// SimulationCreated is emitted when a simulation is created.
type SimulationCreated struct {
	Header
	Config SimConfig
}

// NewSimulationCreated creates a SimulationCreated event with proper header.
func NewSimulationCreated(simID string, tick int, config SimConfig) SimulationCreated {
	return SimulationCreated{
		Header: Header{
			ID:         nextEventID("SimulationCreated"),
			SimID:      simID,
			Type:       "SimulationCreated",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		Config: config,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e SimulationCreated) WithTrace(traceID, spanID, parentSpanID string) SimulationCreated {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e SimulationCreated) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e SimulationCreated) WithCausedBy(eventID string) SimulationCreated {
	e.Header.CausedByID = eventID
	return e
}

// SprintStarted is emitted when a sprint begins.
type SprintStarted struct {
	Header
	Number     int
	StartTick  int
	BufferDays float64 // TameFlow buffer allocation for fever chart
}

// NewSprintStarted creates a SprintStarted event with proper header.
func NewSprintStarted(simID string, tick int, number int, bufferDays float64) SprintStarted {
	return SprintStarted{
		Header: Header{
			ID:         nextEventID("SprintStarted"),
			SimID:      simID,
			Type:       "SprintStarted",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		Number:     number,
		StartTick:  tick,
		BufferDays: bufferDays,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e SprintStarted) WithTrace(traceID, spanID, parentSpanID string) SprintStarted {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e SprintStarted) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e SprintStarted) WithCausedBy(eventID string) SprintStarted {
	e.Header.CausedByID = eventID
	return e
}

// SprintEnded is emitted when a sprint ends.
type SprintEnded struct {
	Header
	Number  int
	EndTick int
}

// NewSprintEnded creates a SprintEnded event with proper header.
func NewSprintEnded(simID string, tick int, number int) SprintEnded {
	return SprintEnded{
		Header: Header{
			ID:         nextEventID("SprintEnded"),
			SimID:      simID,
			Type:       "SprintEnded",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		Number:  number,
		EndTick: tick,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e SprintEnded) WithTrace(traceID, spanID, parentSpanID string) SprintEnded {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e SprintEnded) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e SprintEnded) WithCausedBy(eventID string) SprintEnded {
	e.Header.CausedByID = eventID
	return e
}

// Ticked is emitted when the simulation advances one tick.
type Ticked struct {
	Header
	Tick int
}

// NewTicked creates a Ticked event with proper header.
func NewTicked(simID string, tick int) Ticked {
	return Ticked{
		Header: Header{
			ID:         nextEventID("Ticked"),
			SimID:      simID,
			Type:       "Ticked",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		Tick: tick,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e Ticked) WithTrace(traceID, spanID, parentSpanID string) Ticked {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e Ticked) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e Ticked) WithCausedBy(eventID string) Ticked {
	e.Header.CausedByID = eventID
	return e
}

// TicketAssigned is emitted when a ticket is assigned to a developer.
type TicketAssigned struct {
	Header
	TicketID    string
	DeveloperID string
	Phase       model.WorkflowPhase // Phase being assigned (for handoff model)
	StartedAt   time.Time           // Wall-clock time when ticket was started
}

// NewTicketAssigned creates a TicketAssigned event with proper header.
// The tick parameter is used as StartedTick (available via OccurrenceTime()).
func NewTicketAssigned(simID string, tick int, ticketID, developerID string, phase model.WorkflowPhase, startedAt time.Time) TicketAssigned {
	return TicketAssigned{
		Header: Header{
			ID:         nextEventID("TicketAssigned"),
			SimID:      simID,
			Type:       "TicketAssigned",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID:    ticketID,
		DeveloperID: developerID,
		Phase:       phase,
		StartedAt:   startedAt,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketAssigned) WithTrace(traceID, spanID, parentSpanID string) TicketAssigned {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketAssigned) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketAssigned) WithCausedBy(eventID string) TicketAssigned {
	e.Header.CausedByID = eventID
	return e
}

// TicketStateRestored is emitted during EmitLoadedState to restore full ticket state.
// Unlike TicketAssigned (which always starts at PhaseResearch), this preserves the
// actual Phase and RemainingEffort from persistence.
type TicketStateRestored struct {
	Header
	TicketID        string
	DeveloperID     string
	Phase           model.WorkflowPhase
	RemainingEffort float64
	ActualDays      float64
	StartedAt       time.Time
}

// NewTicketStateRestored creates a TicketStateRestored event for persistence loading.
func NewTicketStateRestored(simID string, tick int, ticketID, developerID string, phase model.WorkflowPhase, remainingEffort, actualDays float64, startedAt time.Time) TicketStateRestored {
	return TicketStateRestored{
		Header: Header{
			ID:         nextEventID("TicketStateRestored"),
			SimID:      simID,
			Type:       "TicketStateRestored",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID:        ticketID,
		DeveloperID:     developerID,
		Phase:           phase,
		RemainingEffort: remainingEffort,
		ActualDays:      actualDays,
		StartedAt:       startedAt,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketStateRestored) WithTrace(traceID, spanID, parentSpanID string) TicketStateRestored {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketStateRestored) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketStateRestored) WithCausedBy(eventID string) TicketStateRestored {
	e.Header.CausedByID = eventID
	return e
}

// TicketCompleted is emitted when a developer completes a ticket.
type TicketCompleted struct {
	Header
	TicketID    string
	DeveloperID string
	ActualDays  float64 // Actual effort spent on ticket (for developer stats)
}

// NewTicketCompleted creates a TicketCompleted event with proper header.
func NewTicketCompleted(simID string, tick int, ticketID, developerID string, actualDays float64) TicketCompleted {
	return TicketCompleted{
		Header: Header{
			ID:         nextEventID("TicketCompleted"),
			SimID:      simID,
			Type:       "TicketCompleted",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID:    ticketID,
		DeveloperID: developerID,
		ActualDays:  actualDays,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketCompleted) WithTrace(traceID, spanID, parentSpanID string) TicketCompleted {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketCompleted) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketCompleted) WithCausedBy(eventID string) TicketCompleted {
	e.Header.CausedByID = eventID
	return e
}

// IncidentStarted is emitted when an incident occurs.
// Incidents are caused by completed tickets that introduce production issues.
type IncidentStarted struct {
	Header
	IncidentID  string
	DeveloperID string         // Developer who completed the causing ticket
	TicketID    string         // Ticket that caused the incident
	Severity    model.Severity // Severity of the incident
}

// NewIncidentStarted creates an IncidentStarted event with proper header.
func NewIncidentStarted(simID string, tick int, incidentID, developerID, ticketID string, severity model.Severity) IncidentStarted {
	return IncidentStarted{
		Header: Header{
			ID:         nextEventID("IncidentStarted"),
			SimID:      simID,
			Type:       "IncidentStarted",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		IncidentID:  incidentID,
		DeveloperID: developerID,
		TicketID:    ticketID,
		Severity:    severity,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e IncidentStarted) WithTrace(traceID, spanID, parentSpanID string) IncidentStarted {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e IncidentStarted) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e IncidentStarted) WithCausedBy(eventID string) IncidentStarted {
	e.Header.CausedByID = eventID
	return e
}

// IncidentResolved is emitted when an incident is resolved.
type IncidentResolved struct {
	Header
	IncidentID  string
	DeveloperID string
}

// NewIncidentResolved creates an IncidentResolved event with proper header.
func NewIncidentResolved(simID string, tick int, incidentID, developerID string) IncidentResolved {
	return IncidentResolved{
		Header: Header{
			ID:         nextEventID("IncidentResolved"),
			SimID:      simID,
			Type:       "IncidentResolved",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		IncidentID:  incidentID,
		DeveloperID: developerID,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e IncidentResolved) WithTrace(traceID, spanID, parentSpanID string) IncidentResolved {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e IncidentResolved) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e IncidentResolved) WithCausedBy(eventID string) IncidentResolved {
	e.Header.CausedByID = eventID
	return e
}

// DeveloperAdded is emitted when a developer joins the simulation.
type DeveloperAdded struct {
	Header
	DeveloperID     string
	Name            string
	Velocity        float64
	PhaseExperience [8]model.ExperienceLevel
}

// NewDeveloperAdded creates a DeveloperAdded event with proper header.
// Defaults all phases to ExperienceMedium.
func NewDeveloperAdded(simID string, tick int, devID, name string, velocity float64) DeveloperAdded {
	var exp [8]model.ExperienceLevel
	for i := range exp {
		exp[i] = model.ExperienceMedium
	}
	return DeveloperAdded{
		Header: Header{
			ID:         nextEventID("DeveloperAdded"),
			SimID:      simID,
			Type:       "DeveloperAdded",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    2,
		},
		DeveloperID:     devID,
		Name:            name,
		Velocity:        velocity,
		PhaseExperience: exp,
	}
}

// NewDeveloperAddedWithExperience creates a DeveloperAdded event with explicit experience levels.
func NewDeveloperAddedWithExperience(simID string, tick int, devID, name string, velocity float64, exp [8]model.ExperienceLevel) DeveloperAdded {
	return DeveloperAdded{
		Header: Header{
			ID:         nextEventID("DeveloperAdded"),
			SimID:      simID,
			Type:       "DeveloperAdded",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    2,
		},
		DeveloperID:     devID,
		Name:            name,
		Velocity:        velocity,
		PhaseExperience: exp,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e DeveloperAdded) WithTrace(traceID, spanID, parentSpanID string) DeveloperAdded {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e DeveloperAdded) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e DeveloperAdded) WithCausedBy(eventID string) DeveloperAdded {
	e.Header.CausedByID = eventID
	return e
}

// TicketCreated is emitted when a ticket is added to the backlog.
type TicketCreated struct {
	Header
	TicketID      string
	Title         string
	EstimatedDays float64
	Understanding model.UnderstandingLevel
	Priority      model.Priority
	IntakeStatus  model.IntakeStatus
}

// NewTicketCreated creates a TicketCreated event with proper header.
func NewTicketCreated(simID string, tick int, ticketID, title string, estimatedDays float64, understanding model.UnderstandingLevel, priority model.Priority, intakeStatus model.IntakeStatus) TicketCreated {
	return TicketCreated{
		Header: Header{
			ID:         nextEventID("TicketCreated"),
			SimID:      simID,
			Type:       "TicketCreated",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID:      ticketID,
		Title:         title,
		EstimatedDays: estimatedDays,
		Understanding: understanding,
		Priority:      priority,
		IntakeStatus:  intakeStatus,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketCreated) WithTrace(traceID, spanID, parentSpanID string) TicketCreated {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketCreated) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketCreated) WithCausedBy(eventID string) TicketCreated {
	e.Header.CausedByID = eventID
	return e
}

// WorkProgressed is emitted when effort is applied to a ticket.
type WorkProgressed struct {
	Header
	TicketID      string
	Phase         model.WorkflowPhase // Phase in which work was done (for PhaseEffortSpent tracking)
	EffortApplied float64
}

// NewWorkProgressed creates a WorkProgressed event with proper header.
func NewWorkProgressed(simID string, tick int, ticketID string, phase model.WorkflowPhase, effort float64) WorkProgressed {
	return WorkProgressed{
		Header: Header{
			ID:         nextEventID("WorkProgressed"),
			SimID:      simID,
			Type:       "WorkProgressed",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID:      ticketID,
		Phase:         phase,
		EffortApplied: effort,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e WorkProgressed) WithTrace(traceID, spanID, parentSpanID string) WorkProgressed {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e WorkProgressed) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e WorkProgressed) WithCausedBy(eventID string) WorkProgressed {
	e.Header.CausedByID = eventID
	return e
}

// TicketPhaseChanged is emitted when a ticket advances to the next phase.
type TicketPhaseChanged struct {
	Header
	TicketID string
	OldPhase model.WorkflowPhase
	NewPhase model.WorkflowPhase
}

// NewTicketPhaseChanged creates a TicketPhaseChanged event with proper header.
func NewTicketPhaseChanged(simID string, tick int, ticketID string, oldPhase, newPhase model.WorkflowPhase) TicketPhaseChanged {
	return TicketPhaseChanged{
		Header: Header{
			ID:         nextEventID("TicketPhaseChanged"),
			SimID:      simID,
			Type:       "TicketPhaseChanged",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID: ticketID,
		OldPhase: oldPhase,
		NewPhase: newPhase,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketPhaseChanged) WithTrace(traceID, spanID, parentSpanID string) TicketPhaseChanged {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketPhaseChanged) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketPhaseChanged) WithCausedBy(eventID string) TicketPhaseChanged {
	e.Header.CausedByID = eventID
	return e
}

// BufferConsumed is emitted when sprint buffer is consumed due to schedule variance.
type BufferConsumed struct {
	Header
	DaysConsumed float64 // Amount of buffer consumed this tick
}

// NewBufferConsumed creates a BufferConsumed event with proper header.
func NewBufferConsumed(simID string, tick int, daysConsumed float64) BufferConsumed {
	return BufferConsumed{
		Header: Header{
			ID:         nextEventID("BufferConsumed"),
			SimID:      simID,
			Type:       "BufferConsumed",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		DaysConsumed: daysConsumed,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e BufferConsumed) WithTrace(traceID, spanID, parentSpanID string) BufferConsumed {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e BufferConsumed) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e BufferConsumed) WithCausedBy(eventID string) BufferConsumed {
	e.Header.CausedByID = eventID
	return e
}

// BufferZoneChanged is emitted when the fever chart zone transitions.
type BufferZoneChanged struct {
	Header
	OldZone     model.FeverStatus
	NewZone     model.FeverStatus
	Penetration float64 // Buffer penetration ratio (0.0-1.0)
}

// NewBufferZoneChanged creates a BufferZoneChanged event with proper header.
func NewBufferZoneChanged(simID string, tick int, oldZone, newZone model.FeverStatus, penetration float64) BufferZoneChanged {
	return BufferZoneChanged{
		Header: Header{
			ID:         nextEventID("BufferZoneChanged"),
			SimID:      simID,
			Type:       "BufferZoneChanged",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		OldZone:     oldZone,
		NewZone:     newZone,
		Penetration: penetration,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e BufferZoneChanged) WithTrace(traceID, spanID, parentSpanID string) BufferZoneChanged {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e BufferZoneChanged) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e BufferZoneChanged) WithCausedBy(eventID string) BufferZoneChanged {
	e.Header.CausedByID = eventID
	return e
}

// IsZoneChange returns true if this event represents a zone transition.
// This is a method expression usable as a predicate: Event.IsZoneChange
func (e BufferZoneChanged) IsZoneChange() bool {
	return true
}

// PolicyChanged is emitted when the simulation's sizing policy is changed.
type PolicyChanged struct {
	Header
	OldPolicy model.SizingPolicy
	NewPolicy model.SizingPolicy
}

// NewPolicyChanged creates a PolicyChanged event with proper header.
func NewPolicyChanged(simID string, tick int, oldPolicy, newPolicy model.SizingPolicy) PolicyChanged {
	return PolicyChanged{
		Header: Header{
			ID:         nextEventID("PolicyChanged"),
			SimID:      simID,
			Type:       "PolicyChanged",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		OldPolicy: oldPolicy,
		NewPolicy: newPolicy,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e PolicyChanged) WithTrace(traceID, spanID, parentSpanID string) PolicyChanged {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e PolicyChanged) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e PolicyChanged) WithCausedBy(eventID string) PolicyChanged {
	e.Header.CausedByID = eventID
	return e
}

// ChildTicket contains essential fields for a decomposed child ticket.
type ChildTicket struct {
	ID            string
	Title         string
	EstimatedDays float64
	Understanding model.UnderstandingLevel
}

// TicketDecomposed is emitted when a parent ticket is decomposed into children.
type TicketDecomposed struct {
	Header
	ParentTicketID string
	Children       []ChildTicket
}

// NewTicketDecomposed creates a TicketDecomposed event with proper header.
func NewTicketDecomposed(simID string, tick int, parentID string, children []ChildTicket) TicketDecomposed {
	return TicketDecomposed{
		Header: Header{
			ID:         nextEventID("TicketDecomposed"),
			SimID:      simID,
			Type:       "TicketDecomposed",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		ParentTicketID: parentID,
		Children:       children,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketDecomposed) WithTrace(traceID, spanID, parentSpanID string) TicketDecomposed {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketDecomposed) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketDecomposed) WithCausedBy(eventID string) TicketDecomposed {
	e.Header.CausedByID = eventID
	return e
}

// SprintWIPUpdated is emitted to track WIP metrics during a sprint.
type SprintWIPUpdated struct {
	Header
	CurrentWIP int // Number of active tickets at this moment
}

// NewSprintWIPUpdated creates a SprintWIPUpdated event with proper header.
func NewSprintWIPUpdated(simID string, tick int, currentWIP int) SprintWIPUpdated {
	return SprintWIPUpdated{
		Header: Header{
			ID:         nextEventID("SprintWIPUpdated"),
			SimID:      simID,
			Type:       "SprintWIPUpdated",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		CurrentWIP: currentWIP,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e SprintWIPUpdated) WithTrace(traceID, spanID, parentSpanID string) SprintWIPUpdated {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e SprintWIPUpdated) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e SprintWIPUpdated) WithCausedBy(eventID string) SprintWIPUpdated {
	e.Header.CausedByID = eventID
	return e
}

// BugDiscovered is emitted when a bug is discovered in an active ticket.
// This adds rework effort to the ticket's remaining work.
type BugDiscovered struct {
	Header
	TicketID     string
	ReworkEffort float64 // Additional effort needed (typically 0.5 days)
}

// NewBugDiscovered creates a BugDiscovered event with proper header.
func NewBugDiscovered(simID string, tick int, ticketID string, reworkEffort float64) BugDiscovered {
	return BugDiscovered{
		Header: Header{
			ID:         nextEventID("BugDiscovered"),
			SimID:      simID,
			Type:       "BugDiscovered",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID:     ticketID,
		ReworkEffort: reworkEffort,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e BugDiscovered) WithTrace(traceID, spanID, parentSpanID string) BugDiscovered {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e BugDiscovered) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e BugDiscovered) WithCausedBy(eventID string) BugDiscovered {
	e.Header.CausedByID = eventID
	return e
}

// ScopeCreepOccurred is emitted when scope creep increases a ticket's work.
// This adds to both remaining effort and the original estimate.
type ScopeCreepOccurred struct {
	Header
	TicketID      string
	EffortAdded   float64 // Additional effort needed
	EstimateAdded float64 // Additional estimate (may differ from effort)
}

// NewScopeCreepOccurred creates a ScopeCreepOccurred event with proper header.
func NewScopeCreepOccurred(simID string, tick int, ticketID string, effortAdded, estimateAdded float64) ScopeCreepOccurred {
	return ScopeCreepOccurred{
		Header: Header{
			ID:         nextEventID("ScopeCreepOccurred"),
			SimID:      simID,
			Type:       "ScopeCreepOccurred",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID:      ticketID,
		EffortAdded:   effortAdded,
		EstimateAdded: estimateAdded,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e ScopeCreepOccurred) WithTrace(traceID, spanID, parentSpanID string) ScopeCreepOccurred {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e ScopeCreepOccurred) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e ScopeCreepOccurred) WithCausedBy(eventID string) ScopeCreepOccurred {
	e.Header.CausedByID = eventID
	return e
}

// TicketQueued is emitted when a ticket enters a phase queue after handoff.
type TicketQueued struct {
	Header
	TicketID      string
	Phase         model.WorkflowPhase
	PreviousDevID string // dev who completed the previous phase
}

// NewTicketQueued creates a TicketQueued event with proper header.
func NewTicketQueued(simID string, tick int, ticketID string, phase model.WorkflowPhase, previousDevID string) TicketQueued {
	return TicketQueued{
		Header: Header{
			ID:         nextEventID("TicketQueued"),
			SimID:      simID,
			Type:       "TicketQueued",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID:      ticketID,
		Phase:         phase,
		PreviousDevID: previousDevID,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketQueued) WithTrace(traceID, spanID, parentSpanID string) TicketQueued {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketQueued) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketQueued) WithCausedBy(eventID string) TicketQueued {
	e.Header.CausedByID = eventID
	return e
}

// DeveloperReleased is emitted when a developer finishes a phase and is released from a ticket.
type DeveloperReleased struct {
	Header
	DeveloperID string
	TicketID    string
}

// NewDeveloperReleased creates a DeveloperReleased event with proper header.
func NewDeveloperReleased(simID string, tick int, developerID, ticketID string) DeveloperReleased {
	return DeveloperReleased{
		Header: Header{
			ID:         nextEventID("DeveloperReleased"),
			SimID:      simID,
			Type:       "DeveloperReleased",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		DeveloperID: developerID,
		TicketID:    ticketID,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e DeveloperReleased) WithTrace(traceID, spanID, parentSpanID string) DeveloperReleased {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e DeveloperReleased) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e DeveloperReleased) WithCausedBy(eventID string) DeveloperReleased {
	e.Header.CausedByID = eventID
	return e
}

// CICDSlotConsumed is emitted when a ticket starts using a CI/CD pipeline slot.
type CICDSlotConsumed struct {
	Header
	TicketID string
}

// NewCICDSlotConsumed creates a CICDSlotConsumed event with proper header.
func NewCICDSlotConsumed(simID string, tick int, ticketID string) CICDSlotConsumed {
	return CICDSlotConsumed{
		Header: Header{
			ID:         nextEventID("CICDSlotConsumed"),
			SimID:      simID,
			Type:       "CICDSlotConsumed",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID: ticketID,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e CICDSlotConsumed) WithTrace(traceID, spanID, parentSpanID string) CICDSlotConsumed {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e CICDSlotConsumed) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e CICDSlotConsumed) WithCausedBy(eventID string) CICDSlotConsumed {
	e.Header.CausedByID = eventID
	return e
}

// CICDSlotReleased is emitted when a ticket finishes CI/CD and releases its pipeline slot.
type CICDSlotReleased struct {
	Header
	TicketID string
}

// NewCICDSlotReleased creates a CICDSlotReleased event with proper header.
func NewCICDSlotReleased(simID string, tick int, ticketID string) CICDSlotReleased {
	return CICDSlotReleased{
		Header: Header{
			ID:         nextEventID("CICDSlotReleased"),
			SimID:      simID,
			Type:       "CICDSlotReleased",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID: ticketID,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e CICDSlotReleased) WithTrace(traceID, spanID, parentSpanID string) CICDSlotReleased {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e CICDSlotReleased) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e CICDSlotReleased) WithCausedBy(eventID string) CICDSlotReleased {
	e.Header.CausedByID = eventID
	return e
}

// TicketQueueRepaired is emitted when an orphaned ticket ID is removed from a phase queue.
// Indicates projection corruption: queue contained a ticket not in ActiveTickets.
type TicketQueueRepaired struct {
	Header
	TicketID string
	Phase    model.WorkflowPhase
}

// NewTicketQueueRepaired creates a TicketQueueRepaired event.
func NewTicketQueueRepaired(simID string, tick int, ticketID string, phase model.WorkflowPhase) TicketQueueRepaired {
	return TicketQueueRepaired{
		Header: Header{
			ID:         nextEventID("TicketQueueRepaired"),
			SimID:      simID,
			Type:       "TicketQueueRepaired",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID: ticketID,
		Phase:    phase,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketQueueRepaired) WithTrace(traceID, spanID, parentSpanID string) TicketQueueRepaired {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketQueueRepaired) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketQueueRepaired) WithCausedBy(eventID string) TicketQueueRepaired {
	e.Header.CausedByID = eventID
	return e
}

// MentorPaired is emitted when a High-experience dev is paired as mentor for a Low-experience dev.
type MentorPaired struct {
	Header
	MentorID string
	MenteeID string
	TicketID string
	Phase    model.WorkflowPhase
}

// NewMentorPaired creates a MentorPaired event with proper header.
func NewMentorPaired(simID string, tick int, mentorID, menteeID, ticketID string, phase model.WorkflowPhase) MentorPaired {
	return MentorPaired{
		Header: Header{
			ID:         nextEventID("MentorPaired"),
			SimID:      simID,
			Type:       "MentorPaired",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		MentorID: mentorID,
		MenteeID: menteeID,
		TicketID: ticketID,
		Phase:    phase,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e MentorPaired) WithTrace(traceID, spanID, parentSpanID string) MentorPaired {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e MentorPaired) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e MentorPaired) WithCausedBy(eventID string) MentorPaired {
	e.Header.CausedByID = eventID
	return e
}

// MentorReleased is emitted when a mentor is freed after the mentored phase completes.
type MentorReleased struct {
	Header
	MentorID string
	MenteeID string
	TicketID string
	Phase    model.WorkflowPhase
}

// NewMentorReleased creates a MentorReleased event with proper header.
func NewMentorReleased(simID string, tick int, mentorID, menteeID, ticketID string, phase model.WorkflowPhase) MentorReleased {
	return MentorReleased{
		Header: Header{
			ID:         nextEventID("MentorReleased"),
			SimID:      simID,
			Type:       "MentorReleased",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		MentorID: mentorID,
		MenteeID: menteeID,
		TicketID: ticketID,
		Phase:    phase,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e MentorReleased) WithTrace(traceID, spanID, parentSpanID string) MentorReleased {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e MentorReleased) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e MentorReleased) WithCausedBy(eventID string) MentorReleased {
	e.Header.CausedByID = eventID
	return e
}

// TicketTriaged is emitted when a ticket passes triage during sprint planning.
type TicketTriaged struct {
	Header
	TicketID string
}

// NewTicketTriaged creates a TicketTriaged event with proper header.
func NewTicketTriaged(simID string, tick int, ticketID string) TicketTriaged {
	return TicketTriaged{
		Header: Header{
			ID:         nextEventID("TicketTriaged"),
			SimID:      simID,
			Type:       "TicketTriaged",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID: ticketID,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketTriaged) WithTrace(traceID, spanID, parentSpanID string) TicketTriaged {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketTriaged) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketTriaged) WithCausedBy(eventID string) TicketTriaged {
	e.Header.CausedByID = eventID
	return e
}

// TicketCommitted is emitted when a ticket is committed to a sprint.
type TicketCommitted struct {
	Header
	TicketID     string
	SprintNumber int
}

// NewTicketCommitted creates a TicketCommitted event with proper header.
func NewTicketCommitted(simID string, tick int, ticketID string, sprintNumber int) TicketCommitted {
	return TicketCommitted{
		Header: Header{
			ID:         nextEventID("TicketCommitted"),
			SimID:      simID,
			Type:       "TicketCommitted",
			OccurredAt: tick,
			DetectedAt: time.Now(),
			Version:    1,
		},
		TicketID:     ticketID,
		SprintNumber: sprintNumber,
	}
}

// WithTrace returns a copy with tracing fields set for fluent chaining.
func (e TicketCommitted) WithTrace(traceID, spanID, parentSpanID string) TicketCommitted {
	e.Header.Trace = traceID
	e.Header.Span = spanID
	e.Header.ParentSpan = parentSpanID
	return e
}

// withTrace implements Event interface for polymorphic tracing.
func (e TicketCommitted) withTrace(traceID, spanID, parentSpanID string) Event {
	return e.WithTrace(traceID, spanID, parentSpanID)
}

// WithCausedBy returns a copy with causation link to parent event.
func (e TicketCommitted) WithCausedBy(eventID string) TicketCommitted {
	e.Header.CausedByID = eventID
	return e
}

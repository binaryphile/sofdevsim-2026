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

// nextEventID generates a unique event ID.
func nextEventID(eventType string) string {
	seq := atomic.AddUint64(&eventCounter, 1)
	return fmt.Sprintf("%s-%d", eventType, seq)
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
// Per Etzion & Niblett "Event Processing in Action":
// - EventID: unique identifier for tracing
// - OccurrenceTime: when the event actually happened (simulation tick)
// - DetectionTime: when the system detected it (wall clock)
// - CausedBy: relationship to parent event (causation chain)
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

// SimConfig holds simulation configuration captured in SimulationCreated.
type SimConfig struct {
	TeamSize     int
	SprintLength int
	Seed         int64
	Policy       model.SizingPolicy // Sizing policy for ticket decomposition
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
	Number    int
	StartTick int
}

// NewSprintStarted creates a SprintStarted event with proper header.
func NewSprintStarted(simID string, tick int, number int) SprintStarted {
	return SprintStarted{
		Header: Header{
			ID:         nextEventID("SprintStarted"),
			SimID:      simID,
			Type:       "SprintStarted",
			OccurredAt: tick,
			DetectedAt: time.Now(),
		},
		Number:    number,
		StartTick: tick,
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
}

// NewTicketAssigned creates a TicketAssigned event with proper header.
func NewTicketAssigned(simID string, tick int, ticketID, developerID string) TicketAssigned {
	return TicketAssigned{
		Header: Header{
			ID:         nextEventID("TicketAssigned"),
			SimID:      simID,
			Type:       "TicketAssigned",
			OccurredAt: tick,
			DetectedAt: time.Now(),
		},
		TicketID:    ticketID,
		DeveloperID: developerID,
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

// TicketCompleted is emitted when a developer completes a ticket.
type TicketCompleted struct {
	Header
	TicketID    string
	DeveloperID string
}

// NewTicketCompleted creates a TicketCompleted event with proper header.
func NewTicketCompleted(simID string, tick int, ticketID, developerID string) TicketCompleted {
	return TicketCompleted{
		Header: Header{
			ID:         nextEventID("TicketCompleted"),
			SimID:      simID,
			Type:       "TicketCompleted",
			OccurredAt: tick,
			DetectedAt: time.Now(),
		},
		TicketID:    ticketID,
		DeveloperID: developerID,
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
type IncidentStarted struct {
	Header
	IncidentID  string
	DeveloperID string
}

// NewIncidentStarted creates an IncidentStarted event with proper header.
func NewIncidentStarted(simID string, tick int, incidentID, developerID string) IncidentStarted {
	return IncidentStarted{
		Header: Header{
			ID:         nextEventID("IncidentStarted"),
			SimID:      simID,
			Type:       "IncidentStarted",
			OccurredAt: tick,
			DetectedAt: time.Now(),
		},
		IncidentID:  incidentID,
		DeveloperID: developerID,
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
	DeveloperID string
	Name        string
	Velocity    float64
}

// NewDeveloperAdded creates a DeveloperAdded event with proper header.
func NewDeveloperAdded(simID string, tick int, devID, name string, velocity float64) DeveloperAdded {
	return DeveloperAdded{
		Header: Header{
			ID:         nextEventID("DeveloperAdded"),
			SimID:      simID,
			Type:       "DeveloperAdded",
			OccurredAt: tick,
			DetectedAt: time.Now(),
		},
		DeveloperID: devID,
		Name:        name,
		Velocity:    velocity,
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
}

// NewTicketCreated creates a TicketCreated event with proper header.
func NewTicketCreated(simID string, tick int, ticketID, title string, estimatedDays float64, understanding model.UnderstandingLevel) TicketCreated {
	return TicketCreated{
		Header: Header{
			ID:         nextEventID("TicketCreated"),
			SimID:      simID,
			Type:       "TicketCreated",
			OccurredAt: tick,
			DetectedAt: time.Now(),
		},
		TicketID:      ticketID,
		Title:         title,
		EstimatedDays: estimatedDays,
		Understanding: understanding,
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
	EffortApplied float64
}

// NewWorkProgressed creates a WorkProgressed event with proper header.
func NewWorkProgressed(simID string, tick int, ticketID string, effort float64) WorkProgressed {
	return WorkProgressed{
		Header: Header{
			ID:         nextEventID("WorkProgressed"),
			SimID:      simID,
			Type:       "WorkProgressed",
			OccurredAt: tick,
			DetectedAt: time.Now(),
		},
		TicketID:      ticketID,
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

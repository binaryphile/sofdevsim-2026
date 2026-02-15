package engine

import (
	"fmt"
	"time"

	"github.com/binaryphile/fluentfp/either"
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// NotDecomposable explains why decomposition didn't happen.
// Value type: no pointers, use value semantics.
type NotDecomposable struct {
	Reason string
}

// toChildTicket converts model.Ticket to events.ChildTicket.
func toChildTicket(t model.Ticket) events.ChildTicket {
	return events.ChildTicket{
		ID:            t.ID,
		Title:         t.Title,
		EstimatedDays: t.EstimatedDays,
		Understanding: t.UnderstandingLevel,
	}
}

// Engine orchestrates pure and impure operations for simulation execution.
//
// Architecture (per FP Guide ACD pattern):
//   - Calculations: variance model, policy decisions (pure, deterministic)
//   - Actions: event emission, store writes (side effects)
//   - Data: Projection state (immutable, derived from events)
//
// Value receiver: all methods return new Engine (immutable pattern per FP Guide §7).
type Engine struct {
	proj     events.Projection              // Data: immutable state derived from events
	variance VarianceModel                  // Calculation: pure, deterministic variance
	evtGen   EventGenerator                 // Calculation: seeded RNG, deterministic per tick
	policies PolicyEngine                   // Calculation: pure policy decisions
	storeOption option.Basic[events.Store]  // Action: I/O to event store (optional)
	trace    events.TraceContext            // Data: correlation context for events
}

// NewEngine creates a simulation engine without event sourcing.
// Use EmitCreated() or EmitLoadedState() to initialize projection state.
func NewEngine(seed int64) Engine {
	return Engine{
		proj:     events.NewProjection(),
		variance: NewVarianceModel(seed),
		evtGen:   NewEventGenerator(seed),
		policies: NewPolicyEngine(seed),
	}
}

// NewEngineWithStore creates a simulation engine with event sourcing.
// Use EmitCreated() or EmitLoadedState() to initialize projection state.
func NewEngineWithStore(seed int64, store events.Store) Engine {
	return Engine{
		proj:     events.NewProjection(),
		variance: NewVarianceModel(seed),
		evtGen:   NewEventGenerator(seed),
		policies: NewPolicyEngine(seed),
		storeOption: option.Of(store),
	}
}

// EmitCreated emits SimulationCreated event with the given config.
// Call after basic simulation setup is complete.
// Returns new Engine with updated state (immutable pattern).
// Returns error on store conflict (caller should retry).
func (e Engine) EmitCreated(id string, tick int, config events.SimConfig) (Engine, error) {
	return e.emit(events.NewSimulationCreated(id, tick, config))
}

// SetTrace sets the current trace context for event correlation.
// All events emitted will include this trace information.
// Returns new Engine with updated trace (immutable pattern).
func (e Engine) SetTrace(tc events.TraceContext) Engine {
	e.trace = tc
	return e
}

// ClearTrace clears the current trace context.
// Returns new Engine with cleared trace (immutable pattern).
func (e Engine) ClearTrace() Engine {
	e.trace = events.TraceContext{}
	return e
}

// CurrentTrace returns the current trace context.
func (e Engine) CurrentTrace() events.TraceContext {
	return e.trace
}

// Sim returns the current simulation state derived from the projection.
// This is the primary way to access state in event-sourced mode.
func (e Engine) Sim() model.Simulation {
	return e.proj.State()
}

// state returns current simulation state from projection.
// Internal helper for consistent state access within engine methods.
func (e Engine) state() model.Simulation {
	return e.proj.State()
}

// emit sends an event to the store if configured, attaching trace context if set.
// Also applies the event to the projection to keep derived state in sync.
// Returns new Engine with updated projection (immutable pattern).
// Returns error on store concurrency conflict (caller should retry with fresh state).
func (e Engine) emit(evt events.Event) (Engine, error) {
	// Apply trace context if set
	if !e.trace.IsEmpty() {
		evt = e.applyTrace(evt)
	}

	// Capture version BEFORE applying (for optimistic concurrency)
	expectedVersion := e.proj.Version()

	// Apply event to get new projection
	newProj := e.proj.Apply(evt)

	// Only append to store if configured
	if store, ok := e.storeOption.Get(); ok {
		if err := store.Append(evt.SimulationID(), expectedVersion, evt); err != nil {
			// Concurrency conflict - return error for retry
			return e, err
		}
	}

	return e.withProj(newProj), nil
}

// withProj returns a new Engine with the given projection, preserving all other fields.
// Pure construction helper for immutable pattern.
func (e Engine) withProj(proj events.Projection) Engine {
	return Engine{
		proj:     proj,
		variance: e.variance,
		evtGen:   e.evtGen,
		policies: e.policies,
		storeOption: e.storeOption,
		trace:    e.trace,
	}
}

// ApplyEvent applies an external event to the projection without writing to store.
// Idempotent: if the event was already processed (self-event), returns unchanged Engine.
func (e Engine) ApplyEvent(evt events.Event) Engine {
	return e.withProj(e.proj.Apply(evt))
}

// ProjectionVersion returns the projection's event count for self-event detection.
func (e Engine) ProjectionVersion() int {
	return e.proj.Version()
}

// applyTrace applies the current trace context to an event using the Event interface.
func (e Engine) applyTrace(evt events.Event) events.Event {
	return events.ApplyTrace(evt, e.trace)
}

// Tick advances the simulation by one day.
// Returns new Engine, UI events, and error (immutable pattern).
// Returns error on store conflict (caller should retry with fresh state).
func (e Engine) Tick() (Engine, []model.Event, error) {
	var err error
	allEvents := make([]model.Event, 0)

	// Increment tick: emit first, projection handler updates state
	newTick := e.state().CurrentTick + 1
	if e, err = e.emit(events.NewTicked(e.state().ID, newTick)); err != nil {
		return e, nil, err
	}

	// 1. Developers work on assigned tickets
	// Read from projection after Ticked emit
	state := e.state()
	for i := range state.Developers { // justified:EP
		dev := state.Developers[i]
		if dev.IsIdle() {
			continue
		}

		ticketIdx := state.FindActiveTicketIndex(dev.CurrentTicket)
		if ticketIdx == -1 {
			// Developer assigned to non-existent ticket - shouldn't happen with proper ES
			continue
		}

		ticket := state.ActiveTickets[ticketIdx]

		// Calculate work done with variance
		variance := e.variance.Calculate(ticket, e.state().CurrentTick)
		workDone := dev.Velocity * variance

		// Emit WorkProgressed event FIRST - projection handler updates:
		// - ticket.RemainingEffort -= workDone
		// - ticket.ActualDays += workDone
		// - ticket.PhaseEffortSpent[phase] += workDone
		if e, err = e.emit(events.NewWorkProgressed(e.state().ID, e.state().CurrentTick, ticket.ID, ticket.Phase, workDone)); err != nil {
			return e, nil, err
		}

		// Re-lookup ticket by ID after emit (index may shift if earlier tickets completed)
		ticketIdx = e.state().FindActiveTicketIndex(ticket.ID)
		if ticketIdx == -1 {
			continue // ticket completed and removed during this tick
		}
		ticket = e.state().ActiveTickets[ticketIdx]
		if ticket.RemainingEffort <= 0 {
			var uiEvents []model.Event
			if e, uiEvents, err = e.advancePhaseEmitOnly(ticket, dev); err != nil {
				return e, nil, err
			}
			allEvents = append(allEvents, uiEvents...)
		}
	}

	// 2. Generate random events (bugs, scope creep)
	// Generators return ES events; emit them and convert to UI events
	for _, evt := range e.evtGen.GenerateRandomEvents(e.state()) { // justified:EP
		if e, err = e.emit(evt); err != nil {
			return e, nil, err
		}
		allEvents = append(allEvents, e.toUIEvent(evt))
	}

	// 3. Check for incidents on recently deployed tickets
	for _, evt := range e.evtGen.CheckForIncidents(e.state()) { // justified:EP
		if e, err = e.emit(evt); err != nil {
			return e, nil, err
		}
		allEvents = append(allEvents, e.toUIEvent(evt))
	}

	// 4. Track WIP for export
	if e, err = e.trackWIP(); err != nil {
		return e, nil, err
	}

	// 6. Check sprint end (read from projection - updated by earlier emits)
	if sprint, ok := e.state().CurrentSprintOption.Get(); ok && e.state().CurrentTick >= sprint.EndDay {
		var endEvents []model.Event
		if e, endEvents, err = e.endSprint(); err != nil {
			return e, nil, err
		}
		allEvents = append(allEvents, endEvents...)
	}

	return e, allEvents, nil
}

// advancePhaseEmitOnly emits phase change events - projection handlers update state.
// Returns new Engine, UI events, and error (immutable pattern).
func (e Engine) advancePhaseEmitOnly(ticket model.Ticket, dev model.Developer) (Engine, []model.Event, error) {
	var err error
	modelEvents := make([]model.Event, 0)

	oldPhase := ticket.Phase
	newPhase := oldPhase + 1

	if newPhase == model.PhaseDone {
		// Emit TicketCompleted - projection handler:
		// - Moves ticket to CompletedTickets
		// - Updates developer stats (CurrentTicket="", TicketsCompleted++, etc)
		if e, err = e.emit(events.NewTicketCompleted(e.state().ID, e.state().CurrentTick, ticket.ID, dev.ID, ticket.ActualDays)); err != nil {
			return e, nil, err
		}

		// CCPM: Adjust buffer based on variance from estimate
		// Positive variance (over estimate) consumes buffer
		// Negative variance (under estimate) reclaims buffer
		variance := ticket.ActualDays - ticket.EstimatedDays
		if variance != 0 {
			if e, err = e.emit(events.NewBufferConsumed(e.state().ID, e.state().CurrentTick, variance)); err != nil {
				return e, nil, err
			}
		}

		modelEvents = append(modelEvents, model.NewEvent(
			model.EventTicketComplete,
			fmt.Sprintf("%s completed (%.1f days actual vs %.1f estimated)", ticket.ID, ticket.ActualDays, ticket.EstimatedDays),
			e.state().CurrentTick,
		))
	} else {
		// Emit TicketPhaseChanged - projection handler:
		// - Updates ticket.Phase
		// - Sets ticket.RemainingEffort for new phase
		if e, err = e.emit(events.NewTicketPhaseChanged(e.state().ID, e.state().CurrentTick, ticket.ID, oldPhase, newPhase)); err != nil {
			return e, nil, err
		}

		modelEvents = append(modelEvents, model.NewEvent(
			model.EventPhaseAdvance,
			fmt.Sprintf("%s: %s -> %s", ticket.ID, oldPhase, newPhase),
			e.state().CurrentTick,
		))
	}

	return e, modelEvents, nil
}


// EmitBufferConsumed directly consumes buffer days (for lesson debt carryover).
// Returns new Engine and error (immutable pattern).
func (e Engine) EmitBufferConsumed(days float64) (Engine, error) {
	return e.emit(events.NewBufferConsumed(e.state().ID, e.state().CurrentTick, days))
}

// trackWIP records work-in-progress metrics for export.
// Returns new Engine and error (immutable pattern).
func (e Engine) trackWIP() (Engine, error) {
	_, ok := e.state().CurrentSprintOption.Get()
	if !ok {
		return e, nil
	}

	currentWIP := len(e.state().ActiveTickets)

	// Emit event - projection handler will update WIP metrics
	return e.emit(events.NewSprintWIPUpdated(e.state().ID, e.state().CurrentTick, currentWIP))
}

// endSprint handles sprint completion.
// Returns new Engine, UI events, and error (immutable pattern).
func (e Engine) endSprint() (Engine, []model.Event, error) {
	var err error
	modelEvents := make([]model.Event, 0)

	sprint, ok := e.state().CurrentSprintOption.Get()
	if ok {
		// Emit SprintEnded event
		if e, err = e.emit(events.NewSprintEnded(e.state().ID, e.state().CurrentTick, sprint.Number)); err != nil {
			return e, nil, err
		}
	}

	// Any unfinished active tickets stay active for next sprint
	// (In a real sim, we might handle carryover differently)

	return e, modelEvents, nil
}

// StartSprint begins a new sprint and emits SprintStarted event.
// Returns new Engine and error (immutable pattern).
func (e Engine) StartSprint() (Engine, error) {
	// Calculate sprint data (what sim.StartSprint would do)
	sprintNumber := e.state().SprintNumber + 1
	startDay := e.state().CurrentTick
	bufferDays := float64(e.state().SprintLength) * e.state().BufferPct

	// Emit first - projection handler creates the sprint
	return e.emit(events.NewSprintStarted(e.state().ID, startDay, sprintNumber, bufferDays))
}

// RunSprint executes a complete sprint.
// Returns new Engine, UI events, and error (immutable pattern).
func (e Engine) RunSprint() (Engine, []model.Event, error) {
	var err error
	allEvents := make([]model.Event, 0)

	if e, err = e.StartSprint(); err != nil {
		return e, nil, err
	}

	// Read from projection (updated by SprintStarted emit in StartSprint)
	sprint, _ := e.state().CurrentSprintOption.Get()
	for e.state().CurrentTick < sprint.EndDay {
		var tickEvents []model.Event
		if e, tickEvents, err = e.Tick(); err != nil {
			return e, nil, err
		}
		allEvents = append(allEvents, tickEvents...)
	}

	return e, allEvents, nil
}

// AssignTicket assigns a ticket to a developer and starts work.
// Returns new Engine and error (immutable pattern).
// Error may be validation error or store conflict (caller should retry on conflict).
func (e Engine) AssignTicket(ticketID, devID string) (Engine, error) {
	// Validate ticket exists in backlog
	if e.state().FindBacklogTicketIndex(ticketID) == -1 {
		return e, fmt.Errorf("ticket %s not found in backlog", ticketID)
	}

	// Validate developer exists and is idle
	devIdx := e.state().FindDeveloperIndex(devID)
	if devIdx == -1 {
		return e, fmt.Errorf("developer %s not found", devID)
	}
	if !e.state().Developers[devIdx].IsIdle() {
		return e, fmt.Errorf("developer %s is busy with %s", devID, e.state().Developers[devIdx].CurrentTicket)
	}

	// Emit FIRST - projection handler does all the work:
	// - Moves ticket from backlog to active
	// - Sets AssignedTo, StartedTick, StartedAt, Phase, RemainingEffort
	// - Updates developer CurrentTicket, WIPCount
	// - Adds ticket to sprint
	startedAt := time.Now()
	return e.emit(events.NewTicketAssigned(e.state().ID, e.state().CurrentTick, ticketID, devID, startedAt))
}

// TryDecompose applies sizing policy and decomposes if needed.
// Returns new Engine, Either[NotDecomposable, []Ticket], and error (immutable pattern).
// Error is non-nil only for store conflicts (caller should retry).
func (e Engine) TryDecompose(ticketID string) (Engine, either.Either[NotDecomposable, []model.Ticket], error) {
	ticketIdx := e.state().FindBacklogTicketIndex(ticketID)
	if ticketIdx == -1 {
		return e, either.Left[NotDecomposable, []model.Ticket](NotDecomposable{Reason: "ticket not found"}), nil
	}

	ticket := e.state().Backlog[ticketIdx]

	if !e.policies.ShouldDecompose(ticket, e.state().SizingPolicy) {
		return e, either.Left[NotDecomposable, []model.Ticket](NotDecomposable{Reason: "policy forbids decomposition"}), nil
	}

	children := e.policies.Decompose(ticket)

	// Build ChildTicket slice for event
	childTickets := slice.MapTo[events.ChildTicket](children).Map(toChildTicket)

	// Emit TicketDecomposed - projection handler removes parent, adds children
	var err error
	if e, err = e.emit(events.NewTicketDecomposed(e.state().ID, e.state().CurrentTick, ticketID, childTickets)); err != nil {
		return e, either.Left[NotDecomposable, []model.Ticket](NotDecomposable{}), err
	}

	// Return children from projection (now populated by handler)
	// Find them by matching IDs from the event
	result := make([]model.Ticket, 0, len(children))
	for _, child := range childTickets { // justified:CF
		for _, t := range e.state().Backlog { // justified:CF
			if t.ID == child.ID {
				result = append(result, t)
				break
			}
		}
	}

	return e, either.Right[NotDecomposable](result), nil
}

// AddDeveloper adds a developer and emits DeveloperAdded event.
// Returns new Engine and error (immutable pattern).
func (e Engine) AddDeveloper(id, name string, velocity float64) (Engine, error) {
	return e.emit(events.NewDeveloperAdded(e.state().ID, e.state().CurrentTick, id, name, velocity))
}

// AddTicket adds a ticket to the backlog and emits TicketCreated event.
// Returns new Engine and error (immutable pattern).
func (e Engine) AddTicket(t model.Ticket) (Engine, error) {
	return e.emit(events.NewTicketCreated(e.state().ID, e.state().CurrentTick, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel))
}

// SetPolicy changes the sizing policy and emits PolicyChanged event.
// Returns new Engine and error (immutable pattern).
func (e Engine) SetPolicy(newPolicy model.SizingPolicy) (Engine, error) {
	oldPolicy := e.state().SizingPolicy
	return e.emit(events.NewPolicyChanged(e.state().ID, e.state().CurrentTick, oldPolicy, newPolicy))
}

// EmitLoadedState emits events for all current state in the simulation.
// Use this after loading from persistence to populate the projection.
// Returns new Engine and error (immutable pattern).
func (e Engine) EmitLoadedState(sim model.Simulation) (Engine, error) {
	var err error
	if e, err = e.emitLoadedConfig(sim); err != nil {
		return e, err
	}
	if e, err = e.emitLoadedTeam(sim); err != nil {
		return e, err
	}
	if e, err = e.emitLoadedBacklog(sim); err != nil {
		return e, err
	}
	if e, err = e.emitLoadedActiveTickets(sim); err != nil {
		return e, err
	}
	if e, err = e.emitLoadedCompletedTickets(sim); err != nil {
		return e, err
	}
	return e.emitLoadedProgress(sim)
}

func (e Engine) emitLoadedConfig(sim model.Simulation) (Engine, error) {
	return e.emit(events.NewSimulationCreated(
		sim.ID,
		0,
		events.SimConfig{
			TeamSize:     len(sim.Developers),
			SprintLength: sim.SprintLength,
			Seed:         sim.Seed,
			Policy:       sim.SizingPolicy,
		},
	))
}

func (e Engine) emitLoadedTeam(sim model.Simulation) (Engine, error) {
	var err error
	for _, dev := range sim.Developers { // justified:EP
		if e, err = e.emit(events.NewDeveloperAdded(e.state().ID, 0, dev.ID, dev.Name, dev.Velocity)); err != nil {
			return e, err
		}
	}
	return e, nil
}

func (e Engine) emitLoadedBacklog(sim model.Simulation) (Engine, error) {
	var err error
	for _, t := range sim.Backlog { // justified:EP
		if e, err = e.emit(events.NewTicketCreated(e.state().ID, 0, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel)); err != nil {
			return e, err
		}
	}
	return e, nil
}

// emitLoadedActiveTickets emits TicketCreated + TicketStateRestored for active tickets.
// Uses TicketStateRestored (not TicketAssigned) to preserve full state including Phase, RemainingEffort.
func (e Engine) emitLoadedActiveTickets(sim model.Simulation) (Engine, error) {
	var err error
	for _, t := range sim.ActiveTickets { // justified:EP
		if e, err = e.emit(events.NewTicketCreated(e.state().ID, 0, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel)); err != nil {
			return e, err
		}
		if e, err = e.emit(events.NewTicketStateRestored(e.state().ID, t.StartedTick, t.ID, t.AssignedTo, t.Phase, t.RemainingEffort, t.ActualDays, t.StartedAt)); err != nil {
			return e, err
		}
	}
	return e, nil
}

func (e Engine) emitLoadedCompletedTickets(sim model.Simulation) (Engine, error) {
	var err error
	for _, t := range sim.CompletedTickets { // justified:EP
		if e, err = e.emit(events.NewTicketCreated(e.state().ID, 0, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel)); err != nil {
			return e, err
		}
		if e, err = e.emit(events.NewTicketAssigned(e.state().ID, t.StartedTick, t.ID, t.AssignedTo, t.StartedAt)); err != nil {
			return e, err
		}
		if e, err = e.emit(events.NewTicketCompleted(e.state().ID, t.CompletedTick, t.ID, t.AssignedTo, t.ActualDays)); err != nil {
			return e, err
		}
	}
	return e, nil
}

func (e Engine) emitLoadedProgress(sim model.Simulation) (Engine, error) {
	var err error
	if sim.CurrentTick > 0 {
		if e, err = e.emit(events.NewTicked(e.state().ID, sim.CurrentTick)); err != nil {
			return e, err
		}
	}
	if sprint, ok := sim.CurrentSprintOption.Get(); ok {
		if e, err = e.emit(events.NewSprintStarted(e.state().ID, sprint.StartDay, sprint.Number, sprint.BufferDays)); err != nil {
			return e, err
		}
	}
	return e, nil
}

// toUIEvent converts an event-sourcing event to a UI display event.
// This bridges the two event systems: ES events for state mutations, UI events for display.
func (e Engine) toUIEvent(evt events.Event) model.Event {
	tick := evt.OccurrenceTime()
	switch ev := evt.(type) {
	case events.BugDiscovered:
		return model.NewEvent(model.EventBugDiscovered,
			fmt.Sprintf("Bug discovered in %s (+%.1f days)", ev.TicketID, ev.ReworkEffort), tick)
	case events.ScopeCreepOccurred:
		return model.NewEvent(model.EventScopeCreep,
			fmt.Sprintf("Scope creep on %s (+%.1f days)", ev.TicketID, ev.EffortAdded), tick)
	case events.IncidentStarted:
		return model.NewEvent(model.EventIncident,
			fmt.Sprintf("Incident %s: %s caused production issue", ev.IncidentID, ev.TicketID), tick)
	case events.IncidentResolved:
		return model.NewEvent(model.EventIncidentResolved,
			fmt.Sprintf("Incident %s resolved", ev.IncidentID), tick)
	default:
		return model.Event{} // Unknown event type - shouldn't happen for generator events
	}
}

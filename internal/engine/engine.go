package engine

import (
	"fmt"
	"time"

	"github.com/binaryphile/fluentfp/either"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// NotDecomposable explains why decomposition didn't happen.
// Value type: no pointers, use value semantics.
type NotDecomposable struct {
	Reason string
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
	proj     events.Projection   // Source of truth for event sourcing
	variance VarianceModel       // Value type: pure calculation
	evtGen   EventGenerator      // deterministic event generation
	policies PolicyEngine        // Value type: pure decision logic
	store    events.Store        // optional event store for event sourcing
	trace    events.TraceContext // current trace context for event correlation
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
		store:    store,
	}
}

// EmitCreated emits SimulationCreated event with the given config.
// Call after basic simulation setup is complete.
// Returns new Engine with updated state (immutable pattern).
func (e Engine) EmitCreated(id string, tick int, config events.SimConfig) Engine {
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
// Panics on store concurrency conflict (indicates bug in single-user simulation).
func (e Engine) emit(evt events.Event) Engine {
	// Apply trace context if set
	if !e.trace.IsEmpty() {
		evt = e.applyTrace(evt)
	}

	// Capture version BEFORE applying (for optimistic concurrency)
	expectedVersion := e.proj.Version()

	// Apply event to get new projection
	newProj := e.proj.Apply(evt)

	// Only append to store if configured
	if e.store != nil {
		if err := e.store.Append(evt.SimulationID(), expectedVersion, evt); err != nil {
			// Concurrency conflict - projection and store are now inconsistent
			// For single-user simulation, this indicates a bug (shouldn't happen)
			panic(fmt.Sprintf("event store concurrency conflict: %v", err))
		}
	}

	return e.withProj(newProj)
}

// withProj returns a new Engine with the given projection, preserving all other fields.
// Pure construction helper for immutable pattern.
func (e Engine) withProj(proj events.Projection) Engine {
	return Engine{
		proj:     proj,
		variance: e.variance,
		evtGen:   e.evtGen,
		policies: e.policies,
		store:    e.store,
		trace:    e.trace,
	}
}

// applyTrace applies the current trace context to an event using the Event interface.
func (e Engine) applyTrace(evt events.Event) events.Event {
	return events.ApplyTrace(evt, e.trace)
}

// Tick advances the simulation by one day.
// Returns new Engine and UI events (immutable pattern).
func (e Engine) Tick() (Engine, []model.Event) {
	allEvents := make([]model.Event, 0)

	// Increment tick: emit first, projection handler updates state
	newTick := e.state().CurrentTick + 1
	e = e.emit(events.NewTicked(e.state().ID, newTick))

	// 1. Developers work on assigned tickets
	// Read from projection after Ticked emit
	state := e.state()
	for i := range state.Developers {
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
		e = e.emit(events.NewWorkProgressed(e.state().ID, e.state().CurrentTick, ticket.ID, ticket.Phase, workDone))

		// Check phase completion from projection (now updated by WorkProgressed)
		ticket = e.state().ActiveTickets[ticketIdx]
		if ticket.RemainingEffort <= 0 {
			var uiEvents []model.Event
			e, uiEvents = e.advancePhaseEmitOnly(ticket, dev)
			allEvents = append(allEvents, uiEvents...)
		}
	}

	// 2. Generate random events (bugs, scope creep)
	// Generators return ES events; emit them and convert to UI events
	for _, evt := range e.evtGen.GenerateRandomEvents(e.state()) {
		e = e.emit(evt)
		allEvents = append(allEvents, e.toUIEvent(evt))
	}

	// 3. Check for incidents on recently deployed tickets
	for _, evt := range e.evtGen.CheckForIncidents(e.state()) {
		e = e.emit(evt)
		allEvents = append(allEvents, e.toUIEvent(evt))
	}

	// 4. Update sprint buffer
	e = e.updateBuffer()

	// 5. Track WIP for export
	e = e.trackWIP()

	// 6. Check sprint end (read from projection - updated by earlier emits)
	if sprint, ok := e.state().CurrentSprintOption.Get(); ok && e.state().CurrentTick >= sprint.EndDay {
		var endEvents []model.Event
		e, endEvents = e.endSprint()
		allEvents = append(allEvents, endEvents...)
	}

	return e, allEvents
}

// advancePhaseEmitOnly emits phase change events - projection handlers update state.
// Returns new Engine and UI events (immutable pattern).
func (e Engine) advancePhaseEmitOnly(ticket model.Ticket, dev model.Developer) (Engine, []model.Event) {
	modelEvents := make([]model.Event, 0)

	oldPhase := ticket.Phase
	newPhase := oldPhase + 1

	if newPhase == model.PhaseDone {
		// Emit TicketCompleted - projection handler:
		// - Moves ticket to CompletedTickets
		// - Updates developer stats (CurrentTicket="", TicketsCompleted++, etc)
		e = e.emit(events.NewTicketCompleted(e.state().ID, e.state().CurrentTick, ticket.ID, dev.ID, ticket.ActualDays))

		modelEvents = append(modelEvents, model.NewEvent(
			model.EventTicketComplete,
			fmt.Sprintf("%s completed (%.1f days actual vs %.1f estimated)", ticket.ID, ticket.ActualDays, ticket.EstimatedDays),
			e.state().CurrentTick,
		))
	} else {
		// Emit TicketPhaseChanged - projection handler:
		// - Updates ticket.Phase
		// - Sets ticket.RemainingEffort for new phase
		e = e.emit(events.NewTicketPhaseChanged(e.state().ID, e.state().CurrentTick, ticket.ID, oldPhase, newPhase))

		modelEvents = append(modelEvents, model.NewEvent(
			model.EventPhaseAdvance,
			fmt.Sprintf("%s: %s -> %s", ticket.ID, oldPhase, newPhase),
			e.state().CurrentTick,
		))
	}

	return e, modelEvents
}

// updateBuffer consumes buffer when tickets are behind schedule.
// Returns new Engine (immutable pattern).
func (e Engine) updateBuffer() Engine {
	sprint, ok := e.state().CurrentSprintOption.Get()
	if !ok {
		return e
	}

	// Calculate expected vs actual progress
	progressPct := sprint.ProgressPct(e.state().CurrentTick)
	expectedComplete := progressPct * float64(len(e.state().ActiveTickets))

	// completedInCurrentSprint returns true if ticket was completed after sprint started.
	completedInCurrentSprint := func(t model.Ticket) bool { return t.CompletedTick >= sprint.StartDay }
	completedInSprint := slice.From(e.state().CompletedTickets).
		KeepIf(completedInCurrentSprint).
		Len()

	// If behind schedule, consume buffer
	if float64(completedInSprint) < expectedComplete {
		bufferConsumption := (expectedComplete - float64(completedInSprint)) * 0.1
		// Emit BufferConsumed - projection handler updates sprint.BufferConsumed
		e = e.emit(events.NewBufferConsumed(e.state().ID, e.state().CurrentTick, bufferConsumption))
	}

	return e
}

// trackWIP records work-in-progress metrics for export.
// Returns new Engine (immutable pattern).
func (e Engine) trackWIP() Engine {
	_, ok := e.state().CurrentSprintOption.Get()
	if !ok {
		return e
	}

	currentWIP := len(e.state().ActiveTickets)

	// Emit event - projection handler will update WIP metrics
	return e.emit(events.NewSprintWIPUpdated(e.state().ID, e.state().CurrentTick, currentWIP))
}

// endSprint handles sprint completion.
// Returns new Engine and UI events (immutable pattern).
func (e Engine) endSprint() (Engine, []model.Event) {
	modelEvents := make([]model.Event, 0)

	sprint, ok := e.state().CurrentSprintOption.Get()
	if ok {
		// Emit SprintEnded event
		e = e.emit(events.NewSprintEnded(e.state().ID, e.state().CurrentTick, sprint.Number))
	}

	// Any unfinished active tickets stay active for next sprint
	// (In a real sim, we might handle carryover differently)

	return e, modelEvents
}

// StartSprint begins a new sprint and emits SprintStarted event.
// Returns new Engine (immutable pattern).
func (e Engine) StartSprint() Engine {
	// Calculate sprint data (what sim.StartSprint would do)
	sprintNumber := e.state().SprintNumber + 1
	startDay := e.state().CurrentTick
	bufferDays := float64(e.state().SprintLength) * e.state().BufferPct

	// Emit first - projection handler creates the sprint
	return e.emit(events.NewSprintStarted(e.state().ID, startDay, sprintNumber, bufferDays))
}

// RunSprint executes a complete sprint.
// Returns new Engine and UI events (immutable pattern).
func (e Engine) RunSprint() (Engine, []model.Event) {
	allEvents := make([]model.Event, 0)

	e = e.StartSprint()

	// Read from projection (updated by SprintStarted emit in StartSprint)
	sprint, _ := e.state().CurrentSprintOption.Get()
	for e.state().CurrentTick < sprint.EndDay {
		var tickEvents []model.Event
		e, tickEvents = e.Tick()
		allEvents = append(allEvents, tickEvents...)
	}

	return e, allEvents
}

// AssignTicket assigns a ticket to a developer and starts work.
// Returns new Engine and error (immutable pattern).
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
	e = e.emit(events.NewTicketAssigned(e.state().ID, e.state().CurrentTick, ticketID, devID, startedAt))

	return e, nil
}

// TryDecompose applies sizing policy and decomposes if needed.
// Returns new Engine and Either[NotDecomposable, []Ticket] (immutable pattern).
func (e Engine) TryDecompose(ticketID string) (Engine, either.Either[NotDecomposable, []model.Ticket]) {
	ticketIdx := e.state().FindBacklogTicketIndex(ticketID)
	if ticketIdx == -1 {
		return e, either.Left[NotDecomposable, []model.Ticket](NotDecomposable{Reason: "ticket not found"})
	}

	ticket := e.state().Backlog[ticketIdx]

	if !e.policies.ShouldDecompose(ticket, e.state().SizingPolicy) {
		return e, either.Left[NotDecomposable, []model.Ticket](NotDecomposable{Reason: "policy forbids decomposition"})
	}

	children := e.policies.Decompose(ticket)

	// Build ChildTicket slice for event
	childTickets := make([]events.ChildTicket, len(children))
	for i, c := range children {
		childTickets[i] = events.ChildTicket{
			ID:            c.ID,
			Title:         c.Title,
			EstimatedDays: c.EstimatedDays,
			Understanding: c.UnderstandingLevel,
		}
	}

	// Emit TicketDecomposed - projection handler removes parent, adds children
	e = e.emit(events.NewTicketDecomposed(e.state().ID, e.state().CurrentTick, ticketID, childTickets))

	// Return children from projection (now populated by handler)
	// Find them by matching IDs from the event
	result := make([]model.Ticket, 0, len(children))
	for _, child := range childTickets {
		for _, t := range e.state().Backlog {
			if t.ID == child.ID {
				result = append(result, t)
				break
			}
		}
	}

	return e, either.Right[NotDecomposable](result)
}

// AddDeveloper adds a developer and emits DeveloperAdded event.
// Returns new Engine (immutable pattern).
func (e Engine) AddDeveloper(id, name string, velocity float64) Engine {
	return e.emit(events.NewDeveloperAdded(e.state().ID, e.state().CurrentTick, id, name, velocity))
}

// AddTicket adds a ticket to the backlog and emits TicketCreated event.
// Returns new Engine (immutable pattern).
func (e Engine) AddTicket(t model.Ticket) Engine {
	return e.emit(events.NewTicketCreated(e.state().ID, e.state().CurrentTick, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel))
}

// SetPolicy changes the sizing policy and emits PolicyChanged event.
// Returns new Engine (immutable pattern).
func (e Engine) SetPolicy(newPolicy model.SizingPolicy) Engine {
	oldPolicy := e.state().SizingPolicy
	return e.emit(events.NewPolicyChanged(e.state().ID, e.state().CurrentTick, oldPolicy, newPolicy))
}

// EmitLoadedState emits events for all current state in the simulation.
// Use this after loading from persistence to populate the projection.
// Returns new Engine (immutable pattern).
func (e Engine) EmitLoadedState(sim model.Simulation) Engine {
	// Emit SimulationCreated
	e = e.emit(events.NewSimulationCreated(
		sim.ID,
		0, // Original tick 0
		events.SimConfig{
			TeamSize:     len(sim.Developers),
			SprintLength: sim.SprintLength,
			Seed:         sim.Seed,
			Policy:       sim.SizingPolicy,
		},
	))

	// After SimulationCreated emit, e.state().ID is available
	// Emit DeveloperAdded for each developer
	for _, dev := range sim.Developers {
		e = e.emit(events.NewDeveloperAdded(e.state().ID, 0, dev.ID, dev.Name, dev.Velocity))
	}

	// Emit TicketCreated for backlog tickets
	for _, t := range sim.Backlog {
		e = e.emit(events.NewTicketCreated(e.state().ID, 0, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel))
	}

	// Emit TicketCreated + TicketStateRestored for active tickets
	// Use TicketStateRestored (not TicketAssigned) to preserve full state including Phase, RemainingEffort
	for _, t := range sim.ActiveTickets {
		e = e.emit(events.NewTicketCreated(e.state().ID, 0, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel))
		e = e.emit(events.NewTicketStateRestored(e.state().ID, t.StartedTick, t.ID, t.AssignedTo, t.Phase, t.RemainingEffort, t.ActualDays, t.StartedAt))
	}

	// Emit TicketCreated + TicketAssigned + TicketCompleted for completed tickets
	for _, t := range sim.CompletedTickets {
		e = e.emit(events.NewTicketCreated(e.state().ID, 0, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel))
		e = e.emit(events.NewTicketAssigned(e.state().ID, t.StartedTick, t.ID, t.AssignedTo, t.StartedAt))
		e = e.emit(events.NewTicketCompleted(e.state().ID, t.CompletedTick, t.ID, t.AssignedTo, t.ActualDays))
	}

	// Emit Ticked to set current tick
	if sim.CurrentTick > 0 {
		e = e.emit(events.NewTicked(e.state().ID, sim.CurrentTick))
	}

	// Emit SprintStarted if sprint is active
	if sprint, ok := sim.CurrentSprintOption.Get(); ok {
		e = e.emit(events.NewSprintStarted(e.state().ID, sprint.StartDay, sprint.Number, sprint.BufferDays))
	}

	return e
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

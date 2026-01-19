package engine

import (
	"fmt"
	"time"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Engine runs the simulation tick loop.
// Pointer receiver: mutates sim and proj fields.
type Engine struct {
	sim      *model.Simulation   // Legacy: will be removed in Step 2.3f
	proj     events.Projection   // Source of truth for event sourcing
	variance VarianceModel       // Value type: pure calculation
	evtGen   *EventGenerator     // Keep pointer: has *rand.Rand (stateful)
	policies PolicyEngine        // Value type: pure decision logic
	store    events.Store        // optional event store for event sourcing
	trace    events.TraceContext // current trace context for event correlation
}

// NewEngine creates a simulation engine without event sourcing
func NewEngine(sim *model.Simulation) *Engine {
	return &Engine{
		sim:      sim,
		proj:     events.NewProjection(),
		variance: NewVarianceModel(sim.Seed),
		evtGen:   NewEventGenerator(sim.Seed),
		policies: NewPolicyEngine(sim.Seed),
	}
}

// NewEngineWithStore creates a simulation engine with event sourcing.
// Call EmitCreated() after simulation setup is complete to emit SimulationCreated event.
func NewEngineWithStore(sim *model.Simulation, store events.Store) *Engine {
	return &Engine{
		sim:      sim,
		proj:     events.NewProjection(),
		variance: NewVarianceModel(sim.Seed),
		evtGen:   NewEventGenerator(sim.Seed),
		policies: NewPolicyEngine(sim.Seed),
		store:    store,
	}
}

// EmitCreated emits SimulationCreated event. Call after simulation setup is complete.
func (e *Engine) EmitCreated() {
	e.emit(events.NewSimulationCreated(
		e.sim.ID,
		e.sim.CurrentTick,
		events.SimConfig{
			TeamSize:     len(e.sim.Developers),
			SprintLength: e.sim.SprintLength,
			Seed:         e.sim.Seed,
		},
	))
}

// SetTrace sets the current trace context for event correlation.
// All events emitted will include this trace information.
func (e *Engine) SetTrace(tc events.TraceContext) {
	e.trace = tc
}

// ClearTrace clears the current trace context.
func (e *Engine) ClearTrace() {
	e.trace = events.TraceContext{}
}

// CurrentTrace returns the current trace context.
func (e *Engine) CurrentTrace() events.TraceContext {
	return e.trace
}

// Sim returns the current simulation state derived from the projection.
// This is the primary way to access state in event-sourced mode.
func (e *Engine) Sim() model.Simulation {
	return e.proj.State()
}

// emit sends an event to the store if configured, attaching trace context if set.
// Also applies the event to the projection to keep derived state in sync.
func (e *Engine) emit(evt events.Event) {
	// Apply trace context if set
	if !e.trace.IsEmpty() {
		evt = e.applyTrace(evt)
	}

	// Always update projection (whether or not store is configured)
	e.proj = e.proj.Apply(evt)

	// Only append to store if configured
	if e.store != nil {
		e.store.Append(e.sim.ID, evt)
	}
}

// applyTrace applies the current trace context to an event using the Event interface.
func (e *Engine) applyTrace(evt events.Event) events.Event {
	return events.ApplyTrace(evt, e.trace)
}

// Tick advances the simulation by one day
func (e *Engine) Tick() []model.Event {
	allEvents := make([]model.Event, 0)
	e.sim.CurrentTick++

	// Emit Ticked event
	e.emit(events.NewTicked(e.sim.ID, e.sim.CurrentTick))

	// 1. Developers work on assigned tickets
	for i := range e.sim.Developers {
		dev := e.sim.Developers[i]
		if dev.IsIdle() {
			continue
		}

		ticketIdx := e.sim.FindActiveTicketIndex(dev.CurrentTicket)
		if ticketIdx == -1 {
			e.sim.Developers[i] = dev.WithoutTicket()
			continue
		}

		ticket := e.sim.ActiveTickets[ticketIdx]

		// Calculate work done with variance
		variance := e.variance.Calculate(ticket, e.sim.CurrentTick)
		workDone := dev.Velocity * variance
		ticket.RemainingEffort -= workDone
		ticket.PhaseEffortSpent[ticket.Phase] += workDone
		ticket.ActualDays += workDone

		// Emit WorkProgressed event
		e.emit(events.NewWorkProgressed(e.sim.ID, e.sim.CurrentTick, ticket.ID, ticket.Phase, workDone))

		// Check phase completion
		if ticket.RemainingEffort <= 0 {
			events, updatedTicket, updatedDev := e.advancePhase(ticket, dev)
			allEvents = append(allEvents, events...)
			ticket = updatedTicket
			dev = updatedDev

			if ticket.Phase == model.PhaseDone {
				// Ticket completed - add to completed and remove from active
				e.sim.CompletedTickets = append(e.sim.CompletedTickets, ticket)
				e.sim.ActiveTickets = append(e.sim.ActiveTickets[:ticketIdx], e.sim.ActiveTickets[ticketIdx+1:]...)
				e.sim.Developers[i] = dev
				continue
			}
		}

		// Write back (still active)
		e.sim.ActiveTickets[ticketIdx] = ticket
		e.sim.Developers[i] = dev
	}

	// 2. Generate random events (bugs, scope creep)
	randomEvents := e.evtGen.GenerateRandomEvents(e.sim)
	allEvents = append(allEvents, randomEvents...)

	// 3. Check for incidents on recently deployed tickets
	incidentEvents := e.evtGen.CheckForIncidents(e.sim)
	allEvents = append(allEvents, incidentEvents...)

	// 4. Update sprint buffer
	e.updateBuffer()

	// 5. Track WIP for export
	e.trackWIP()

	// 6. Check sprint end
	if sprint, ok := e.sim.CurrentSprintOption.Get(); ok && e.sim.CurrentTick >= sprint.EndDay {
		endEvents := e.endSprint()
		allEvents = append(allEvents, endEvents...)
	}

	return allEvents
}

// advancePhase moves a ticket to the next phase or completes it
func (e *Engine) advancePhase(ticket model.Ticket, dev model.Developer) ([]model.Event, model.Ticket, model.Developer) {
	modelEvents := make([]model.Event, 0)

	oldPhase := ticket.Phase
	ticket.Phase++

	if ticket.Phase == model.PhaseDone {
		// Ticket complete
		ticket.CompletedAt = time.Now()
		ticket.CompletedTick = e.sim.CurrentTick
		dev = dev.WithCompletedTicket(ticket.ActualDays)

		// Emit TicketCompleted event
		e.emit(events.NewTicketCompleted(e.sim.ID, e.sim.CurrentTick, ticket.ID, dev.ID))

		modelEvents = append(modelEvents, model.NewEvent(
			model.EventTicketComplete,
			fmt.Sprintf("%s completed (%.1f days actual vs %.1f estimated)", ticket.ID, ticket.ActualDays, ticket.EstimatedDays),
			e.sim.CurrentTick,
		))
	} else {
		// Advancing to next phase
		ticket.RemainingEffort = ticket.CalculatePhaseEffort(ticket.Phase)

		// Emit TicketPhaseChanged event
		e.emit(events.NewTicketPhaseChanged(e.sim.ID, e.sim.CurrentTick, ticket.ID, oldPhase, ticket.Phase))

		modelEvents = append(modelEvents, model.NewEvent(
			model.EventPhaseAdvance,
			fmt.Sprintf("%s: %s -> %s", ticket.ID, oldPhase, ticket.Phase),
			e.sim.CurrentTick,
		))
	}

	return modelEvents, ticket, dev
}

// updateBuffer consumes buffer when tickets are behind schedule
func (e *Engine) updateBuffer() {
	sprint, ok := e.sim.CurrentSprintOption.Get()
	if !ok {
		return
	}

	// Calculate expected vs actual progress
	progressPct := sprint.ProgressPct(e.sim.CurrentTick)
	expectedComplete := progressPct * float64(len(e.sim.ActiveTickets))

	// completedInCurrentSprint returns true if ticket was completed after sprint started.
	completedInCurrentSprint := func(t model.Ticket) bool { return t.CompletedTick >= sprint.StartDay }
	completedInSprint := slice.From(e.sim.CompletedTickets).
		KeepIf(completedInCurrentSprint).
		Len()

	// If behind schedule, consume buffer
	if float64(completedInSprint) < expectedComplete {
		bufferConsumption := (expectedComplete - float64(completedInSprint)) * 0.1
		sprint = sprint.WithConsumedBuffer(bufferConsumption)
		e.sim.CurrentSprintOption = option.Of(sprint)

		// Emit BufferConsumed event for projection tracking
		e.emit(events.NewBufferConsumed(e.sim.ID, e.sim.CurrentTick, bufferConsumption))
	}
}

// trackWIP records work-in-progress metrics for export
func (e *Engine) trackWIP() {
	sprint, ok := e.sim.CurrentSprintOption.Get()
	if !ok {
		return
	}

	currentWIP := len(e.sim.ActiveTickets)

	if currentWIP > sprint.MaxWIP {
		sprint.MaxWIP = currentWIP
	}
	sprint.WIPSum += currentWIP
	sprint.WIPTicks++

	e.sim.CurrentSprintOption = option.Of(sprint)
}

// endSprint handles sprint completion
func (e *Engine) endSprint() []model.Event {
	modelEvents := make([]model.Event, 0)

	sprint, ok := e.sim.CurrentSprintOption.Get()
	if ok {
		// Emit SprintEnded event
		e.emit(events.NewSprintEnded(e.sim.ID, e.sim.CurrentTick, sprint.Number))
	}

	// Any unfinished active tickets stay active for next sprint
	// (In a real sim, we might handle carryover differently)

	return modelEvents
}

// StartSprint begins a new sprint and emits SprintStarted event
func (e *Engine) StartSprint() {
	e.sim.StartSprint()

	sprint, _ := e.sim.CurrentSprintOption.Get()
	e.emit(events.NewSprintStarted(e.sim.ID, sprint.StartDay, sprint.Number, sprint.BufferDays))
}

// RunSprint executes a complete sprint
func (e *Engine) RunSprint() []model.Event {
	allEvents := make([]model.Event, 0)

	e.StartSprint()

	sprint, _ := e.sim.CurrentSprintOption.Get()
	for e.sim.CurrentTick < sprint.EndDay {
		events := e.Tick()
		allEvents = append(allEvents, events...)
	}

	return allEvents
}

// AssignTicket assigns a ticket to a developer and starts work
func (e *Engine) AssignTicket(ticketID, devID string) error {
	ticketIdx := e.sim.FindBacklogTicketIndex(ticketID)
	if ticketIdx == -1 {
		return fmt.Errorf("ticket %s not found in backlog", ticketID)
	}

	devIdx := e.sim.FindDeveloperIndex(devID)
	if devIdx == -1 {
		return fmt.Errorf("developer %s not found", devID)
	}

	dev := e.sim.Developers[devIdx]
	if !dev.IsIdle() {
		return fmt.Errorf("developer %s is busy with %s", devID, dev.CurrentTicket)
	}

	// Move from backlog to active
	e.moveToActive(ticketID)

	// Find ticket in active (it was just moved)
	ticketIdx = e.sim.FindActiveTicketIndex(ticketID)
	ticket := e.sim.ActiveTickets[ticketIdx]

	// Start the ticket
	ticket.AssignedTo = devID
	ticket.StartedAt = time.Now()
	ticket.StartedTick = e.sim.CurrentTick
	ticket.Phase = model.PhaseResearch
	ticket.RemainingEffort = ticket.CalculatePhaseEffort(model.PhaseResearch)
	e.sim.ActiveTickets[ticketIdx] = ticket

	// Assign to developer
	e.sim.Developers[devIdx] = dev.WithTicket(ticketID)

	// Emit TicketAssigned event
	e.emit(events.NewTicketAssigned(e.sim.ID, e.sim.CurrentTick, ticketID, devID))

	// Add to sprint if there is one
	if sprint, ok := e.sim.CurrentSprintOption.Get(); ok {
		sprint = sprint.WithTicket(ticketID)
		e.sim.CurrentSprintOption = option.Of(sprint)
	}

	return nil
}

// moveToActive moves a ticket from backlog to active
func (e *Engine) moveToActive(ticketID string) {
	for i, t := range e.sim.Backlog {
		if t.ID == ticketID {
			e.sim.ActiveTickets = append(e.sim.ActiveTickets, t)
			e.sim.Backlog = append(e.sim.Backlog[:i], e.sim.Backlog[i+1:]...)
			return
		}
	}
}

// TryDecompose applies sizing policy and decomposes if needed
func (e *Engine) TryDecompose(ticketID string) ([]model.Ticket, bool) {
	ticketIdx := e.sim.FindBacklogTicketIndex(ticketID)
	if ticketIdx == -1 {
		return nil, false
	}

	ticket := e.sim.Backlog[ticketIdx]

	if !e.policies.ShouldDecompose(ticket, e.sim.SizingPolicy) {
		return nil, false
	}

	children := e.policies.Decompose(ticket)

	// Remove parent from backlog
	e.sim.Backlog = append(e.sim.Backlog[:ticketIdx], e.sim.Backlog[ticketIdx+1:]...)

	// Add children to backlog
	for _, child := range children {
		e.sim.Backlog = append(e.sim.Backlog, child)
	}

	return children, true
}

// AddDeveloper adds a developer and emits DeveloperAdded event.
func (e *Engine) AddDeveloper(id, name string, velocity float64) {
	// Add to legacy sim (for backwards compatibility during migration)
	e.sim.AddDeveloper(model.NewDeveloper(id, name, velocity))

	// Emit event (also updates projection via emit())
	e.emit(events.NewDeveloperAdded(e.sim.ID, e.sim.CurrentTick, id, name, velocity))
}

// AddTicket adds a ticket to the backlog and emits TicketCreated event.
func (e *Engine) AddTicket(t model.Ticket) {
	// Add to legacy sim (for backwards compatibility during migration)
	e.sim.AddTicket(t)

	// Emit event (also updates projection via emit())
	e.emit(events.NewTicketCreated(e.sim.ID, e.sim.CurrentTick, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel))
}

// SetPolicy changes the sizing policy and emits PolicyChanged event.
func (e *Engine) SetPolicy(newPolicy model.SizingPolicy) {
	oldPolicy := e.sim.SizingPolicy

	// Update legacy sim (for backwards compatibility during migration)
	e.sim.SizingPolicy = newPolicy

	// Emit event (also updates projection via emit())
	e.emit(events.NewPolicyChanged(e.sim.ID, e.sim.CurrentTick, oldPolicy, newPolicy))
}

// EmitLoadedState emits events for all current state in the simulation.
// Use this after loading from persistence to populate the projection.
func (e *Engine) EmitLoadedState() {
	// Emit SimulationCreated
	e.emit(events.NewSimulationCreated(
		e.sim.ID,
		0, // Original tick 0
		events.SimConfig{
			TeamSize:     len(e.sim.Developers),
			SprintLength: e.sim.SprintLength,
			Seed:         e.sim.Seed,
			Policy:       e.sim.SizingPolicy,
		},
	))

	// Emit DeveloperAdded for each developer
	for _, dev := range e.sim.Developers {
		e.emit(events.NewDeveloperAdded(e.sim.ID, 0, dev.ID, dev.Name, dev.Velocity))
	}

	// Emit TicketCreated for backlog tickets
	for _, t := range e.sim.Backlog {
		e.emit(events.NewTicketCreated(e.sim.ID, 0, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel))
	}

	// Emit TicketCreated + TicketAssigned for active tickets
	for _, t := range e.sim.ActiveTickets {
		e.emit(events.NewTicketCreated(e.sim.ID, 0, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel))
		e.emit(events.NewTicketAssigned(e.sim.ID, t.StartedTick, t.ID, t.AssignedTo))
	}

	// Emit TicketCreated + TicketAssigned + TicketCompleted for completed tickets
	for _, t := range e.sim.CompletedTickets {
		e.emit(events.NewTicketCreated(e.sim.ID, 0, t.ID, t.Title, t.EstimatedDays, t.UnderstandingLevel))
		e.emit(events.NewTicketAssigned(e.sim.ID, t.StartedTick, t.ID, t.AssignedTo))
		e.emit(events.NewTicketCompleted(e.sim.ID, t.CompletedTick, t.ID, t.AssignedTo))
	}

	// Emit Ticked to set current tick
	if e.sim.CurrentTick > 0 {
		e.emit(events.NewTicked(e.sim.ID, e.sim.CurrentTick))
	}

	// Emit SprintStarted if sprint is active
	if sprint, ok := e.sim.CurrentSprintOption.Get(); ok {
		e.emit(events.NewSprintStarted(e.sim.ID, sprint.StartDay, sprint.Number, sprint.BufferDays))
	}
}

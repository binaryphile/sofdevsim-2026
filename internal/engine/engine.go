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
// Pointer receiver: mutates sim field (will be replaced by Projection in Phase 2).
type Engine struct {
	sim      *model.Simulation   // Will be replaced by Projection in Phase 2
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

// emit sends an event to the store if configured, attaching trace context if set.
func (e *Engine) emit(evt events.Event) {
	if e.store == nil {
		return
	}

	// Apply trace context if set
	if !e.trace.IsEmpty() {
		evt = e.applyTrace(evt)
	}

	e.store.Append(e.sim.ID, evt)
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
	events := make([]model.Event, 0)

	// Any unfinished active tickets stay active for next sprint
	// (In a real sim, we might handle carryover differently)

	return events
}

// StartSprint begins a new sprint and emits SprintStarted event
func (e *Engine) StartSprint() {
	e.sim.StartSprint()

	sprint, _ := e.sim.CurrentSprintOption.Get()
	e.emit(events.NewSprintStarted(e.sim.ID, sprint.StartDay, sprint.Number))
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

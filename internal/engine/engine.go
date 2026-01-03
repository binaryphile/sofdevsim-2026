package engine

import (
	"fmt"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Engine runs the simulation tick loop
type Engine struct {
	sim       *model.Simulation
	variance  *VarianceModel
	events    *EventGenerator
	policies  *PolicyEngine
}

// NewEngine creates a simulation engine
func NewEngine(sim *model.Simulation) *Engine {
	return &Engine{
		sim:      sim,
		variance: NewVarianceModel(sim.Seed),
		events:   NewEventGenerator(sim.Seed),
		policies: NewPolicyEngine(sim.Seed),
	}
}

// Tick advances the simulation by one day
func (e *Engine) Tick() []model.Event {
	allEvents := make([]model.Event, 0)
	e.sim.CurrentTick++

	// 1. Developers work on assigned tickets
	for i := range e.sim.Developers {
		dev := &e.sim.Developers[i]
		if dev.IsIdle() {
			continue
		}

		ticket := e.sim.FindTicketByID(dev.CurrentTicket)
		if ticket == nil {
			dev.Unassign()
			continue
		}

		// Calculate work done with variance
		variance := e.variance.Calculate(ticket, e.sim.CurrentTick)
		workDone := dev.Velocity * variance
		ticket.RemainingEffort -= workDone
		ticket.PhaseEffortSpent[ticket.Phase] += workDone
		ticket.ActualDays += workDone

		// Check phase completion
		if ticket.RemainingEffort <= 0 {
			events := e.advancePhase(ticket, dev)
			allEvents = append(allEvents, events...)
		}
	}

	// 2. Generate random events (bugs, scope creep)
	randomEvents := e.events.GenerateRandomEvents(e.sim)
	allEvents = append(allEvents, randomEvents...)

	// 3. Check for incidents on recently deployed tickets
	incidentEvents := e.events.CheckForIncidents(e.sim)
	allEvents = append(allEvents, incidentEvents...)

	// 4. Update sprint buffer
	e.updateBuffer()

	// 5. Track WIP for export
	e.trackWIP()

	// 6. Check sprint end
	if e.sim.CurrentSprint != nil && e.sim.CurrentTick >= e.sim.CurrentSprint.EndDay {
		endEvents := e.endSprint()
		allEvents = append(allEvents, endEvents...)
	}

	return allEvents
}

// advancePhase moves a ticket to the next phase or completes it
func (e *Engine) advancePhase(ticket *model.Ticket, dev *model.Developer) []model.Event {
	events := make([]model.Event, 0)

	oldPhase := ticket.Phase
	ticket.Phase++

	if ticket.Phase == model.PhaseDone {
		// Ticket complete
		ticket.CompletedAt = time.Now()
		ticket.CompletedTick = e.sim.CurrentTick
		dev.CompleteTicket(ticket.ActualDays)

		// Move from active to completed
		e.moveToCompleted(ticket.ID)

		events = append(events, model.NewEvent(
			model.EventTicketComplete,
			fmt.Sprintf("%s completed (%.1f days actual vs %.1f estimated)", ticket.ID, ticket.ActualDays, ticket.EstimatedDays),
			e.sim.CurrentTick,
		))
	} else {
		// Advancing to next phase
		ticket.RemainingEffort = ticket.CalculatePhaseEffort(ticket.Phase)

		events = append(events, model.NewEvent(
			model.EventPhaseAdvance,
			fmt.Sprintf("%s: %s -> %s", ticket.ID, oldPhase, ticket.Phase),
			e.sim.CurrentTick,
		))
	}

	return events
}

// moveToCompleted moves a ticket from active to completed
func (e *Engine) moveToCompleted(ticketID string) {
	for i, t := range e.sim.ActiveTickets {
		if t.ID == ticketID {
			e.sim.CompletedTickets = append(e.sim.CompletedTickets, t)
			e.sim.ActiveTickets = append(e.sim.ActiveTickets[:i], e.sim.ActiveTickets[i+1:]...)
			return
		}
	}
}

// updateBuffer consumes buffer when tickets are behind schedule
func (e *Engine) updateBuffer() {
	if e.sim.CurrentSprint == nil {
		return
	}

	// Calculate expected vs actual progress
	progressPct := e.sim.CurrentSprint.ProgressPct(e.sim.CurrentTick)
	expectedComplete := progressPct * float64(len(e.sim.ActiveTickets))

	completedInSprint := 0
	for _, t := range e.sim.CompletedTickets {
		if t.CompletedTick >= e.sim.CurrentSprint.StartDay {
			completedInSprint++
		}
	}

	// If behind schedule, consume buffer
	if float64(completedInSprint) < expectedComplete {
		bufferConsumption := (expectedComplete - float64(completedInSprint)) * 0.1
		e.sim.CurrentSprint.ConsumeBuffer(bufferConsumption)
	}
}

// trackWIP records work-in-progress metrics for export
func (e *Engine) trackWIP() {
	if e.sim.CurrentSprint == nil {
		return
	}

	currentWIP := len(e.sim.ActiveTickets)
	if currentWIP > e.sim.CurrentSprint.MaxWIP {
		e.sim.CurrentSprint.MaxWIP = currentWIP
	}
	e.sim.CurrentSprint.WIPSum += currentWIP
	e.sim.CurrentSprint.WIPTicks++
}

// endSprint handles sprint completion
func (e *Engine) endSprint() []model.Event {
	events := make([]model.Event, 0)

	// Any unfinished active tickets stay active for next sprint
	// (In a real sim, we might handle carryover differently)

	return events
}

// RunSprint executes a complete sprint
func (e *Engine) RunSprint() []model.Event {
	allEvents := make([]model.Event, 0)

	e.sim.StartSprint()

	for e.sim.CurrentTick < e.sim.CurrentSprint.EndDay {
		events := e.Tick()
		allEvents = append(allEvents, events...)
	}

	return allEvents
}

// AssignTicket assigns a ticket to a developer and starts work
func (e *Engine) AssignTicket(ticketID, devID string) error {
	ticket := e.sim.FindTicketByID(ticketID)
	if ticket == nil {
		return fmt.Errorf("ticket %s not found", ticketID)
	}

	dev := e.sim.FindDeveloperByID(devID)
	if dev == nil {
		return fmt.Errorf("developer %s not found", devID)
	}

	if !dev.IsIdle() {
		return fmt.Errorf("developer %s is busy with %s", devID, dev.CurrentTicket)
	}

	// Move from backlog to active
	e.moveToActive(ticketID)

	// Start the ticket
	ticket = e.sim.FindTicketByID(ticketID) // Re-find after move
	ticket.AssignedTo = devID
	ticket.StartedAt = time.Now()
	ticket.StartedTick = e.sim.CurrentTick
	ticket.Phase = model.PhaseResearch
	ticket.RemainingEffort = ticket.CalculatePhaseEffort(model.PhaseResearch)

	// Assign to developer
	dev.Assign(ticketID)

	// Add to sprint if there is one
	if e.sim.CurrentSprint != nil {
		e.sim.CurrentSprint.AddTicket(ticketID)
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
	ticket := e.sim.FindTicketByID(ticketID)
	if ticket == nil {
		return nil, false
	}

	if !e.policies.ShouldDecompose(*ticket, e.sim.SizingPolicy) {
		return nil, false
	}

	children := e.policies.Decompose(*ticket)

	// Remove parent from backlog
	for i, t := range e.sim.Backlog {
		if t.ID == ticketID {
			e.sim.Backlog = append(e.sim.Backlog[:i], e.sim.Backlog[i+1:]...)
			break
		}
	}

	// Add children to backlog
	for _, child := range children {
		e.sim.Backlog = append(e.sim.Backlog, child)
	}

	// Update parent with child IDs
	ticket.ChildIDs = make([]string, len(children))
	for i, child := range children {
		ticket.ChildIDs[i] = child.ID
	}

	return children, true
}

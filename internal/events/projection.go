package events

import (
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Projection rebuilds Simulation state from events.
// Value receiver: immutable, returns new Projection with updated state.
// This is the core of event sourcing - state is derived, not stored.
type Projection struct {
	sim     model.Simulation // Value, not pointer - enables value semantics
	version int              // Event count for optimistic concurrency
}

// NewProjection creates an empty projection.
func NewProjection() Projection {
	return Projection{version: 0}
}

// Apply processes a single event, returning new Projection.
// Pure function: no side effects. Creates new Projection, doesn't mutate receiver.
func (p Projection) Apply(evt Event) Projection {
	// Create new projection with incremented version
	// Note: p.sim is a value, so this copies the Simulation
	next := Projection{
		sim:     p.sim,
		version: p.version + 1,
	}

	switch e := evt.(type) {
	case SimulationCreated:
		next.sim = model.Simulation{
			ID:                 e.Header.SimID,
			SizingPolicy:       e.Config.Policy,
			Seed:               e.Config.Seed,
			SprintLength:       e.Config.SprintLength,
			Developers:         make([]model.Developer, 0),
			Backlog:            make([]model.Ticket, 0),
			ActiveTickets:      make([]model.Ticket, 0),
			CompletedTickets:   make([]model.Ticket, 0),
			CurrentSprintOption: model.NoSprint,
		}

	case Ticked:
		next.sim.CurrentTick = e.Tick

	case SprintStarted:
		next.sim.CurrentSprintOption = option.Of(model.Sprint{
			Number:       e.Number,
			StartDay:     e.StartTick,
			EndDay:       e.StartTick + next.sim.SprintLength,
			DurationDays: next.sim.SprintLength,
			BufferDays:   e.BufferDays,
			FeverStatus:  model.FeverGreen, // Start with green status
		})

	case SprintEnded:
		next.sim.CurrentSprintOption = model.NoSprint

	case DeveloperAdded:
		next.sim.Developers = append(next.sim.Developers, model.Developer{
			ID:       e.DeveloperID,
			Name:     e.Name,
			Velocity: e.Velocity,
		})

	case TicketCreated:
		next.sim.Backlog = append(next.sim.Backlog, model.Ticket{
			ID:                 e.TicketID,
			Title:              e.Title,
			EstimatedDays:      e.EstimatedDays,
			UnderstandingLevel: e.Understanding,
			Phase:              model.PhaseBacklog,
			PhaseEffortSpent:   make(map[model.WorkflowPhase]float64),
		})

	case TicketAssigned:
		// Find and move ticket from Backlog to ActiveTickets
		for i, t := range next.sim.Backlog {
			if t.ID == e.TicketID {
				// Start the ticket
				t.AssignedTo = e.DeveloperID
				t.Phase = model.PhaseResearch
				t.RemainingEffort = t.CalculatePhaseEffort(model.PhaseResearch)
				next.sim.ActiveTickets = append(next.sim.ActiveTickets, t)
				next.sim.Backlog = append(next.sim.Backlog[:i], next.sim.Backlog[i+1:]...)
				break
			}
		}
		// Update developer state
		for i, d := range next.sim.Developers {
			if d.ID == e.DeveloperID {
				next.sim.Developers[i].CurrentTicket = e.TicketID
				break
			}
		}

	case TicketCompleted:
		// Move from ActiveTickets to CompletedTickets
		for i, t := range next.sim.ActiveTickets {
			if t.ID == e.TicketID {
				t.Phase = model.PhaseDone
				t.CompletedTick = e.OccurrenceTime()
				next.sim.CompletedTickets = append(next.sim.CompletedTickets, t)
				next.sim.ActiveTickets = append(next.sim.ActiveTickets[:i], next.sim.ActiveTickets[i+1:]...)
				break
			}
		}
		// Clear developer assignment
		for i, d := range next.sim.Developers {
			if d.ID == e.DeveloperID {
				next.sim.Developers[i].CurrentTicket = ""
				break
			}
		}

	case WorkProgressed:
		for i, t := range next.sim.ActiveTickets {
			if t.ID == e.TicketID {
				next.sim.ActiveTickets[i].RemainingEffort -= e.EffortApplied
				next.sim.ActiveTickets[i].ActualDays += e.EffortApplied
				next.sim.ActiveTickets[i].PhaseEffortSpent[e.Phase] += e.EffortApplied
				break
			}
		}

	case TicketPhaseChanged:
		for i, t := range next.sim.ActiveTickets {
			if t.ID == e.TicketID {
				next.sim.ActiveTickets[i].Phase = e.NewPhase
				next.sim.ActiveTickets[i].RemainingEffort = t.CalculatePhaseEffort(e.NewPhase)
				break
			}
		}

	case PolicyChanged:
		next.sim.SizingPolicy = e.NewPolicy

	case BufferConsumed:
		// Update sprint buffer consumption
		if sprint, ok := next.sim.CurrentSprintOption.Get(); ok {
			sprint.BufferConsumed += e.DaysConsumed
			sprint = sprint.WithUpdatedFeverStatus()
			next.sim.CurrentSprintOption = option.Of(sprint)
		}

	case IncidentStarted:
		// Note: model.Incident may need to be defined or adjusted
		// For now, skip incident handling until model is verified

	case IncidentResolved:
		// Skip for now - verify model.Incident exists

	default:
		// Unknown event type - silently ignore per event sourcing convention
		// This allows forward compatibility when new events are added
	}

	return next
}

// State returns a copy of current simulation state.
func (p Projection) State() model.Simulation {
	return p.sim
}

// Version returns event count for optimistic concurrency checks.
func (p Projection) Version() int {
	return p.version
}

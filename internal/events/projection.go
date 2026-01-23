package events

import (
	"sort"
	"time"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Projection rebuilds Simulation state from events.
// Value receiver: immutable, returns new Projection with updated state.
// This is the core of event sourcing - state is derived, not stored.
type Projection struct {
	sim       model.Simulation // Value, not pointer - enables value semantics
	version   int              // Event count for optimistic concurrency
	processed []string         // Sorted slice of processed EventIDs for idempotency
}

// NewProjection creates an empty projection.
func NewProjection() Projection {
	return Projection{version: 0}
}

// Apply processes a single event, returning new Projection.
// Pure function: no side effects. Creates new Projection, doesn't mutate receiver.
// Idempotent: if event was already processed, returns unchanged projection.
func (p Projection) Apply(evt Event) Projection {
	eventID := evt.EventID()

	// Idempotency check: if already processed, return unchanged
	if eventID != "" && p.hasProcessed(eventID) {
		return p // No version bump, no state change
	}

	// Create new projection with incremented version and event ID tracked
	// Note: p.sim is a value, so this copies the Simulation
	next := Projection{
		sim:       p.sim,
		version:   p.version + 1,
		processed: p.withProcessed(eventID),
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
			OpenIncidents:      make([]model.Incident, 0),
			ResolvedIncidents:  make([]model.Incident, 0),
			CurrentSprintOption: model.NoSprint,
		}

	case Ticked:
		next.sim.CurrentTick = e.Tick

	case SprintStarted:
		next.sim.SprintNumber = e.Number
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
				t.StartedTick = e.OccurrenceTime()
				t.StartedAt = e.StartedAt
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
				next.sim.Developers[i].WIPCount++
				break
			}
		}
		// Add ticket to current sprint if one exists
		if sprint, ok := next.sim.CurrentSprintOption.Get(); ok {
			sprint = sprint.WithTicket(e.TicketID)
			next.sim.CurrentSprintOption = option.Of(sprint)
		}

	case TicketStateRestored:
		// Restore full ticket state from persistence (used by EmitLoadedState)
		// Unlike TicketAssigned, this preserves the actual Phase and RemainingEffort
		for i, t := range next.sim.Backlog {
			if t.ID == e.TicketID {
				// Restore full state
				t.AssignedTo = e.DeveloperID
				t.StartedTick = e.OccurrenceTime()
				t.StartedAt = e.StartedAt
				t.Phase = e.Phase
				t.RemainingEffort = e.RemainingEffort
				t.ActualDays = e.ActualDays
				next.sim.ActiveTickets = append(next.sim.ActiveTickets, t)
				next.sim.Backlog = append(next.sim.Backlog[:i], next.sim.Backlog[i+1:]...)
				break
			}
		}
		// Update developer state
		for i, d := range next.sim.Developers {
			if d.ID == e.DeveloperID {
				next.sim.Developers[i].CurrentTicket = e.TicketID
				next.sim.Developers[i].WIPCount++
				break
			}
		}
		// Add ticket to current sprint if one exists
		if sprint, ok := next.sim.CurrentSprintOption.Get(); ok {
			sprint = sprint.WithTicket(e.TicketID)
			next.sim.CurrentSprintOption = option.Of(sprint)
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
		// Update developer stats (matches Developer.WithCompletedTicket)
		for i, d := range next.sim.Developers {
			if d.ID == e.DeveloperID {
				next.sim.Developers[i].CurrentTicket = ""
				next.sim.Developers[i].TicketsCompleted++
				next.sim.Developers[i].TotalEffort += e.ActualDays
				next.sim.Developers[i].WIPCount--
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
		next.sim.OpenIncidents = append(next.sim.OpenIncidents, model.Incident{
			ID:       e.IncidentID,
			TicketID: e.TicketID,
			Severity: e.Severity,
		})
		// Mark the completed ticket as having caused this incident
		for i, t := range next.sim.CompletedTickets {
			if t.ID == e.TicketID {
				next.sim.CompletedTickets[i].CausedIncident = true
				next.sim.CompletedTickets[i].IncidentID = e.IncidentID
				break
			}
		}

	case IncidentResolved:
		for i, inc := range next.sim.OpenIncidents {
			if inc.ID == e.IncidentID {
				resolved := time.Time{} // Use zero time for projection purity
				inc.ResolvedAt = &resolved
				next.sim.ResolvedIncidents = append(next.sim.ResolvedIncidents, inc)
				next.sim.OpenIncidents = append(next.sim.OpenIncidents[:i], next.sim.OpenIncidents[i+1:]...)
				break
			}
		}

	case TicketDecomposed:
		// Remove parent ticket from backlog
		for i, t := range next.sim.Backlog {
			if t.ID == e.ParentTicketID {
				next.sim.Backlog = append(next.sim.Backlog[:i], next.sim.Backlog[i+1:]...)
				break
			}
		}
		// Add children to backlog
		for _, child := range e.Children {
			next.sim.Backlog = append(next.sim.Backlog, model.Ticket{
				ID:                 child.ID,
				Title:              child.Title,
				EstimatedDays:      child.EstimatedDays,
				UnderstandingLevel: child.Understanding,
				Phase:              model.PhaseBacklog,
				PhaseEffortSpent:   make(map[model.WorkflowPhase]float64),
			})
		}

	case SprintWIPUpdated:
		// Update sprint WIP metrics
		if sprint, ok := next.sim.CurrentSprintOption.Get(); ok {
			if e.CurrentWIP > sprint.MaxWIP {
				sprint.MaxWIP = e.CurrentWIP
			}
			sprint.WIPSum += e.CurrentWIP
			sprint.WIPTicks++
			next.sim.CurrentSprintOption = option.Of(sprint)
		}

	case BugDiscovered:
		// Add rework effort to the active ticket
		for i, t := range next.sim.ActiveTickets {
			if t.ID == e.TicketID {
				next.sim.ActiveTickets[i].RemainingEffort += e.ReworkEffort
				break
			}
		}

	case ScopeCreepOccurred:
		// Add effort and estimate to the active ticket
		for i, t := range next.sim.ActiveTickets {
			if t.ID == e.TicketID {
				next.sim.ActiveTickets[i].RemainingEffort += e.EffortAdded
				next.sim.ActiveTickets[i].EstimatedDays += e.EstimateAdded
				break
			}
		}

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

// hasProcessed checks if an event ID has already been processed.
// Uses binary search for O(log n) lookup.
func (p Projection) hasProcessed(id string) bool {
	i := sort.SearchStrings(p.processed, id)
	return i < len(p.processed) && p.processed[i] == id
}

// withProcessed returns a new sorted slice with the event ID added.
// Maintains sort order for O(log n) lookup.
func (p Projection) withProcessed(id string) []string {
	if id == "" {
		// Don't track empty IDs
		return p.processed
	}
	i := sort.SearchStrings(p.processed, id)
	result := make([]string, len(p.processed)+1)
	copy(result[:i], p.processed[:i])
	result[i] = id
	copy(result[i+1:], p.processed[i:])
	return result
}

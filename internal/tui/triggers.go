package tui

import (
	"github.com/binaryphile/sofdevsim-2026/internal/lessons"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// TriggerProjection computes trigger state from simulation events.
// Read model pattern: query handlers read from optimized views, not aggregates.
// Data: SimulationState → TriggerState
//
// Idempotency: All projection methods are idempotent—calling ProjectFromSimulation
// with the same Simulation always produces the same TriggerProjection. The projection
// has no side effects and maintains no internal state between calls. Per CQRS Guide §15,
// idempotent projections enable safe replay and retry semantics.
//
// Scope: Currently projects SprintCount only (for UC22 FiveFocusing trigger).
// Future phases may extend to project buffer status, queue depths from events.
// This limited scope is intentional per CQRS Guide §8 - projections should be
// fit-for-purpose read models, not general-purpose state caches.
type TriggerProjection struct {
	sprintCount int
}

// ProjectFromSimulation builds the projection from current simulation state.
// Calculation: model.Simulation → TriggerProjection
func ProjectFromSimulation(sim model.Simulation) TriggerProjection {
	return TriggerProjection{
		sprintCount: sim.SprintNumber,
	}
}

// ToTriggerState converts projection to TriggerState for lesson selection.
// Calculation: TriggerProjection → TriggerState (partial)
// Note: Returns partial TriggerState; caller must fill event-based triggers.
func (p TriggerProjection) ToTriggerState() TriggerState {
	return TriggerState{
		SprintCount: p.sprintCount,
	}
}

// MergeEventTriggers combines projection state with event-based triggers.
// Calculation: (TriggerState, bool, bool, bool) → TriggerState
// The projection provides SprintCount; event detectors provide UC19/20/21 triggers.
func (p TriggerProjection) MergeEventTriggers(hasRedBufferWithLowTicket, hasQueueImbalance, hasHighChildVariance bool) TriggerState {
	return TriggerState{
		SprintCount:               p.sprintCount,
		HasRedBufferWithLowTicket: hasRedBufferWithLowTicket,
		HasQueueImbalance:         hasQueueImbalance,
		HasHighChildVariance:      hasHighChildVariance,
	}
}

// BuildTriggerStateFromEngine builds TriggerState for engine mode using CQRS projection.
// Calculation: (model.Simulation, FeverStatus, []model.Ticket, []model.Ticket) → TriggerState
// Separates projection (SprintCount) from event detection (UC19/20/21).
//
// Note on event sourcing: Per CQRS Guide §3, "CQRS should only be used on specific
// portions of a system." This function implements the read model (projection) portion
// of CQRS without full event sourcing. The projection reads current aggregate state
// rather than replaying events. This is appropriate for lesson triggers which need
// current state, not historical reconstruction. Full event sourcing exists in
// internal/events/ for simulation state; this projection bridges to that model.
func BuildTriggerStateFromEngine(sim model.Simulation, feverStatus model.FeverStatus, activeTickets, completedTickets []model.Ticket) TriggerState {
	// Build projection from simulation
	projection := ProjectFromSimulation(sim)

	// Detect event-based triggers
	hasRedBufferWithLowTicket := lessons.HasRedBufferWithLowTicket(feverStatus, activeTickets)
	hasQueueImbalance := lessons.HasQueueImbalance(activeTickets)
	hasHighChildVariance := lessons.HasHighChildVariance(completedTickets)

	// Merge projection state with event triggers
	return projection.MergeEventTriggers(hasRedBufferWithLowTicket, hasQueueImbalance, hasHighChildVariance)
}

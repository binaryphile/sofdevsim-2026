package model

import (
	"log/slog"
	"time"
)

// Ticket represents a unit of work progressing through the 8-phase workflow
type Ticket struct {
	ID          string
	Title       string
	Description string

	// Sizing discriminants (the tension we're testing)
	EstimatedDays      float64            // DORA's discriminant
	UnderstandingLevel UnderstandingLevel // TameFlow's discriminant
	Type               TicketType         // UC37: shape of work; drives per-phase effort distribution

	// Realization
	ActualDays      float64
	RemainingEffort float64

	// Workflow
	Phase            WorkflowPhase
	PhaseEffortSpent map[WorkflowPhase]float64 // Track effort per phase

	// Timestamps (for DORA metrics)
	CreatedAt   time.Time
	StartedAt   time.Time // First commit proxy
	CompletedAt time.Time // Deployed proxy

	// Simulation ticks (for internal calculations)
	StartedTick   int
	CompletedTick int

	// Decomposition
	ParentID string
	ChildIDs []string

	// Assignment
	AssignedTo   string
	Priority     Priority     // business urgency (exogenous input)
	IntakeStatus IntakeStatus // intake pipeline progress

	// Phase visit tracking (for handoff model)
	PhaseEnteredTick  int    // tick when current phase was entered (queued)
	PhaseAssignedTick int    // tick when dev was assigned for current phase
	Contributors []string // dev IDs who contributed to this ticket (for review disqualification)

	// Failure tracking (for CFR/MTTR)
	CausedIncident bool
	IncidentID     string
}

// NewTicket creates a ticket with initialized maps
func NewTicket(id, title string, estimatedDays float64, understanding UnderstandingLevel) Ticket {
	return Ticket{
		ID:                 id,
		Title:              title,
		EstimatedDays:      estimatedDays,
		UnderstandingLevel: understanding,
		Phase:              PhaseBacklog,
		PhaseEffortSpent:   make(map[WorkflowPhase]float64),
		CreatedAt:          time.Now(),
		Priority:           PriorityNormal,
		IntakeStatus:       IntakeTriaged,
	}
}

// NewSubmittedTicket creates a ticket that needs triage before sprint commitment.
func NewSubmittedTicket(id, title string, estimatedDays float64, understanding UnderstandingLevel, priority Priority) Ticket {
	return Ticket{
		ID:                 id,
		Title:              title,
		EstimatedDays:      estimatedDays,
		UnderstandingLevel: understanding,
		Phase:              PhaseBacklog,
		PhaseEffortSpent:   make(map[WorkflowPhase]float64),
		CreatedAt:          time.Now(),
		Priority:           priority,
		IntakeStatus:       IntakeSubmitted,
	}
}

// IsAssigned returns true if the ticket has a developer assigned
func (t Ticket) IsAssigned() bool {
	return t.AssignedTo != ""
}

// IsActive returns true if the ticket is being worked on
func (t Ticket) IsActive() bool {
	return t.Phase != PhaseBacklog && t.Phase != PhaseDone
}

// IsComplete returns true if the ticket is done
func (t Ticket) IsComplete() bool {
	return t.Phase == PhaseDone
}

// HasChildren returns true if this ticket was decomposed
func (t Ticket) HasChildren() bool {
	return len(t.ChildIDs) > 0
}

// IsChild returns true if this ticket was created from decomposition
func (t Ticket) IsChild() bool {
	return t.ParentID != ""
}

// GetEstimatedDays returns the estimated days for FluentFP ToFloat64 operations.
func (t Ticket) GetEstimatedDays() float64 { return t.EstimatedDays }

// TypePhaseEffortPct defines per-type phase-effort distributions. Each row sums to 1.0.
// Done is implicit 0 across all types. Per UC37 (see docs/design.md §"Heterogeneous
// Ticket Types"); per-type rationale documented there.
//
// IMMUTABLE BY CONVENTION (/c absorption per FP unified guide §3 Data layer + Go dev
// guide §3 value semantics): do NOT mutate this map at runtime. It is exposed for
// read-only access via CalculatePhaseEffort and for test introspection. The
// PhaseEffortPct backward-compat alias below is a deep-copy snapshot so accidental
// mutation through that alias does not leak into this canonical table.
var TypePhaseEffortPct = map[TicketType]map[WorkflowPhase]float64{
	TicketTypeFeature: {
		PhaseResearch:  0.05,
		PhaseSizing:    0.02,
		PhasePlanning:  0.03,
		PhaseImplement: 0.55,
		PhaseVerify:    0.20,
		PhaseCICD:      0.05,
		PhaseReview:    0.10,
	},
	TicketTypeBug: {
		PhaseResearch:  0.10,
		PhaseSizing:    0.05,
		PhasePlanning:  0.00, // bugs bypass Planning (root cause already known)
		PhaseImplement: 0.35,
		PhaseVerify:    0.30, // heavier — regression coverage dominates
		PhaseCICD:      0.05,
		PhaseReview:    0.15,
	},
	TicketTypeSpike: {
		PhaseResearch:  0.55, // research-heavy (the whole point)
		PhaseSizing:    0.05,
		PhasePlanning:  0.05,
		PhaseImplement: 0.25, // throwaway prototype
		PhaseVerify:    0.05,
		PhaseCICD:      0.00, // spikes don't deploy
		PhaseReview:    0.05,
	},
	TicketTypeMigration: {
		PhaseResearch:  0.05,
		PhaseSizing:    0.05,
		PhasePlanning:  0.10, // cross-system coordination bumps Planning
		PhaseImplement: 0.30,
		PhaseVerify:    0.35, // lingers in Verify — data-shape correctness
		PhaseCICD:      0.05,
		PhaseReview:    0.10,
	},
	TicketTypeInfra: {
		PhaseResearch:  0.10,
		PhaseSizing:    0.02,
		PhasePlanning:  0.08, // rollout sequencing matters
		PhaseImplement: 0.30,
		PhaseVerify:    0.20,
		PhaseCICD:      0.20, // heavy CI/CD — infra changes are pipeline-shaped
		PhaseReview:    0.10,
	},
}

// PhaseEffortPct is a backward-compat alias pointing to the Feature distribution.
// Preserved so downstream consumers (CSV export's PhaseEffortDistribution map; any
// test reading the global directly) work unchanged after UC37.
//
// Deep-copied at init (/c absorption per FP unified guide §3 ACD): the alias is a
// SNAPSHOT of TypePhaseEffortPct[TicketTypeFeature], not a pointer to the same
// underlying map. Mutating PhaseEffortPct does NOT leak into TypePhaseEffortPct.
// IMMUTABLE BY CONVENTION same as the canonical table above.
var PhaseEffortPct = func() map[WorkflowPhase]float64 {
	src := TypePhaseEffortPct[TicketTypeFeature]
	dst := make(map[WorkflowPhase]float64, len(src))
	for k, v := range src { // justified:MB (map building — deep-copy snapshot)
		dst[k] = v
	}
	return dst
}()

// UnderstandingPhaseMultiplier adjusts phase effort based on understanding
var UnderstandingPhaseMultiplier = map[UnderstandingLevel]map[WorkflowPhase]float64{
	LowUnderstanding: {
		PhaseResearch:  3.0, // Much more research needed
		PhaseImplement: 1.5, // More false starts
		PhaseVerify:    1.3, // More edge cases discovered
	},
	MediumUnderstanding: {
		PhaseResearch:  1.5,
		PhaseImplement: 1.1,
		PhaseVerify:    1.1,
	},
	HighUnderstanding: {
		PhaseResearch:  0.5, // Quick confirmation
		PhaseImplement: 0.9, // Efficient execution
		PhaseVerify:    0.9,
	},
}

// CalculatePhaseEffort returns the effort required for a specific phase, looking up
// the per-type distribution then applying the understanding-level multiplier on
// Research/Implement/Verify. Defensive fallback: unrecognised Type values fall back
// to the Feature distribution rather than silently zero-ing phase effort.
func (t Ticket) CalculatePhaseEffort(phase WorkflowPhase) float64 {
	typeDist, ok := TypePhaseEffortPct[t.Type]
	if !ok {
		// Defensive fallback (UC37 plan §commit 5): unrecognised Type → Feature distribution.
		// Protects against external-source corruption where Type could be out-of-range.
		// /c absorption (Go dev guide §8 + N5 audit): log the fallback so silent corruption
		// is observable. Use slog.Warn to surface the bug without breaking the simulation.
		slog.Warn("ticket has unrecognised Type; falling back to Feature distribution",
			"ticket_id", t.ID, "type_int", int(t.Type))
		typeDist = TypePhaseEffortPct[TicketTypeFeature]
	}

	basePct, ok := typeDist[phase]
	if !ok {
		return 0
	}

	effort := t.EstimatedDays * basePct

	// Apply understanding multiplier if applicable
	if multipliers, ok := UnderstandingPhaseMultiplier[t.UnderstandingLevel]; ok {
		if mult, ok := multipliers[phase]; ok {
			effort *= mult
		}
	}

	return effort
}

// TotalPhaseEffort returns the sum of effort spent across all phases
func (t Ticket) TotalPhaseEffort() float64 {
	var total float64
	for _, effort := range t.PhaseEffortSpent {
		total += effort
	}
	return total
}

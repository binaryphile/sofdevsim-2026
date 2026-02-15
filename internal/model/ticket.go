package model

import "time"

// Ticket represents a unit of work progressing through the 8-phase workflow
type Ticket struct {
	ID          string
	Title       string
	Description string

	// Sizing discriminants (the tension we're testing)
	EstimatedDays      float64            // DORA's discriminant
	UnderstandingLevel UnderstandingLevel // TameFlow's discriminant

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
	AssignedTo string

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

// PhaseEffortPct defines how effort is distributed across phases (sums to 1.0)
var PhaseEffortPct = map[WorkflowPhase]float64{
	PhaseResearch:  0.05, // 5% - quick for understood work, longer for unknown
	PhaseSizing:    0.02, // 2% - estimation overhead
	PhasePlanning:  0.03, // 3% - planning overhead
	PhaseImplement: 0.55, // 55% - bulk of work
	PhaseVerify:    0.20, // 20% - testing
	PhaseCICD:      0.05, // 5% - CI/CD pipeline time
	PhaseReview:    0.10, // 10% - code review
}

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

// CalculatePhaseEffort returns the effort required for a specific phase
func (t Ticket) CalculatePhaseEffort(phase WorkflowPhase) float64 {
	basePct, ok := PhaseEffortPct[phase]
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

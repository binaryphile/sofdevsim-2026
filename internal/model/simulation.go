package model

import (
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
)

// NoSprint is the zero value for option.Option[Sprint], representing no active sprint.
var NoSprint option.Option[Sprint]

// Mentorship represents an active mentor-mentee pairing on a ticket phase.
type Mentorship struct {
	MentorID string
	MenteeID string
	TicketID string
	Phase    WorkflowPhase
}

// Simulation holds the complete state of a simulation run
type Simulation struct {
	ID                   string // Unique identifier for event sourcing
	CurrentTick          int    // 1 tick = 1 day
	CurrentSprintOption  option.Option[Sprint]
	SprintNumber         int

	// Team
	Developers []Developer

	// Work
	Backlog          []Ticket
	ActiveTickets    []Ticket
	CompletedTickets []Ticket
	CommittedTickets []Ticket // tickets committed to current sprint, not yet in phase queues

	// Incidents
	OpenIncidents     []Incident
	ResolvedIncidents []Incident

	// Mentoring
	ActiveMentorships []Mentorship

	// Phase queues (handoff model)
	PhaseQueues map[WorkflowPhase][]string // ticket IDs waiting per phase
	CICDSlots   int                        // max concurrent CI/CD pipeline runs
	CICDInUse   int                        // current pipeline runs

	// Configuration
	SizingPolicy     SizingPolicy
	SprintLength     int     // days
	BufferPct        float64
	ExperienceConfig ExperienceConfig

	// RNG seed for reproducibility
	Seed int64
}

// ExperienceConfig holds tunable parameters for experience and sprint planning.
// Zero value uses defaults via DefaultExperienceConfig().
type ExperienceConfig struct {
	HighMultiplier       float64 // velocity multiplier for High experience (default 1.1)
	MediumMultiplier     float64 // velocity multiplier for Medium experience (default 1.0)
	LowMultiplier        float64 // velocity multiplier for Low experience alone (default 0.6)
	LowMentoredMultiplier float64 // velocity multiplier for Low with mentor (default 0.9)
	LowMaxEstimate       float64 // max ticket size for Low-experience devs (default 5.0)
	SprintCapacityFactor float64 // overhead factor for sprint commitment (default 0.8)
}

// DefaultExperienceConfig returns the standard configuration.
func DefaultExperienceConfig() ExperienceConfig {
	return ExperienceConfig{
		HighMultiplier:        1.1,
		MediumMultiplier:      1.0,
		LowMultiplier:         0.6,
		LowMentoredMultiplier: 0.9,
		LowMaxEstimate:        5.0,
		SprintCapacityFactor:  0.8,
	}
}

// ExperienceMultiplier returns the velocity multiplier for an experience level.
// Uses option.NonZero to fall back to defaults for zero-value configs.
func (c ExperienceConfig) ExperienceMultiplier(level ExperienceLevel, mentored bool) float64 {
	switch level {
	case ExperienceHigh:
		return option.NonZero(c.HighMultiplier).Or(1.1)
	case ExperienceLow:
		if mentored {
			return option.NonZero(c.LowMentoredMultiplier).Or(0.9)
		}
		return option.NonZero(c.LowMultiplier).Or(0.6)
	default:
		return option.NonZero(c.MediumMultiplier).Or(1.0)
	}
}

// NewSimulation creates a simulation with default configuration.
// Returns value type for consistency with Simulation's value semantics.
func NewSimulation(id string, policy SizingPolicy, seed int64) Simulation {
	return Simulation{
		ID:                id,
		CurrentTick:       0,
		SprintNumber:      0,
		Developers:        make([]Developer, 0),
		Backlog:           make([]Ticket, 0),
		ActiveTickets:     make([]Ticket, 0),
		CompletedTickets:  make([]Ticket, 0),
		CommittedTickets:  make([]Ticket, 0),
		OpenIncidents:     make([]Incident, 0),
		ResolvedIncidents: make([]Incident, 0),
		PhaseQueues:       make(map[WorkflowPhase][]string),
		CICDSlots:         2, // default 2 CI/CD pipeline slots
		SizingPolicy:      policy,
		SprintLength:      10, // 2-week sprints
		BufferPct:         0.2,
		ExperienceConfig:  DefaultExperienceConfig(),
		Seed:              seed,
	}
}

// Clone returns a deep copy with independent slice backing arrays.
// Required because value copies of Simulation share underlying slice arrays,
// which causes corruption when Projection.Apply mutates slices via append.
func (s Simulation) Clone() Simulation {
	s.Developers = append([]Developer(nil), s.Developers...)
	s.Backlog = append([]Ticket(nil), s.Backlog...)
	s.ActiveTickets = append([]Ticket(nil), s.ActiveTickets...)
	s.CompletedTickets = append([]Ticket(nil), s.CompletedTickets...)
	s.CommittedTickets = append([]Ticket(nil), s.CommittedTickets...)
	s.OpenIncidents = append([]Incident(nil), s.OpenIncidents...)
	s.ResolvedIncidents = append([]Incident(nil), s.ResolvedIncidents...)

	s.ActiveMentorships = append([]Mentorship(nil), s.ActiveMentorships...)

	// Deep copy PhaseQueues map and its slice values
	if s.PhaseQueues != nil {
		cloned := make(map[WorkflowPhase][]string, len(s.PhaseQueues))
		for k, v := range s.PhaseQueues {
			cloned[k] = append([]string(nil), v...)
		}
		s.PhaseQueues = cloned
	}

	return s
}

// isMentor returns true if the mentorship's mentor matches the given devID.
func isMentor(devID string) func(Mentorship) bool {
	return func(m Mentorship) bool { return m.MentorID == devID }
}

// IsMentoring returns true if the developer is currently mentoring someone.
func (s Simulation) IsMentoring(devID string) bool {
	return slice.From(s.ActiveMentorships).Any(isMentor(devID))
}

// IsDevAvailable returns true if the developer is idle and not mentoring.
func (s Simulation) IsDevAvailable(devID string) bool {
	devIdx := s.FindDeveloperIndex(devID)
	if devIdx == -1 {
		return false
	}
	return s.Developers[devIdx].IsIdle() && !s.IsMentoring(devID)
}

// MentorForTicket returns the mentor ID for a ticket in a specific phase, if any.
func (s Simulation) MentorForTicket(ticketID string, phase WorkflowPhase) (string, bool) {
	// matchesTicketPhase returns true if the mentorship matches the given ticket and phase.
	matchesTicketPhase := func(m Mentorship) bool { return m.TicketID == ticketID && m.Phase == phase }
	if m, ok := slice.From(s.ActiveMentorships).Find(matchesTicketPhase).Get(); ok {
		return m.MentorID, true
	}
	return "", false
}

// FindCommittedTicketIndex returns index of ticket in CommittedTickets, or -1 if not found
func (s Simulation) FindCommittedTicketIndex(id string) int {
	for i := range s.CommittedTickets { // justified:IX
		if s.CommittedTickets[i].ID == id {
			return i
		}
	}
	return -1
}

// FindActiveTicketIndex returns index of ticket in ActiveTickets, or -1 if not found
func (s Simulation) FindActiveTicketIndex(id string) int {
	for i := range s.ActiveTickets { // justified:IX
		if s.ActiveTickets[i].ID == id {
			return i
		}
	}
	return -1
}

// FindBacklogTicketIndex returns index of ticket in Backlog, or -1 if not found
func (s Simulation) FindBacklogTicketIndex(id string) int {
	for i := range s.Backlog { // justified:IX
		if s.Backlog[i].ID == id {
			return i
		}
	}
	return -1
}

// FindDeveloperIndex returns index of developer, or -1 if not found
func (s Simulation) FindDeveloperIndex(id string) int {
	for i := range s.Developers { // justified:IX
		if s.Developers[i].ID == id {
			return i
		}
	}
	return -1
}

// IdleDevelopers returns developers without assigned tickets
func (s Simulation) IdleDevelopers() []Developer {
	return slice.From(s.Developers).KeepIf(Developer.IsIdle)
}

// TotalOpenIncidents returns count of unresolved incidents
func (s Simulation) TotalOpenIncidents() int {
	return slice.From(s.OpenIncidents).
		KeepIf(Incident.IsOpen).
		Len()
}

// TotalIncidents returns count of all incidents
func (s Simulation) TotalIncidents() int {
	return len(s.OpenIncidents) + len(s.ResolvedIncidents)
}

// TotalDeploys returns count of completed tickets
func (s Simulation) TotalDeploys() int {
	return len(s.CompletedTickets)
}

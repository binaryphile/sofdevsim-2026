package model

import (
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
)

// NoSprint is the zero value for option.Basic[Sprint], representing no active sprint.
var NoSprint option.Basic[Sprint]

// Simulation holds the complete state of a simulation run
type Simulation struct {
	ID                   string // Unique identifier for event sourcing
	CurrentTick          int    // 1 tick = 1 day
	CurrentSprintOption  option.Basic[Sprint]
	SprintNumber         int

	// Team
	Developers []Developer

	// Work
	Backlog          []Ticket
	ActiveTickets    []Ticket
	CompletedTickets []Ticket

	// Incidents
	OpenIncidents     []Incident
	ResolvedIncidents []Incident

	// Configuration
	SizingPolicy SizingPolicy
	SprintLength int // days
	BufferPct    float64

	// RNG seed for reproducibility
	Seed int64
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
		OpenIncidents:     make([]Incident, 0),
		ResolvedIncidents: make([]Incident, 0),
		SizingPolicy:      policy,
		SprintLength:      10, // 2-week sprints
		BufferPct:         0.2,
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
	s.OpenIncidents = append([]Incident(nil), s.OpenIncidents...)
	s.ResolvedIncidents = append([]Incident(nil), s.ResolvedIncidents...)
	return s
}

// FindActiveTicketIndex returns index of ticket in ActiveTickets, or -1 if not found
func (s Simulation) FindActiveTicketIndex(id string) int {
	for i := range s.ActiveTickets {
		if s.ActiveTickets[i].ID == id {
			return i
		}
	}
	return -1
}

// FindBacklogTicketIndex returns index of ticket in Backlog, or -1 if not found
func (s Simulation) FindBacklogTicketIndex(id string) int {
	for i := range s.Backlog {
		if s.Backlog[i].ID == id {
			return i
		}
	}
	return -1
}

// FindDeveloperIndex returns index of developer, or -1 if not found
func (s Simulation) FindDeveloperIndex(id string) int {
	for i := range s.Developers {
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

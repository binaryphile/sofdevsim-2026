package model

import (
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
)

// NoSprint is the zero value for option.Basic[Sprint], representing no active sprint.
var NoSprint option.Basic[Sprint]

// Simulation holds the complete state of a simulation run
type Simulation struct {
	CurrentTick          int // 1 tick = 1 day
	CurrentSprintOption  option.Basic[Sprint]
	SprintNumber  int

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

// NewSimulation creates a simulation with default configuration
func NewSimulation(policy SizingPolicy, seed int64) *Simulation {
	return &Simulation{
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

// AddDeveloper adds a developer to the team
func (s *Simulation) AddDeveloper(dev Developer) {
	s.Developers = append(s.Developers, dev)
}

// AddTicket adds a ticket to the backlog
func (s *Simulation) AddTicket(ticket Ticket) {
	s.Backlog = append(s.Backlog, ticket)
}

// StartSprint begins a new sprint
func (s *Simulation) StartSprint() {
	s.SprintNumber++
	sprint := NewSprint(s.SprintNumber, s.CurrentTick, s.SprintLength, s.BufferPct)
	s.CurrentSprintOption = option.Of(sprint)
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

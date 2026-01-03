package model

import "github.com/binaryphile/fluentfp/slice"

// Simulation holds the complete state of a simulation run
type Simulation struct {
	CurrentTick   int // 1 tick = 1 day
	CurrentSprint *Sprint
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
	s.CurrentSprint = &sprint
}

// FindTicketByID searches all ticket collections for a ticket
func (s *Simulation) FindTicketByID(id string) *Ticket {
	for i := range s.ActiveTickets {
		if s.ActiveTickets[i].ID == id {
			return &s.ActiveTickets[i]
		}
	}
	for i := range s.Backlog {
		if s.Backlog[i].ID == id {
			return &s.Backlog[i]
		}
	}
	for i := range s.CompletedTickets {
		if s.CompletedTickets[i].ID == id {
			return &s.CompletedTickets[i]
		}
	}
	return nil
}

// FindDeveloperByID finds a developer by ID
func (s *Simulation) FindDeveloperByID(id string) *Developer {
	for i := range s.Developers {
		if s.Developers[i].ID == id {
			return &s.Developers[i]
		}
	}
	return nil
}

// IdleDevelopers returns developers without assigned tickets
func (s *Simulation) IdleDevelopers() []Developer {
	return slice.From(s.Developers).KeepIf(func(d Developer) bool { return d.IsIdle() })
}

// TotalOpenIncidents returns count of unresolved incidents
func (s *Simulation) TotalOpenIncidents() int {
	return slice.From(s.OpenIncidents).KeepIf(func(i Incident) bool { return i.IsOpen() }).Len()
}

// TotalIncidents returns count of all incidents
func (s *Simulation) TotalIncidents() int {
	return len(s.OpenIncidents) + len(s.ResolvedIncidents)
}

// TotalDeploys returns count of completed tickets
func (s *Simulation) TotalDeploys() int {
	return len(s.CompletedTickets)
}

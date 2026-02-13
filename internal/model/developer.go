package model

// Developer represents a team member who works on tickets
type Developer struct {
	ID            string
	Name          string
	Velocity      float64 // Base throughput (effort/day)
	CurrentTicket string  // Currently assigned ticket ID
	WIPCount      int

	// Stats
	TicketsCompleted int
	TotalEffort      float64
}

// NewDeveloper creates a developer with sensible defaults
func NewDeveloper(id, name string, velocity float64) Developer {
	return Developer{
		ID:       id,
		Name:     name,
		Velocity: velocity,
	}
}

// IsIdle returns true if the developer has no assigned ticket
func (d Developer) IsIdle() bool {
	return d.CurrentTicket == ""
}

// GetID returns the developer ID (accessor for FluentFP).
func (d Developer) GetID() string { return d.ID }

// GetName returns the developer name (accessor for FluentFP).
func (d Developer) GetName() string { return d.Name }

// WithTicket returns a developer assigned to the given ticket
func (d Developer) WithTicket(ticketID string) Developer {
	d.CurrentTicket = ticketID
	d.WIPCount++
	return d
}

// WithoutTicket returns a developer with no ticket assignment
func (d Developer) WithoutTicket() Developer {
	d.CurrentTicket = ""
	return d
}

// WithCompletedTicket returns a developer with updated completion stats
func (d Developer) WithCompletedTicket(effort float64) Developer {
	d.TicketsCompleted++
	d.TotalEffort += effort
	d.WIPCount--
	d.CurrentTicket = ""
	return d
}

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
func (d *Developer) IsIdle() bool {
	return d.CurrentTicket == ""
}

// Assign assigns a ticket to this developer
func (d *Developer) Assign(ticketID string) {
	d.CurrentTicket = ticketID
	d.WIPCount++
}

// Unassign clears the current ticket assignment
func (d *Developer) Unassign() {
	d.CurrentTicket = ""
}

// CompleteTicket records completion and clears assignment
func (d *Developer) CompleteTicket(effort float64) {
	d.TicketsCompleted++
	d.TotalEffort += effort
	d.WIPCount--
	d.CurrentTicket = ""
}

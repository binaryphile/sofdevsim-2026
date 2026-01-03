package model

import "time"

// Incident represents a production issue caused by a deployed ticket
// Used for calculating MTTR and Change Fail Rate
type Incident struct {
	ID         string
	TicketID   string     // Ticket that caused it
	CreatedAt  time.Time  // When detected
	ResolvedAt *time.Time // When fixed (nil if open)
	Severity   Severity
}

// NewIncident creates an open incident
func NewIncident(id, ticketID string, severity Severity) Incident {
	return Incident{
		ID:        id,
		TicketID:  ticketID,
		CreatedAt: time.Now(),
		Severity:  severity,
	}
}

// IsOpen returns true if the incident hasn't been resolved
func (i *Incident) IsOpen() bool {
	return i.ResolvedAt == nil
}

// Resolve marks the incident as resolved
func (i *Incident) Resolve() {
	now := time.Now()
	i.ResolvedAt = &now
}

// TimeToResolve returns the duration from creation to resolution
// Returns 0 if still open
func (i *Incident) TimeToResolve() time.Duration {
	if i.ResolvedAt == nil {
		return 0
	}
	return i.ResolvedAt.Sub(i.CreatedAt)
}

// DaysToResolve returns TimeToResolve in days
func (i *Incident) DaysToResolve() float64 {
	return i.TimeToResolve().Hours() / 24
}

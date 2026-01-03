package model

// Sprint represents a time-boxed iteration
type Sprint struct {
	ID           string
	Number       int
	StartDay     int
	EndDay       int
	DurationDays int

	// TameFlow buffer
	BufferDays     float64
	BufferConsumed float64
	FeverStatus    FeverStatus

	// Tickets committed to sprint
	Tickets []string // Ticket IDs
}

// NewSprint creates a sprint with buffer calculated as percentage of duration
func NewSprint(number, startDay, durationDays int, bufferPct float64) Sprint {
	return Sprint{
		ID:           sprintID(number),
		Number:       number,
		StartDay:     startDay,
		EndDay:       startDay + durationDays,
		DurationDays: durationDays,
		BufferDays:   float64(durationDays) * bufferPct,
		FeverStatus:  FeverGreen,
		Tickets:      make([]string, 0),
	}
}

func sprintID(number int) string {
	return "SPR-" + string(rune('0'+number))
}

// DaysRemaining returns days left in the sprint
func (s *Sprint) DaysRemaining(currentDay int) int {
	remaining := s.EndDay - currentDay
	if remaining < 0 {
		return 0
	}
	return remaining
}

// DaysElapsed returns days completed in the sprint
func (s *Sprint) DaysElapsed(currentDay int) int {
	elapsed := currentDay - s.StartDay
	if elapsed < 0 {
		return 0
	}
	if elapsed > s.DurationDays {
		return s.DurationDays
	}
	return elapsed
}

// ProgressPct returns the sprint progress as a percentage
func (s *Sprint) ProgressPct(currentDay int) float64 {
	if s.DurationDays == 0 {
		return 0
	}
	return float64(s.DaysElapsed(currentDay)) / float64(s.DurationDays)
}

// BufferRemaining returns unused buffer days
func (s *Sprint) BufferRemaining() float64 {
	remaining := s.BufferDays - s.BufferConsumed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// BufferPctUsed returns buffer consumption as percentage
func (s *Sprint) BufferPctUsed() float64 {
	if s.BufferDays == 0 {
		return 0
	}
	return s.BufferConsumed / s.BufferDays
}

// UpdateFeverStatus recalculates fever status based on buffer consumption
func (s *Sprint) UpdateFeverStatus() {
	pctUsed := s.BufferPctUsed()
	switch {
	case pctUsed < 0.33:
		s.FeverStatus = FeverGreen
	case pctUsed < 0.66:
		s.FeverStatus = FeverYellow
	default:
		s.FeverStatus = FeverRed
	}
}

// ConsumeBuffer adds to buffer consumption and updates fever status
func (s *Sprint) ConsumeBuffer(days float64) {
	s.BufferConsumed += days
	s.UpdateFeverStatus()
}

// AddTicket adds a ticket to the sprint
func (s *Sprint) AddTicket(ticketID string) {
	s.Tickets = append(s.Tickets, ticketID)
}

// TicketCount returns the number of tickets in the sprint
func (s *Sprint) TicketCount() int {
	return len(s.Tickets)
}

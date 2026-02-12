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
	Progress       float64 // Work progress (0.0-1.0) when fever status was last updated

	// Tickets committed to sprint
	Tickets []string // Ticket IDs

	// WIP tracking for export
	MaxWIP   int // Maximum WIP observed during sprint
	WIPSum   int // Accumulator for average WIP
	WIPTicks int // Count of ticks for average
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
func (s Sprint) DaysRemaining(currentDay int) int {
	remaining := s.EndDay - currentDay
	if remaining < 0 {
		return 0
	}
	return remaining
}

// DaysElapsed returns days completed in the sprint
func (s Sprint) DaysElapsed(currentDay int) int {
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
func (s Sprint) ProgressPct(currentDay int) float64 {
	if s.DurationDays == 0 {
		return 0
	}
	return float64(s.DaysElapsed(currentDay)) / float64(s.DurationDays)
}

// BufferRemaining returns unused buffer days
func (s Sprint) BufferRemaining() float64 {
	remaining := s.BufferDays - s.BufferConsumed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// BufferPctUsed returns buffer consumption as percentage
func (s Sprint) BufferPctUsed() float64 {
	if s.BufferDays == 0 {
		return 0
	}
	return s.BufferConsumed / s.BufferDays
}

// WithUpdatedFeverStatus returns sprint with fever status recalculated using
// TameFlow diagonal thresholds. Zone boundaries compare buffer consumption
// to work progress:
//   - Green: bufferPct <= progress × 0.66 (ahead of schedule)
//   - Red: bufferPct >= 0.33 + progress × 0.67 (behind schedule)
//   - Yellow: between thresholds (on track)
func (s Sprint) WithUpdatedFeverStatus(progress float64) Sprint {
	s.Progress = progress
	bufferPct := s.BufferPctUsed()
	greenThreshold := progress * 0.66
	redThreshold := 0.33 + progress*0.67

	switch {
	case bufferPct <= greenThreshold:
		s.FeverStatus = FeverGreen
	case bufferPct > redThreshold:
		s.FeverStatus = FeverRed
	default:
		s.FeverStatus = FeverYellow
	}
	return s
}

// WithConsumedBuffer returns sprint with buffer consumed and fever updated
func (s Sprint) WithConsumedBuffer(days, progress float64) Sprint {
	s.BufferConsumed += days
	return s.WithUpdatedFeverStatus(progress)
}

// WithTicket returns sprint with ticket added
func (s Sprint) WithTicket(ticketID string) Sprint {
	s.Tickets = append(s.Tickets, ticketID)
	return s
}

// TicketCount returns the number of tickets in the sprint
func (s Sprint) TicketCount() int {
	return len(s.Tickets)
}

// AvgWIP returns the average WIP over the sprint
func (s Sprint) AvgWIP() float64 {
	if s.WIPTicks == 0 {
		return 0
	}
	return float64(s.WIPSum) / float64(s.WIPTicks)
}

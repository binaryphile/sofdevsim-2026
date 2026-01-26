// Package lessons trigger detection functions.
//
// UC19 Trigger: HasRedBufferWithLowTicket
//
//	Fires when: Buffer consumed >= 66% AND any active ticket has LOW understanding
//	Purpose: "Aha moment" lesson - reveals understanding as the constraint
//	Once: Only shows once per session (checked in Select via SeenMap)
//
// Two implementations exist to avoid import cycles:
//   - HasRedBufferWithLowTicket: Uses model types (engine mode)
//   - HasRedBufferWithLowTicketFromStrings: Uses primitives (client mode)
package lessons

import (
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// HasRedBufferWithLowTicket detects UC19 trigger (engine mode):
// Buffer >66% consumed AND at least one active LOW understanding ticket.
func HasRedBufferWithLowTicket(feverStatus model.FeverStatus, activeTickets []model.Ticket) bool {
	if feverStatus != model.FeverRed {
		return false
	}
	for _, t := range activeTickets {
		if t.UnderstandingLevel == model.LowUnderstanding {
			return true
		}
	}
	return false
}

// LowUnderstandingString is the string representation of LOW understanding level.
// Matches model.LowUnderstanding.String() but defined here to avoid import cycles.
const LowUnderstandingString = "LOW"

// HasRedBufferWithLowTicketFromStrings detects UC19 trigger (client mode):
// Takes primitive []string to avoid importing tui package (would cause cycle).
func HasRedBufferWithLowTicketFromStrings(isRedBuffer bool, understandingLevels []string) bool {
	if !isRedBuffer {
		return false
	}
	for _, u := range understandingLevels {
		if u == LowUnderstandingString {
			return true
		}
	}
	return false
}

// HasQueueImbalance detects UC20 trigger (engine mode).
// Delegates to metrics package for calculation.
func HasQueueImbalance(activeTickets []model.Ticket) bool {
	return metrics.HasQueueImbalance(activeTickets)
}

// HasHighChildVariance detects UC21 trigger (engine mode).
// Delegates to metrics package for calculation.
func HasHighChildVariance(completedTickets []model.Ticket) bool {
	return metrics.HasHighChildVariance(completedTickets)
}

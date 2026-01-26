package metrics

import "github.com/binaryphile/sofdevsim-2026/internal/model"

// QueueDepthPerPhase returns ticket counts for each active phase.
// Excludes PhaseBacklog and PhaseDone (not active work).
// Calculation: []Ticket → map[Phase]count
func QueueDepthPerPhase(activeTickets []model.Ticket) map[model.WorkflowPhase]int {
	depths := make(map[model.WorkflowPhase]int)
	for _, t := range activeTickets {
		depths[t.Phase]++
	}
	return depths
}

// HasQueueImbalance returns true if any phase queue > 2× average.
// Calculation: []Ticket → bool
func HasQueueImbalance(activeTickets []model.Ticket) bool {
	depths := QueueDepthPerPhase(activeTickets)
	if len(depths) == 0 {
		return false
	}

	var sum int
	for _, d := range depths {
		sum += d
	}
	avg := float64(sum) / float64(len(depths))

	for _, d := range depths {
		if float64(d) > 2*avg {
			return true
		}
	}
	return false
}

package metrics

import (
	"sort"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// PhaseSnapshot captures the minimal state needed for TOC state diffing.
// Deep-copyable (maps only, no pointer aliasing).
type PhaseSnapshot struct {
	TicketPhases  map[string]model.WorkflowPhase // ticketID → current phase
	TicketEntered map[string]int                 // ticketID → PhaseEnteredTick
	DevTickets    map[string]string              // devID → CurrentTicket
}

// NewPhaseSnapshot extracts TOC-relevant state from a simulation.
func NewPhaseSnapshot(sim model.Simulation) PhaseSnapshot {
	ps := PhaseSnapshot{
		TicketPhases:  make(map[string]model.WorkflowPhase, len(sim.ActiveTickets)),
		TicketEntered: make(map[string]int, len(sim.ActiveTickets)),
		DevTickets:    make(map[string]string, len(sim.Developers)),
	}
	for _, t := range sim.ActiveTickets {
		ps.TicketPhases[t.ID] = t.Phase
		ps.TicketEntered[t.ID] = t.PhaseEnteredTick
	}
	for _, d := range sim.Developers {
		if d.CurrentTicket != "" {
			ps.DevTickets[d.ID] = d.CurrentTicket
		}
	}
	return ps
}

// TickData holds per-tick observations for the rolling window ring buffer.
type TickData struct {
	PhaseQueue    map[model.WorkflowPhase]int   // queue depth per phase this tick
	PhaseWorkers  map[model.WorkflowPhase]int   // devs working in each phase this tick
	PhaseArrivals map[model.WorkflowPhase]int   // tickets entering each phase this tick
	PhaseDepartures map[model.WorkflowPhase]int // tickets leaving each phase this tick
	DwellSamples  map[model.WorkflowPhase][]int // dwell times for tickets completing each phase
}

// NewTickData creates an empty TickData.
func NewTickData() *TickData {
	return &TickData{
		PhaseQueue:      make(map[model.WorkflowPhase]int),
		PhaseWorkers:    make(map[model.WorkflowPhase]int),
		PhaseArrivals:   make(map[model.WorkflowPhase]int),
		PhaseDepartures: make(map[model.WorkflowPhase]int),
		DwellSamples:    make(map[model.WorkflowPhase][]int),
	}
}

// CollectTickData builds a TickData from current and previous snapshots.
func CollectTickData(prev, curr PhaseSnapshot, currentTick int, sim model.Simulation) *TickData {
	td := NewTickData()

	// Queue depth: tickets in phase queues + assigned active tickets per phase
	for phase, queue := range sim.PhaseQueues {
		td.PhaseQueue[phase] += len(queue)
	}
	for _, t := range sim.ActiveTickets {
		if t.AssignedTo != "" {
			td.PhaseQueue[t.Phase]++ // assigned tickets count toward phase occupancy
		}
	}

	// Workers: devs whose CurrentTicket is in each phase
	for _, d := range sim.Developers { // justified:CF
		if d.CurrentTicket == "" {
			continue
		}
		idx := sim.FindActiveTicketIndex(d.CurrentTicket)
		if idx == -1 {
			continue
		}
		phase := sim.ActiveTickets[idx].Phase
		td.PhaseWorkers[phase]++
	}

	// Arrivals/Departures: diff ticket phases between prev and curr snapshots
	for ticketID, currPhase := range curr.TicketPhases {
		prevPhase, existed := prev.TicketPhases[ticketID]
		if !existed {
			// New ticket entered the system — arrival to its current phase
			td.PhaseArrivals[currPhase]++
		} else if currPhase != prevPhase {
			// Phase changed — departure from old, arrival to new
			td.PhaseDepartures[prevPhase]++
			td.PhaseArrivals[currPhase]++

			// Dwell time for the completed phase visit
			if enterTick, ok := prev.TicketEntered[ticketID]; ok && enterTick > 0 {
				dwell := currentTick - enterTick
				if dwell > 0 {
					td.DwellSamples[prevPhase] = append(td.DwellSamples[prevPhase], dwell)
				}
			}
		}
	}
	// Tickets that disappeared from active (completed)
	for ticketID, prevPhase := range prev.TicketPhases {
		if _, exists := curr.TicketPhases[ticketID]; !exists {
			td.PhaseDepartures[prevPhase]++
			if enterTick, ok := prev.TicketEntered[ticketID]; ok && enterTick > 0 {
				dwell := currentTick - enterTick
				if dwell > 0 {
					td.DwellSamples[prevPhase] = append(td.DwellSamples[prevPhase], dwell)
				}
			}
		}
	}

	return td
}

// MedianInt returns the median of a slice of ints. Returns 0 for empty slices.
func MedianInt(values []int) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	sorted := make([]int, n)
	copy(sorted, values)
	sort.Ints(sorted)
	if n%2 == 0 {
		return float64(sorted[n/2-1]+sorted[n/2]) / 2.0
	}
	return float64(sorted[n/2])
}

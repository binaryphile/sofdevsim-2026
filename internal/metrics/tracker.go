package metrics

import (
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Tracker combines all metrics tracking
type Tracker struct {
	DORA  DORAMetrics
	Fever FeverChart
	TOC   *TOCState
}

// NewTracker creates an initialized metrics tracker
func NewTracker() Tracker {
	return Tracker{
		DORA:  NewDORAMetrics(),
		Fever: NewFeverChart(),
		TOC:   NewTOCState(TOCFlow, 10),
	}
}

// Updated recalculates all metrics from simulation state and returns the updated value
func (t Tracker) Updated(sim model.Simulation) Tracker {
	t.DORA = t.DORA.Updated(sim)
	if sprint, ok := sim.CurrentSprintOption.Get(); ok {
		t.Fever = t.Fever.Updated(sprint)
	}
	if t.TOC != nil {
		t.TOC.Update(sim)
	}
	return t
}

// GetResult extracts a simulation result for comparison
func (t Tracker) GetResult(policy model.SizingPolicy, sim model.Simulation) SimulationResult {
	// Calculate average fever status from history
	avgFever := 0.0
	if len(t.Fever.History) > 0 {
		// sumFeverStatus accumulates fever status values as float64.
		sumFeverStatus := func(acc float64, s FeverSnapshot) float64 { return acc + float64(s.Status) }
		sum := slice.Fold(t.Fever.History, 0.0, sumFeverStatus)
		avgFever = sum / float64(len(t.Fever.History))
	}

	return SimulationResult{
		Policy:          policy,
		FinalMetrics:    t.DORA,
		TicketsComplete: len(sim.CompletedTickets),
		IncidentCount:   sim.TotalIncidents(),
		AvgFeverStatus:  avgFever,
	}
}

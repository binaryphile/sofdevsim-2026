package metrics

import (
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Tracker combines all metrics tracking
type Tracker struct {
	DORA  *DORAMetrics
	Fever *FeverChart
}

// NewTracker creates an initialized metrics tracker
func NewTracker() *Tracker {
	return &Tracker{
		DORA:  NewDORAMetrics(),
		Fever: NewFeverChart(),
	}
}

// Update recalculates all metrics from simulation state
func (t *Tracker) Update(sim *model.Simulation) {
	t.DORA.Update(sim)
	if sprint, ok := sim.CurrentSprintOption.Get(); ok {
		t.Fever.Update(sprint)
	}
}

// GetResult extracts a simulation result for comparison
func (t *Tracker) GetResult(policy model.SizingPolicy, sim *model.Simulation) SimulationResult {
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

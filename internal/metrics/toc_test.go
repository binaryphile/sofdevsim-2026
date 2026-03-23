package metrics

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestMedianInt(t *testing.T) {
	tests := []struct {
		name   string
		values []int
		want   float64
	}{
		{"empty", nil, 0},
		{"single", []int{5}, 5.0},
		{"two", []int{3, 7}, 5.0},
		{"odd", []int{1, 3, 5}, 3.0},
		{"even", []int{1, 3, 5, 7}, 4.0},
		{"unsorted", []int{7, 1, 5, 3}, 4.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MedianInt(tt.values)
			if got != tt.want {
				t.Errorf("MedianInt(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestPhaseSnapshot_DetectsPhaseChange(t *testing.T) {
	prev := PhaseSnapshot{
		TicketPhases:  map[string]model.WorkflowPhase{"T-1": model.PhaseImplement},
		TicketEntered: map[string]int{"T-1": 5},
		DevTickets:    map[string]string{"d1": "T-1"},
	}
	curr := PhaseSnapshot{
		TicketPhases:  map[string]model.WorkflowPhase{"T-1": model.PhaseVerify},
		TicketEntered: map[string]int{"T-1": 8},
		DevTickets:    map[string]string{"d1": "T-1"},
	}

	sim := model.NewSimulation("test", model.PolicyNone, 42)
	sim.ActiveTickets = append(sim.ActiveTickets, model.NewTicket("T-1", "Task", 5.0, model.MediumUnderstanding))
	sim.ActiveTickets[0].Phase = model.PhaseVerify
	sim.CurrentTick = 8

	td := CollectTickData(prev, curr, 8, sim)

	if td.PhaseDepartures[model.PhaseImplement] != 1 {
		t.Errorf("expected 1 departure from Implement, got %d", td.PhaseDepartures[model.PhaseImplement])
	}
	if td.PhaseArrivals[model.PhaseVerify] != 1 {
		t.Errorf("expected 1 arrival to Verify, got %d", td.PhaseArrivals[model.PhaseVerify])
	}
	// Dwell time: entered Implement at tick 5, left at tick 8 = 3
	samples := td.DwellSamples[model.PhaseImplement]
	if len(samples) != 1 || samples[0] != 3 {
		t.Errorf("expected dwell sample [3] for Implement, got %v", samples)
	}
}

func TestTOCState_RollingWindow(t *testing.T) {
	toc := NewTOCState(TOCFlow, 5) // 5-tick window for testing

	// Simulate 10 ticks with a simple scenario
	for tick := 1; tick <= 10; tick++ {
		sim := model.NewSimulation("test", model.PolicyNone, 42)
		sim.CurrentTick = tick
		// Add a ticket in Implement phase
		ticket := model.NewTicket("T-1", "Task", 5.0, model.MediumUnderstanding)
		ticket.Phase = model.PhaseImplement
		ticket.PhaseEnteredTick = 1
		ticket.AssignedTo = "d1"
		sim.ActiveTickets = append(sim.ActiveTickets, ticket)
		sim.Developers = append(sim.Developers, model.NewDeveloper("d1", "Dev", 1.0))
		sim.Developers[0].CurrentTicket = "T-1"

		toc.Update(sim)
	}

	// After 10 ticks with window size 5, ring should be full
	if toc.ringLen != 5 {
		t.Errorf("ringLen = %d, want 5", toc.ringLen)
	}

	// Implement should show in queue avg (ticket is assigned = occupancy)
	if toc.Flow.PhaseQueueAvg[model.PhaseImplement] <= 0 {
		t.Error("expected positive queue avg for Implement")
	}
}

func TestTOCState_ConstraintDetection(t *testing.T) {
	toc := NewTOCState(TOCFlow, 3) // small window for fast detection

	// Create a scenario where Review has consistently high dwell time
	for tick := 1; tick <= 25; tick++ {
		sim := model.NewSimulation("test", model.PolicyNone, 42)
		sim.CurrentTick = tick

		// Two tickets: one fast (Sizing), one slow (Review)
		// Simulate departures by alternating phase snapshots

		if tick%2 == 0 {
			// Even ticks: ticket in Review (slow phase)
			ticket := model.NewTicket("T-SLOW", "Slow", 5.0, model.MediumUnderstanding)
			ticket.Phase = model.PhaseReview
			ticket.PhaseEnteredTick = tick - 5 // entered 5 ticks ago
			ticket.AssignedTo = "d1"
			sim.ActiveTickets = append(sim.ActiveTickets, ticket)
		}

		sim.Developers = append(sim.Developers, model.NewDeveloper("d1", "Dev", 1.0))

		toc.Update(sim)
	}

	// After enough ticks, in-flight age for Review should be tracked
	// Constraint detection depends on departures with dwell samples
	// This is a basic smoke test — golden tests below verify specific scenarios
}

func TestTOCState_NoConstraintWhenBalanced(t *testing.T) {
	toc := NewTOCState(TOCFlow, 3)

	// Simulate balanced phases with similar dwell times
	prev := PhaseSnapshot{
		TicketPhases:  make(map[string]model.WorkflowPhase),
		TicketEntered: make(map[string]int),
		DevTickets:    make(map[string]string),
	}
	toc.prevSnap = prev

	for tick := 1; tick <= 30; tick++ {
		sim := model.NewSimulation("test", model.PolicyNone, 42)
		sim.CurrentTick = tick
		toc.Update(sim)
	}

	// No constraint should be identified (no departures = no dwell samples)
	if toc.ConstraintPhase != 0 {
		t.Errorf("expected no constraint, got %v", toc.ConstraintPhase)
	}
}

func TestTOCState_InFlightAge(t *testing.T) {
	toc := NewTOCState(TOCFlow, 3)

	sim := model.NewSimulation("test", model.PolicyNone, 42)
	sim.CurrentTick = 10

	ticket := model.NewTicket("T-1", "Task", 5.0, model.MediumUnderstanding)
	ticket.Phase = model.PhaseImplement
	ticket.PhaseEnteredTick = 3 // entered 7 ticks ago
	ticket.AssignedTo = "d1"
	sim.ActiveTickets = append(sim.ActiveTickets, ticket)
	sim.Developers = append(sim.Developers, model.NewDeveloper("d1", "Dev", 1.0))
	sim.Developers[0].CurrentTicket = "T-1"

	toc.Update(sim)
	toc.Update(sim) // need 2 updates for diff

	// Max in-flight age for Implement should be 7 (tick 10 - entered tick 3)
	if toc.Flow.PhaseMaxAge[model.PhaseImplement] != 7 {
		t.Errorf("max in-flight age = %d, want 7", toc.Flow.PhaseMaxAge[model.PhaseImplement])
	}
}

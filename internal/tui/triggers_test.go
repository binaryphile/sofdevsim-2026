package tui

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestProjectFromSimulation(t *testing.T) {
	sim := model.Simulation{
		SprintNumber: 5,
	}

	projection := ProjectFromSimulation(sim)

	if projection.sprintCount != 5 {
		t.Errorf("sprintCount = %d, want 5", projection.sprintCount)
	}
}

func TestTriggerProjection_ToTriggerState(t *testing.T) {
	projection := TriggerProjection{sprintCount: 3}

	got := projection.ToTriggerState()

	if got.SprintCount != 3 {
		t.Errorf("SprintCount = %d, want 3", got.SprintCount)
	}
	// Event triggers should be zero-valued
	if got.HasRedBufferWithLowTicket {
		t.Error("expected HasRedBufferWithLowTicket to be false")
	}
	if got.HasQueueImbalance {
		t.Error("expected HasQueueImbalance to be false")
	}
	if got.HasHighChildVariance {
		t.Error("expected HasHighChildVariance to be false")
	}
}

func TestTriggerProjection_MergeEventTriggers(t *testing.T) {
	projection := TriggerProjection{sprintCount: 4}

	got := projection.MergeEventTriggers(true, false, true)

	if got.SprintCount != 4 {
		t.Errorf("SprintCount = %d, want 4", got.SprintCount)
	}
	if !got.HasRedBufferWithLowTicket {
		t.Error("expected HasRedBufferWithLowTicket to be true")
	}
	if got.HasQueueImbalance {
		t.Error("expected HasQueueImbalance to be false")
	}
	if !got.HasHighChildVariance {
		t.Error("expected HasHighChildVariance to be true")
	}
}

func TestBuildTriggerStateFromEngine(t *testing.T) {
	sim := model.Simulation{SprintNumber: 3}
	// Create tickets that will trigger UC19 (red buffer + LOW ticket)
	activeTickets := []model.Ticket{
		{UnderstandingLevel: model.LowUnderstanding},
	}

	got := BuildTriggerStateFromEngine(sim, model.FeverRed, activeTickets, nil)

	if got.SprintCount != 3 {
		t.Errorf("SprintCount = %d, want 3", got.SprintCount)
	}
	if !got.HasRedBufferWithLowTicket {
		t.Error("expected HasRedBufferWithLowTicket to be true (FeverRed + LOW ticket)")
	}
}

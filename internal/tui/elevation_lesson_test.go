package tui

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/lessons"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// TestTriggerFlow_ElevationWithoutExploitation — UC40 fu2 (#18517)
// controller integration test. Exercises sim → TOC → BuildTriggers → Select
// → lesson output. Uses a real Tracker + Simulation; no mocks. Khorikov
// §6275 canonical controller-tier posture.
func TestTriggerFlow_ElevationWithoutExploitation(t *testing.T) {
	// Construct a simulation where the elevation-without-exploitation
	// scenario is mid-flight: an investment occurred (LastInvestmentApplied
	// non-empty), the prior sprint identified PhaseImplement as the
	// constraint, the new sprint is running, and the constraint phase has
	// not moved (still PhaseImplement).
	sim := model.NewSimulation("elev-no-exploit", model.PolicyNone, 42)
	sim.SprintNumber = 2
	sim.LastInvestmentApplied = "cicd-slot"

	tracker := metrics.NewTracker()
	tracker.TOC.ConstraintPhase = model.PhaseImplement
	tracker.TOC.LastSprintConstraintPhase = model.PhaseImplement
	tracker.TOC.InvestmentOccurredThisCycle = true

	// Build the engine-mode trigger state via the production path.
	triggers := BuildTriggerStateFromEngine(sim, model.FeverGreen, sim.ActiveTickets, sim.CompletedTickets, tracker.TOC)

	if !triggers.HasElevationWithoutExploitation {
		t.Fatal("triggers.HasElevationWithoutExploitation should be true (investment applied + constraint unchanged)")
	}

	// Select the lesson with Orientation already seen (so we skip the first-time path).
	state := lessons.State{SeenMap: map[lessons.LessonID]bool{lessons.Orientation: true}}
	lesson := SelectLesson(ViewExecution, state, true, false, triggers, ComparisonSummary{})

	if lesson.ID != lessons.ElevationWithoutExploitation {
		t.Errorf("lesson.ID = %v, want %v (full sim → TOC → triggers → Select → lesson flow)", lesson.ID, lessons.ElevationWithoutExploitation)
	}
}

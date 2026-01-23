package model_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Test domain logic: phase effort calculation with understanding multipliers
func TestTicket_CalculatePhaseEffort_ReturnsCorrectDistribution(t *testing.T) {
	tests := []struct {
		name          string
		estimatedDays float64
		understanding model.UnderstandingLevel
		phase         model.WorkflowPhase
		wantMin       float64
		wantMax       float64
	}{
		{
			name:          "high understanding research phase is quick",
			estimatedDays: 10,
			understanding: model.HighUnderstanding,
			phase:         model.PhaseResearch,
			// 10 * 0.05 (base) * 0.5 (high understanding multiplier) = 0.25
			wantMin: 0.20,
			wantMax: 0.30,
		},
		{
			name:          "low understanding research phase takes 3x longer",
			estimatedDays: 10,
			understanding: model.LowUnderstanding,
			phase:         model.PhaseResearch,
			// 10 * 0.05 (base) * 3.0 (low understanding multiplier) = 1.5
			wantMin: 1.40,
			wantMax: 1.60,
		},
		{
			name:          "implement phase is bulk of work",
			estimatedDays: 10,
			understanding: model.MediumUnderstanding,
			phase:         model.PhaseImplement,
			// 10 * 0.55 (base) * 1.1 (medium multiplier) = 6.05
			wantMin: 5.90,
			wantMax: 6.20,
		},
		{
			name:          "phase without multiplier uses base percentage",
			estimatedDays: 10,
			understanding: model.HighUnderstanding,
			phase:         model.PhasePlanning,
			// 10 * 0.03 = 0.3 (no multiplier for planning)
			wantMin: 0.29,
			wantMax: 0.31,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := model.NewTicket("TKT-001", "Test", tt.estimatedDays, tt.understanding)
			got := ticket.CalculatePhaseEffort(tt.phase)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculatePhaseEffort() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// Test that phase effort percentages sum to 1.0 (important invariant)
func TestPhaseEffortPct_SumsToOne_AcrossAllPhases(t *testing.T) {
	var total float64
	for phase := model.PhaseResearch; phase <= model.PhaseReview; phase++ {
		total += model.PhaseEffortPct[phase]
	}

	if total < 0.99 || total > 1.01 {
		t.Errorf("PhaseEffortPct sums to %v, want 1.0", total)
	}
}

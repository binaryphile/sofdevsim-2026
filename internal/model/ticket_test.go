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
	for phase := model.PhaseResearch; phase <= model.PhaseReview; phase++ { // justified:IX
		total += model.PhaseEffortPct[phase]
	}

	if total < 0.99 || total > 1.01 {
		t.Errorf("PhaseEffortPct sums to %v, want 1.0", total)
	}
}

// UC37: each per-type phase-effort distribution sums to 1.00 (row-sum invariant).
func TestTypePhaseEffortPct_RowsSumToOne(t *testing.T) {
	types := []model.TicketType{
		model.TicketTypeFeature,
		model.TicketTypeBug,
		model.TicketTypeSpike,
		model.TicketTypeMigration,
		model.TicketTypeInfra,
	}
	for _, tt := range types {
		var total float64
		for phase := model.PhaseResearch; phase <= model.PhaseReview; phase++ { // justified:IX
			total += model.TypePhaseEffortPct[tt][phase]
		}
		if total < 0.99 || total > 1.01 {
			t.Errorf("TypePhaseEffortPct[%s] sums to %v, want 1.0", tt, total)
		}
	}
}

// UC37: Feature row of TypePhaseEffortPct is byte-identical to pre-UC37 global
// PhaseEffortPct (regression sentinel — all-Feature mixes reproduce today's behaviour).
func TestTypePhaseEffortPct_FeatureRowMatchesLegacyGlobal(t *testing.T) {
	// PhaseEffortPct is a backward-compat alias pointing to TypePhaseEffortPct[TicketTypeFeature];
	// both should be the same map.
	expected := map[model.WorkflowPhase]float64{
		model.PhaseResearch:  0.05,
		model.PhaseSizing:    0.02,
		model.PhasePlanning:  0.03,
		model.PhaseImplement: 0.55,
		model.PhaseVerify:    0.20,
		model.PhaseCICD:      0.05,
		model.PhaseReview:    0.10,
	}
	for phase, wantPct := range expected {
		if got := model.TypePhaseEffortPct[model.TicketTypeFeature][phase]; got != wantPct {
			t.Errorf("Feature row [%s] = %v, want %v", phase, got, wantPct)
		}
		if got := model.PhaseEffortPct[phase]; got != wantPct {
			t.Errorf("legacy PhaseEffortPct alias [%s] = %v, want %v (alias must point to Feature row)", phase, got, wantPct)
		}
	}
}

// UC37: per-type CalculatePhaseEffort returns the type-specific distribution.
func TestCalculatePhaseEffort_PerType(t *testing.T) {
	tests := []struct {
		name          string
		ticketType    model.TicketType
		phase         model.WorkflowPhase
		estimatedDays float64
		// Medium understanding has no multiplier on Sizing/Planning/CICD/Review,
		// so we can compare raw effort there without multiplier noise.
		understanding model.UnderstandingLevel
		want          float64
	}{
		{"bug skips Planning (0.00)", model.TicketTypeBug, model.PhasePlanning, 10, model.MediumUnderstanding, 0.00},
		{"bug heavier Review (0.15)", model.TicketTypeBug, model.PhaseReview, 10, model.MediumUnderstanding, 1.50},
		{"spike CI/CD = 0", model.TicketTypeSpike, model.PhaseCICD, 10, model.MediumUnderstanding, 0.00},
		{"spike Sizing 0.05", model.TicketTypeSpike, model.PhaseSizing, 10, model.MediumUnderstanding, 0.50},
		{"migration Planning 0.10", model.TicketTypeMigration, model.PhasePlanning, 10, model.MediumUnderstanding, 1.00},
		{"infra CI/CD 0.20", model.TicketTypeInfra, model.PhaseCICD, 10, model.MediumUnderstanding, 2.00},
		{"feature Sizing matches legacy 0.02", model.TicketTypeFeature, model.PhaseSizing, 10, model.MediumUnderstanding, 0.20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := model.NewTicket("T1", "test", tt.estimatedDays, tt.understanding)
			ticket.Type = tt.ticketType
			got := ticket.CalculatePhaseEffort(tt.phase)
			if got < tt.want-0.001 || got > tt.want+0.001 {
				t.Errorf("CalculatePhaseEffort(%s, %s) = %v, want %v", tt.ticketType, tt.phase, got, tt.want)
			}
		})
	}
}

// UC37: understanding multiplier still applies to Research/Implement/Verify
// regardless of ticket type (axes-orthogonal contract).
func TestCalculatePhaseEffort_UnderstandingMultiplierPreservedAcrossTypes(t *testing.T) {
	// Spike + Low understanding: Research = 10 * 0.55 * 3.0 = 16.5
	ticket := model.NewTicket("T1", "test", 10, model.LowUnderstanding)
	ticket.Type = model.TicketTypeSpike
	got := ticket.CalculatePhaseEffort(model.PhaseResearch)
	want := 16.5
	if got < want-0.01 || got > want+0.01 {
		t.Errorf("Spike+Low Research effort = %v, want %v (understanding multiplier MUST apply per type)", got, want)
	}
}

// UC37 defensive-fallback (plan §commit 5): unrecognised Type values fall back
// to the Feature distribution. Protects against external-source corruption.
func TestCalculatePhaseEffort_UnknownTypeFallsBackToFeature(t *testing.T) {
	ticket := model.NewTicket("T1", "test", 10, model.MediumUnderstanding)
	ticket.Type = model.TicketType(99) // out-of-range
	// Should use Feature distribution: PhaseSizing = 0.02 * 10 = 0.20
	got := ticket.CalculatePhaseEffort(model.PhaseSizing)
	want := 0.20
	if got < want-0.001 || got > want+0.001 {
		t.Errorf("CalculatePhaseEffort with out-of-range Type = %v, want %v (Feature fallback)", got, want)
	}
	// Sanity: not zero — defensive fallback exists to avoid silent phase-skip
	if got == 0 {
		t.Errorf("CalculatePhaseEffort returned 0 for unrecognised Type — defensive fallback failed")
	}
}

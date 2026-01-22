package engine_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Test variance algorithm: understanding level affects predictability
func TestVarianceModel_Calculate(t *testing.T) {
	vm := engine.NewVarianceModel(12345)

	// Run multiple iterations to verify distribution properties
	const iterations = 1000

	tests := []struct {
		name          string
		understanding model.UnderstandingLevel
		phase         model.WorkflowPhase
		wantMeanMin   float64
		wantMeanMax   float64
		wantSpreadMax float64 // max - min should be small for high understanding
	}{
		{
			name:          "high understanding is predictable",
			understanding: model.HighUnderstanding,
			phase:         model.PhaseImplement,
			wantMeanMin:   0.85,
			wantMeanMax:   0.95,
			wantSpreadMax: 0.15, // tight distribution
		},
		{
			name:          "low understanding is unpredictable",
			understanding: model.LowUnderstanding,
			phase:         model.PhaseImplement,
			// Base: 0.50-1.50 * 1.5 (phase multiplier) = 0.75-2.25, mean ~1.5
			wantMeanMin:   1.30,
			wantMeanMax:   1.70,
			wantSpreadMax: 2.50, // wide distribution
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := model.NewTicket("TKT-001", "Test", 5, tt.understanding)
			ticket.Phase = tt.phase

			var sum, min, max float64
			min = 999
			max = 0

			for i := 0; i < iterations; i++ {
				v := vm.Calculate(ticket, i)
				sum += v
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}

			mean := sum / iterations
			spread := max - min

			if mean < tt.wantMeanMin || mean > tt.wantMeanMax {
				t.Errorf("mean = %v, want between %v and %v", mean, tt.wantMeanMin, tt.wantMeanMax)
			}

			if spread > tt.wantSpreadMax {
				t.Errorf("spread = %v (min=%v, max=%v), want <= %v", spread, min, max, tt.wantSpreadMax)
			}
		})
	}
}

// Test reproducibility: same seed produces same results
func TestVarianceModel_Reproducibility(t *testing.T) {
	seed := int64(42)
	vm1 := engine.NewVarianceModel(seed)
	vm2 := engine.NewVarianceModel(seed)

	ticket := model.NewTicket("TKT-001", "Test", 5, model.MediumUnderstanding)
	ticket.Phase = model.PhaseImplement

	for tick := 0; tick < 100; tick++ {
		v1 := vm1.Calculate(ticket, tick)
		v2 := vm2.Calculate(ticket, tick)

		if v1 != v2 {
			t.Errorf("tick %d: v1=%v != v2=%v (should be reproducible)", tick, v1, v2)
		}
	}
}

// Test edge case: seed=0 should still produce valid, reproducible results
// Per Khorikov: edge case tests protect against regressions
func TestVarianceModel_ZeroSeed(t *testing.T) {
	vm1 := engine.NewVarianceModel(0)
	vm2 := engine.NewVarianceModel(0)

	ticket := model.NewTicket("TKT-001", "Test", 5, model.MediumUnderstanding)
	ticket.Phase = model.PhaseImplement

	for tick := 0; tick < 100; tick++ {
		v1 := vm1.Calculate(ticket, tick)
		v2 := vm2.Calculate(ticket, tick)

		// seed=0 should still be reproducible
		if v1 != v2 {
			t.Errorf("tick %d: seed=0 not reproducible: %v != %v", tick, v1, v2)
		}

		// Variance should always be positive
		if v1 <= 0 {
			t.Errorf("tick %d: variance should be positive, got %v", tick, v1)
		}
	}
}

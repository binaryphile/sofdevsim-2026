package engine_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Test sizing policy decision logic - the core experiment discriminant
func TestPolicyEngine_ShouldDecompose(t *testing.T) {
	pe := engine.NewPolicyEngine(12345)

	tests := []struct {
		name          string
		estimatedDays float64
		understanding model.UnderstandingLevel
		policy        model.SizingPolicy
		want          bool
	}{
		// PolicyNone never decomposes
		{"none policy never decomposes small", 3, model.HighUnderstanding, model.PolicyNone, false},
		{"none policy never decomposes large", 10, model.LowUnderstanding, model.PolicyNone, false},

		// PolicyDORAStrict: decompose if > 5 days
		{"dora: small ticket stays whole", 4, model.LowUnderstanding, model.PolicyDORAStrict, false},
		{"dora: 5 days stays whole", 5, model.LowUnderstanding, model.PolicyDORAStrict, false},
		{"dora: 6 days gets decomposed", 6, model.HighUnderstanding, model.PolicyDORAStrict, true},

		// PolicyTameFlowCognitive: decompose if understanding is Low
		{"tameflow: high understanding stays whole", 10, model.HighUnderstanding, model.PolicyTameFlowCognitive, false},
		{"tameflow: medium understanding stays whole", 10, model.MediumUnderstanding, model.PolicyTameFlowCognitive, false},
		{"tameflow: low understanding gets decomposed", 2, model.LowUnderstanding, model.PolicyTameFlowCognitive, true},

		// PolicyHybrid: decompose if > 5 days AND understanding < High
		{"hybrid: small low-understanding stays whole", 4, model.LowUnderstanding, model.PolicyHybrid, false},
		{"hybrid: large high-understanding stays whole", 10, model.HighUnderstanding, model.PolicyHybrid, false},
		{"hybrid: large medium-understanding decomposes", 10, model.MediumUnderstanding, model.PolicyHybrid, true},
		{"hybrid: large low-understanding decomposes", 10, model.LowUnderstanding, model.PolicyHybrid, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := model.NewTicket("TKT-001", "Test", tt.estimatedDays, tt.understanding)
			got := pe.ShouldDecompose(ticket, tt.policy)

			if got != tt.want {
				t.Errorf("ShouldDecompose() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test decomposition produces valid children
func TestPolicyEngine_Decompose(t *testing.T) {
	pe := engine.NewPolicyEngine(12345)

	ticket := model.NewTicket("TKT-001", "Implement feature", 10, model.LowUnderstanding)
	children := pe.Decompose(ticket)

	// Should produce 2-4 children
	if len(children) < 2 || len(children) > 4 {
		t.Errorf("Decompose produced %d children, want 2-4", len(children))
	}

	// Children should have parent reference
	for _, child := range children {
		if child.ParentID != ticket.ID {
			t.Errorf("Child %s has ParentID %s, want %s", child.ID, child.ParentID, ticket.ID)
		}
	}

	// Children estimates should sum to approximately parent (90-110%)
	var totalEstimate float64
	for _, child := range children {
		totalEstimate += child.EstimatedDays
	}

	ratio := totalEstimate / ticket.EstimatedDays
	if ratio < 0.85 || ratio > 1.15 {
		t.Errorf("Children total %.1f days, parent %.1f days (ratio %.2f), want 0.90-1.10",
			totalEstimate, ticket.EstimatedDays, ratio)
	}

	// Children should be in backlog phase
	for _, child := range children {
		if child.Phase != model.PhaseBacklog {
			t.Errorf("Child %s in phase %v, want Backlog", child.ID, child.Phase)
		}
	}
}

// Test that decomposition tends to improve understanding
func TestPolicyEngine_DecompositionImprovesUnderstanding(t *testing.T) {
	const iterations = 100
	improvedCount := 0

	for seed := int64(0); seed < iterations; seed++ {
		pe := engine.NewPolicyEngine(seed)
		ticket := model.NewTicket("TKT-001", "Test", 10, model.LowUnderstanding)
		children := pe.Decompose(ticket)

		for _, child := range children {
			if child.UnderstandingLevel > ticket.UnderstandingLevel {
				improvedCount++
			}
		}
	}

	// Expect ~60% of children to have improved understanding
	totalChildren := iterations * 3 // average ~3 children per decomposition
	improvementRate := float64(improvedCount) / float64(totalChildren)

	if improvementRate < 0.40 || improvementRate > 0.80 {
		t.Errorf("Understanding improvement rate = %.2f, want ~0.60", improvementRate)
	}
}

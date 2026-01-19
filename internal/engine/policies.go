package engine

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// PolicyEngine handles sizing policy decisions and decomposition
type PolicyEngine struct {
	seed int64
}

// NewPolicyEngine creates a policy engine.
// Returns value type: pure decision logic, no mutation.
func NewPolicyEngine(seed int64) PolicyEngine {
	return PolicyEngine{seed: seed}
}

// ShouldDecompose determines if a ticket should be decomposed based on policy
func (p PolicyEngine) ShouldDecompose(ticket model.Ticket, policy model.SizingPolicy) bool {
	switch policy {
	case model.PolicyNone:
		return false
	case model.PolicyDORAStrict:
		return ticket.EstimatedDays > 5
	case model.PolicyTameFlowCognitive:
		return ticket.UnderstandingLevel == model.LowUnderstanding
	case model.PolicyHybrid:
		return ticket.EstimatedDays > 5 && ticket.UnderstandingLevel < model.HighUnderstanding
	}
	return false
}

// Decompose splits a ticket into smaller children
func (p PolicyEngine) Decompose(ticket model.Ticket) []model.Ticket {
	rng := rand.New(rand.NewSource(p.seed + int64(ticket.ID[0])))

	// Determine number of children (2-4, weighted toward 2-3)
	numChildren := 2 + weightedChoice([]float64{0.4, 0.4, 0.2}, rng)

	children := make([]model.Ticket, numChildren)

	// Distribute parent estimate with variance
	// Children sum to 90-110% of parent (decomposition isn't free, but reveals scope)
	totalMultiplier := 0.9 + rng.Float64()*0.2
	baseEstimate := (ticket.EstimatedDays * totalMultiplier) / float64(numChildren)

	for i := range children {
		// Each child varies ±30% from base
		variance := 0.7 + rng.Float64()*0.6
		childEstimate := baseEstimate * variance

		children[i] = model.Ticket{
			ID:                 fmt.Sprintf("%s-%d", ticket.ID, i+1),
			Title:              fmt.Sprintf("%s (Part %d/%d)", ticket.Title, i+1, numChildren),
			EstimatedDays:      childEstimate,
			UnderstandingLevel: improveUnderstanding(ticket.UnderstandingLevel, rng),
			ParentID:           ticket.ID,
			Phase:              model.PhaseBacklog,
			PhaseEffortSpent:   make(map[model.WorkflowPhase]float64),
			CreatedAt:          time.Now(),
		}
	}

	return children
}

// improveUnderstanding simulates that decomposition often improves understanding
func improveUnderstanding(current model.UnderstandingLevel, rng *rand.Rand) model.UnderstandingLevel {
	if current == model.HighUnderstanding {
		return model.HighUnderstanding
	}
	// 60% chance to improve one level
	if rng.Float64() < 0.6 {
		return current + 1
	}
	return current
}

// weightedChoice returns 0, 1, or 2 based on weights
func weightedChoice(weights []float64, rng *rand.Rand) int {
	r := rng.Float64()
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if r < cumulative {
			return i
		}
	}
	return len(weights) - 1
}

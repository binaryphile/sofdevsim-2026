package engine_test

import (
	"math/rand"
	"testing"

	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Test generator produces tickets with expected distributions
func TestTicketGenerator_Generate(t *testing.T) {
	gen := engine.Scenarios["healthy"]
	rng := rand.New(rand.NewSource(12345))

	tickets := gen.Generate(rng, 100)

	if len(tickets) != 100 {
		t.Fatalf("Generate(100) produced %d tickets", len(tickets))
	}

	// Count understanding distribution
	var low, med, high int
	var totalSize float64

	for _, ticket := range tickets { // justified:CF
		switch ticket.UnderstandingLevel {
		case model.LowUnderstanding:
			low++
		case model.MediumUnderstanding:
			med++
		case model.HighUnderstanding:
			high++
		}
		totalSize += ticket.EstimatedDays

		// All should be in backlog
		if ticket.Phase != model.PhaseBacklog {
			t.Errorf("Ticket %s in phase %v, want Backlog", ticket.ID, ticket.Phase)
		}

		// Size should be clamped
		if ticket.EstimatedDays < 0.5 || ticket.EstimatedDays > 20 {
			t.Errorf("Ticket %s has size %.1f, want 0.5-20", ticket.ID, ticket.EstimatedDays)
		}
	}

	// Check distribution roughly matches config (within 20% tolerance)
	// healthy: LowPct: 0.20, MediumPct: 0.50, HighPct: 0.30
	lowPct := float64(low) / 100
	medPct := float64(med) / 100
	highPct := float64(high) / 100

	if lowPct < 0.10 || lowPct > 0.35 {
		t.Errorf("Low understanding = %.0f%%, want ~20%%", lowPct*100)
	}
	if medPct < 0.35 || medPct > 0.65 {
		t.Errorf("Medium understanding = %.0f%%, want ~50%%", medPct*100)
	}
	if highPct < 0.15 || highPct > 0.45 {
		t.Errorf("High understanding = %.0f%%, want ~30%%", highPct*100)
	}

	// Mean size should be near SizeMean (3 for healthy)
	meanSize := totalSize / 100
	if meanSize < 2.0 || meanSize > 5.0 {
		t.Errorf("Mean size = %.1f, want ~3", meanSize)
	}
}

// Test different scenarios have different characteristics
func TestScenarios_HaveDifferentCharacteristics(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))

	healthy := engine.Scenarios["healthy"]
	overloaded := engine.Scenarios["overloaded"]
	uncertain := engine.Scenarios["uncertain"]

	healthyTickets := healthy.Generate(rng, 50)
	overloadedTickets := overloaded.Generate(rand.New(rand.NewSource(12345)), 50)
	uncertainTickets := uncertain.Generate(rand.New(rand.NewSource(12345)), 50)

	// Calculate mean sizes
	healthySum := slice.From(healthyTickets).ToFloat64(model.Ticket.GetEstimatedDays).Sum()
	overloadedSum := slice.From(overloadedTickets).ToFloat64(model.Ticket.GetEstimatedDays).Sum()

	healthyMean := healthySum / 50
	overloadedMean := overloadedSum / 50

	// Overloaded should have larger tickets
	if overloadedMean <= healthyMean {
		t.Errorf("Overloaded mean (%.1f) should be > healthy mean (%.1f)", overloadedMean, healthyMean)
	}

	// Count low understanding in uncertain scenario
	// isLowUnderstanding returns true if ticket has low understanding.
	isLowUnderstanding := func(t model.Ticket) bool { return t.UnderstandingLevel == model.LowUnderstanding }
	uncertainLow := slice.From(uncertainTickets).KeepIf(isLowUnderstanding).Len()

	// Uncertain should have high proportion of low understanding (60%)
	if float64(uncertainLow)/50 < 0.45 {
		t.Errorf("Uncertain scenario has %d/50 low understanding, want ~60%%", uncertainLow)
	}
}

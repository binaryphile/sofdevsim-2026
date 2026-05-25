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

// UC37 cycle #15442: TicketGenerator.TypeWeights roll respects weights.
// Khorikov Algorithm quadrant — output-based testing over many draws.

// Nil TypeWeights produces all-Feature (regression-safe default).
func TestGenerate_NilWeightsAreAllFeature(t *testing.T) {
	gen := engine.Scenarios["healthy"]
	if gen.TypeWeights != nil {
		t.Fatalf("healthy scenario should have nil TypeWeights (all-Feature default); got %v", gen.TypeWeights)
	}
	rng := rand.New(rand.NewSource(42))
	tickets := gen.Generate(rng, 200)
	for _, tk := range tickets {
		if tk.Type != model.TicketTypeFeature {
			t.Errorf("expected all Feature with nil TypeWeights; got %s", tk.Type)
			break
		}
	}
}

// Single-type degenerate weights → 100% that type (UC37 ext §3a).
func TestGenerate_SingleTypeDegenerate(t *testing.T) {
	gen := engine.Scenarios["healthy"]
	gen.TypeWeights = map[model.TicketType]float64{
		model.TicketTypeBug: 1.0,
	}
	rng := rand.New(rand.NewSource(7))
	tickets := gen.Generate(rng, 100)
	for _, tk := range tickets {
		if tk.Type != model.TicketTypeBug {
			t.Errorf("expected 100%% Bug with single-type weights; got %s", tk.Type)
			break
		}
	}
}

// Non-1.0-sum weights normalise silently (UC37 ext §1a).
// 2:2 → 50/50 distribution within tolerance over 10K draws.
func TestGenerate_TypeWeightsNormalize(t *testing.T) {
	gen := engine.Scenarios["healthy"]
	gen.TypeWeights = map[model.TicketType]float64{
		model.TicketTypeFeature: 2.0,
		model.TicketTypeBug:     2.0,
	}
	rng := rand.New(rand.NewSource(1234))
	tickets := gen.Generate(rng, 10000)
	var feat, bug int
	for _, tk := range tickets { // justified:SM
		switch tk.Type {
		case model.TicketTypeFeature:
			feat++
		case model.TicketTypeBug:
			bug++
		default:
			t.Fatalf("unexpected type %s with Feature+Bug weights", tk.Type)
		}
	}
	// 50/50 within 3% tolerance (5000 ± 150 each)
	if feat < 4700 || feat > 5300 {
		t.Errorf("Feature count = %d, want ≈5000 ± 300 (3%% tolerance) — weight normalisation broken", feat)
	}
	if bug < 4700 || bug > 5300 {
		t.Errorf("Bug count = %d, want ≈5000 ± 300", bug)
	}
}

// research-shop scenario: aggregate Spike count should dominate (≈85%).
// Acts as regression sentinel for the integration-test pair (uc37-default vs research-shop).
func TestGenerate_ResearchShopIsSpikeDominated(t *testing.T) {
	gen := engine.Scenarios["research-shop"]
	rng := rand.New(rand.NewSource(42))
	tickets := gen.Generate(rng, 1000)
	var spike int
	for _, tk := range tickets { // justified:SM
		if tk.Type == model.TicketTypeSpike {
			spike++
		}
	}
	// Spike weighted 0.85 → expect ≈850 of 1000; tolerance ±50 (5%)
	if spike < 800 || spike > 900 {
		t.Errorf("research-shop Spike count = %d/1000, want ≈850 ± 50 (5%% tolerance)", spike)
	}
}

// uc37-default scenario: aggregate Feature count should dominate (≈60%).
func TestGenerate_UC37DefaultIsFeatureDominated(t *testing.T) {
	gen := engine.Scenarios["uc37-default"]
	rng := rand.New(rand.NewSource(42))
	tickets := gen.Generate(rng, 1000)
	var feat int
	for _, tk := range tickets { // justified:SM
		if tk.Type == model.TicketTypeFeature {
			feat++
		}
	}
	// Feature weighted 0.60 → expect ≈600 of 1000; tolerance ±50
	if feat < 550 || feat > 650 {
		t.Errorf("uc37-default Feature count = %d/1000, want ≈600 ± 50 (5%% tolerance)", feat)
	}
}

// Pinned-seed reproducibility: same seed + same mix profile = same Type sequence.
// (Different mix profiles diverge from the first roll because cumulative-weight
// ranges differ, but same-mix reproducibility holds.)
func TestGenerate_SameSeedSameMixIsReproducible(t *testing.T) {
	gen := engine.Scenarios["uc37-default"]
	a := gen.Generate(rand.New(rand.NewSource(42)), 50)
	b := gen.Generate(rand.New(rand.NewSource(42)), 50)
	for i := range a {
		if a[i].Type != b[i].Type {
			t.Errorf("seed-reproducibility broken at index %d: a=%s b=%s", i, a[i].Type, b[i].Type)
			break
		}
	}
}

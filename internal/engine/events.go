package engine

import (
	"fmt"
	"math/rand"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// EventGenerator creates random simulation events.
//
// Value receiver: only reads seed, creates fresh RNG per call (deterministic).
type EventGenerator struct {
	seed int64
}

// NewEventGenerator creates an event generator
func NewEventGenerator(seed int64) EventGenerator {
	return EventGenerator{seed: seed}
}

// GenerateRandomEvents generates bugs, scope creep, and incident resolutions.
// Returns ES events for state changes - caller emits them to update projection.
// This is a pure function: reads state, returns events, no side effects.
func (g EventGenerator) GenerateRandomEvents(state model.Simulation) []events.Event {
	result := make([]events.Event, 0)
	rng := rand.New(rand.NewSource(g.seed + int64(state.CurrentTick)))

	// Process active tickets for bugs and scope creep
	for _, ticket := range state.ActiveTickets {
		// Bug discovered (2% daily chance, higher for low understanding)
		bugChance := 0.02
		if ticket.UnderstandingLevel == model.LowUnderstanding {
			bugChance = 0.06
		}
		if rng.Float64() < bugChance {
			result = append(result, events.NewBugDiscovered(
				state.ID, state.CurrentTick, ticket.ID, 0.5,
			))
		}

		// Scope creep (1% daily chance)
		if rng.Float64() < 0.01 {
			addition := 0.5 + rng.Float64() // 0.5-1.5 days
			result = append(result, events.NewScopeCreepOccurred(
				state.ID, state.CurrentTick, ticket.ID, addition, addition,
			))
		}
	}

	// Resolve some open incidents
	for _, inc := range state.OpenIncidents {
		if !inc.IsOpen() {
			continue
		}

		// Resolution probability based on severity and time open
		daysOpen := state.CurrentTick - inc.CreatedAt.Day()
		resolveChance := 0.3 + float64(daysOpen)*0.1
		if inc.Severity == model.SeverityCritical {
			resolveChance += 0.3
		}

		if rng.Float64() < resolveChance {
			// DeveloperID empty: model.Incident doesn't track resolver
			result = append(result, events.NewIncidentResolved(
				state.ID, state.CurrentTick, inc.ID, "",
			))
		}
	}

	return result
}

// CheckForIncidents checks recently deployed tickets for production issues.
// Returns ES events for new incidents - caller emits them to update projection.
// This is a pure function: reads state, returns events, no side effects.
func (g EventGenerator) CheckForIncidents(state model.Simulation) []events.Event {
	result := make([]events.Event, 0)
	rng := rand.New(rand.NewSource(g.seed + int64(state.CurrentTick)))

	// Count existing incidents for ID generation
	incidentCount := len(state.OpenIncidents) + len(state.ResolvedIncidents)

	for _, ticket := range state.CompletedTickets {
		// Only check tickets deployed in last 3 days
		daysSinceDeployed := state.CurrentTick - ticket.CompletedAt.Day()
		if daysSinceDeployed > 3 || ticket.CausedIncident {
			continue
		}

		// Base failure rate varies by understanding
		var failRate float64
		switch ticket.UnderstandingLevel {
		case model.HighUnderstanding:
			failRate = 0.05 // 5% - well understood, fewer bugs
		case model.MediumUnderstanding:
			failRate = 0.12 // 12% - some unknowns
		case model.LowUnderstanding:
			failRate = 0.25 // 25% - high uncertainty, more bugs
		}

		// Large tickets have higher fail rate
		if ticket.EstimatedDays > 5 {
			failRate *= 1.5
		}

		if rng.Float64() < failRate {
			incidentCount++
			incidentID := fmt.Sprintf("INC-%03d", incidentCount)
			severity := model.Severity(rng.Intn(4))

			result = append(result, events.NewIncidentStarted(
				state.ID, state.CurrentTick, incidentID, ticket.AssignedTo, ticket.ID, severity,
			))
		}
	}

	return result
}

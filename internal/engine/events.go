package engine

import (
	"fmt"
	"math/rand"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// EventGenerator creates random simulation events
type EventGenerator struct {
	seed int64
}

// NewEventGenerator creates an event generator
func NewEventGenerator(seed int64) *EventGenerator {
	return &EventGenerator{seed: seed}
}

// GenerateRandomEvents generates bugs, scope creep, and incident resolutions
func (g *EventGenerator) GenerateRandomEvents(sim *model.Simulation) []model.Event {
	events := make([]model.Event, 0)
	rng := rand.New(rand.NewSource(g.seed + int64(sim.CurrentTick)))

	// Process active tickets for bugs and scope creep
	for i := range sim.ActiveTickets {
		ticket := sim.ActiveTickets[i]

		// Bug discovered (2% daily chance, higher for low understanding)
		bugChance := 0.02
		if ticket.UnderstandingLevel == model.LowUnderstanding {
			bugChance = 0.06
		}
		if rng.Float64() < bugChance {
			ticket.RemainingEffort += 0.5 // Half day of rework
			events = append(events, model.NewEvent(
				model.EventBugDiscovered,
				fmt.Sprintf("Bug discovered in %s (+0.5 days)", ticket.ID),
				sim.CurrentTick,
			))
		}

		// Scope creep (1% daily chance)
		if rng.Float64() < 0.01 {
			addition := 0.5 + rng.Float64() // 0.5-1.5 days
			ticket.RemainingEffort += addition
			ticket.EstimatedDays += addition
			events = append(events, model.NewEvent(
				model.EventScopeCreep,
				fmt.Sprintf("Scope creep on %s (+%.1f days)", ticket.ID, addition),
				sim.CurrentTick,
			))
		}

		sim.ActiveTickets[i] = ticket
	}

	// Resolve some open incidents - collect indices to remove
	var resolvedIndices []int
	for i := range sim.OpenIncidents {
		inc := sim.OpenIncidents[i]
		if !inc.IsOpen() {
			continue
		}

		// Resolution probability based on severity and time open
		daysOpen := sim.CurrentTick - inc.CreatedAt.Day()
		resolveChance := 0.3 + float64(daysOpen)*0.1
		if inc.Severity == model.SeverityCritical {
			resolveChance += 0.3
		}

		if rng.Float64() < resolveChance {
			resolved := inc.Resolved()
			sim.ResolvedIncidents = append(sim.ResolvedIncidents, resolved)
			resolvedIndices = append(resolvedIndices, i)
			events = append(events, model.NewEvent(
				model.EventIncidentResolved,
				fmt.Sprintf("Incident %s resolved", inc.ID),
				sim.CurrentTick,
			))
		}
	}

	// Remove resolved incidents (reverse order to preserve indices)
	for i := len(resolvedIndices) - 1; i >= 0; i-- {
		idx := resolvedIndices[i]
		sim.OpenIncidents = append(sim.OpenIncidents[:idx], sim.OpenIncidents[idx+1:]...)
	}

	return events
}

// CheckForIncidents checks recently deployed tickets for production issues
func (g *EventGenerator) CheckForIncidents(sim *model.Simulation) []model.Event {
	events := make([]model.Event, 0)
	rng := rand.New(rand.NewSource(g.seed + int64(sim.CurrentTick)))

	for i := range sim.CompletedTickets {
		ticket := sim.CompletedTickets[i]

		// Only check tickets deployed in last 3 days
		daysSinceDeployed := sim.CurrentTick - ticket.CompletedAt.Day()
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
			incidentID := fmt.Sprintf("INC-%03d", len(sim.OpenIncidents)+len(sim.ResolvedIncidents)+1)
			incident := model.NewIncident(incidentID, ticket.ID, model.Severity(rng.Intn(4)))
			sim.OpenIncidents = append(sim.OpenIncidents, incident)
			ticket.CausedIncident = true
			ticket.IncidentID = incidentID
			sim.CompletedTickets[i] = ticket

			events = append(events, model.NewEvent(
				model.EventIncident,
				fmt.Sprintf("Incident %s: %s caused production issue", incidentID, ticket.ID),
				sim.CurrentTick,
			))
		}
	}

	return events
}

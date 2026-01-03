package engine

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// TicketGenerator creates stochastic ticket backlogs
type TicketGenerator struct {
	// Size distribution (log-normal for realistic skew)
	SizeMean   float64
	SizeStdDev float64

	// Understanding distribution
	LowPct    float64
	MediumPct float64
	HighPct   float64

	// Arrival rate
	TicketsPerSprint int

	// Title pool
	Titles []string
}

// DefaultTitles provides realistic ticket titles
var DefaultTitles = []string{
	"Implement user authentication flow",
	"Fix payment processing bug",
	"Refactor database layer",
	"Add export functionality",
	"Optimize search performance",
	"Update API endpoints",
	"Fix login redirect issue",
	"Add notification system",
	"Migrate to new framework",
	"Implement caching layer",
	"Fix memory leak",
	"Add logging infrastructure",
	"Update user dashboard",
	"Fix race condition",
	"Add rate limiting",
	"Implement webhook support",
	"Fix timezone handling",
	"Add batch processing",
	"Update security policies",
	"Fix validation errors",
}

// Generate creates a batch of tickets with the configured distribution
func (g *TicketGenerator) Generate(rng *rand.Rand, count int) []model.Ticket {
	tickets := make([]model.Ticket, count)

	for i := range tickets {
		// Log-normal distribution for size (right-skewed, realistic)
		size := math.Exp(rng.NormFloat64()*g.SizeStdDev + math.Log(g.SizeMean))
		size = math.Max(0.5, math.Min(size, 20)) // Clamp 0.5-20 days

		// Understanding level based on distribution
		r := rng.Float64()
		var understanding model.UnderstandingLevel
		switch {
		case r < g.LowPct:
			understanding = model.LowUnderstanding
		case r < g.LowPct+g.MediumPct:
			understanding = model.MediumUnderstanding
		default:
			understanding = model.HighUnderstanding
		}

		tickets[i] = model.Ticket{
			ID:                 fmt.Sprintf("TKT-%03d", i+1),
			Title:              g.Titles[rng.Intn(len(g.Titles))],
			EstimatedDays:      size,
			UnderstandingLevel: understanding,
			Phase:              model.PhaseBacklog,
			PhaseEffortSpent:   make(map[model.WorkflowPhase]float64),
			CreatedAt:          time.Now(),
		}
	}

	return tickets
}

// Scenarios provides preset ticket distributions
var Scenarios = map[string]TicketGenerator{
	"healthy": {
		SizeMean: 3, SizeStdDev: 0.5,
		LowPct: 0.20, MediumPct: 0.50, HighPct: 0.30,
		TicketsPerSprint: 12,
		Titles:           DefaultTitles,
	},
	"overloaded": {
		SizeMean: 7, SizeStdDev: 0.8,
		LowPct: 0.40, MediumPct: 0.40, HighPct: 0.20,
		TicketsPerSprint: 15,
		Titles:           DefaultTitles,
	},
	"uncertain": {
		SizeMean: 3, SizeStdDev: 0.5,
		LowPct: 0.60, MediumPct: 0.30, HighPct: 0.10,
		TicketsPerSprint: 12,
		Titles:           DefaultTitles,
	},
	"mixed": {
		SizeMean: 5, SizeStdDev: 1.0,
		LowPct: 0.33, MediumPct: 0.34, HighPct: 0.33,
		TicketsPerSprint: 12,
		Titles:           DefaultTitles,
	},
}

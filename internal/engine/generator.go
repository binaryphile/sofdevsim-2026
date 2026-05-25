package engine

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/binaryphile/fluentfp/slice"
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

	// UC37: type weights for heterogeneous ticket mix. Nil/empty → all-Feature
	// (regression-safe). Weights silently normalised to sum to 1.0; single-type
	// degenerate weights produce 100% that type. Unrecognised type-keys ignored.
	TypeWeights map[model.TicketType]float64
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

// Generate creates a batch of tickets with the configured distribution.
// Value receiver: pure generation, no mutation.
func (g TicketGenerator) Generate(rng *rand.Rand, count int) []model.Ticket {
	tickets := make([]model.Ticket, count)

	// UC37: pre-compute cumulative type-weights (ordered by TicketType iota)
	// for type roll. Nil/empty weights → all-Feature regression-safe default.
	typeOrder := []model.TicketType{
		model.TicketTypeFeature,
		model.TicketTypeBug,
		model.TicketTypeSpike,
		model.TicketTypeMigration,
		model.TicketTypeInfra,
	}
	// /c absorption (FluentFP decision gate): sum is a fold. Use slice.Fold so the
	// accumulator pattern is expressed in domain language (sum reduction) rather
	// than imperative for-loop with mutable typeSum.
	typeSum := slice.Fold(typeOrder, 0.0, func(acc float64, tt model.TicketType) float64 {
		return acc + g.TypeWeights[tt]
	})
	var typeCum []float64
	if typeSum > 0 {
		// Normalise to 1.0; cumulative for lookup. This loop is a scan (fold-with-trail),
		// not a pure fold — each step contributes both to the running total and to the
		// output slice. FluentFP's slice.Fold accumulates a single value; scan would need
		// custom helper. Annotate as IX (index-dependent: order of typeOrder iteration
		// must match the lookup order in the type roll below).
		var acc float64
		for _, tt := range typeOrder { // justified:IX (scan with ordered output)
			acc += g.TypeWeights[tt] / typeSum
			typeCum = append(typeCum, acc)
		}
	}

	for i := range tickets { // justified:IX (parallel-array build; index-based ticket assignment)
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

		// UC37: ticket-type roll. Nil/empty weights → Feature (zero-value default).
		ticketType := model.TicketTypeFeature
		if len(typeCum) > 0 {
			rt := rng.Float64()
			for idx, c := range typeCum { // justified:IX
				if rt < c {
					ticketType = typeOrder[idx]
					break
				}
			}
		}

		tickets[i] = model.Ticket{
			ID:                 fmt.Sprintf("TKT-%03d", i+1),
			Title:              g.Titles[rng.Intn(len(g.Titles))],
			EstimatedDays:      size,
			UnderstandingLevel: understanding,
			Type:               ticketType,
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
	// UC37 typed scenarios (cycle #15442). Existing 4 above remain all-Feature
	// (TypeWeights nil) to preserve their behavioural contracts.
	"uc37-default": {
		SizeMean: 4, SizeStdDev: 0.7,
		LowPct: 0.25, MediumPct: 0.50, HighPct: 0.25,
		TicketsPerSprint: 12,
		Titles:           DefaultTitles,
		TypeWeights: map[model.TicketType]float64{
			model.TicketTypeFeature:   0.60,
			model.TicketTypeBug:       0.25,
			model.TicketTypeSpike:     0.10,
			model.TicketTypeMigration: 0.05,
		},
	},
	"bug-heavy": {
		SizeMean: 3, SizeStdDev: 0.6,
		LowPct: 0.30, MediumPct: 0.45, HighPct: 0.25,
		TicketsPerSprint: 12,
		Titles:           DefaultTitles,
		TypeWeights: map[model.TicketType]float64{
			model.TicketTypeFeature:   0.30,
			model.TicketTypeBug:       0.50,
			model.TicketTypeSpike:     0.05,
			model.TicketTypeMigration: 0.10,
			model.TicketTypeInfra:     0.05,
		},
	},
	"migration-quarter": {
		SizeMean: 5, SizeStdDev: 0.8,
		LowPct: 0.25, MediumPct: 0.50, HighPct: 0.25,
		TicketsPerSprint: 12,
		Titles:           DefaultTitles,
		TypeWeights: map[model.TicketType]float64{
			model.TicketTypeFeature:   0.30,
			model.TicketTypeBug:       0.15,
			model.TicketTypeSpike:     0.05,
			model.TicketTypeMigration: 0.45,
			model.TicketTypeInfra:     0.05,
		},
	},
	"infra-push": {
		SizeMean: 4, SizeStdDev: 0.7,
		LowPct: 0.25, MediumPct: 0.50, HighPct: 0.25,
		TicketsPerSprint: 12,
		Titles:           DefaultTitles,
		TypeWeights: map[model.TicketType]float64{
			model.TicketTypeFeature:   0.35,
			model.TicketTypeBug:       0.15,
			model.TicketTypeSpike:     0.05,
			model.TicketTypeMigration: 0.10,
			model.TicketTypeInfra:     0.35,
		},
	},
	"research-shop": {
		// Spike-heavy; aggregate Research dominates (≈0.48). Designed as
		// contrasting integration-test pair vs uc37-default.
		SizeMean: 4, SizeStdDev: 0.6,
		LowPct: 0.30, MediumPct: 0.50, HighPct: 0.20,
		TicketsPerSprint: 12,
		Titles:           DefaultTitles,
		TypeWeights: map[model.TicketType]float64{
			model.TicketTypeFeature:   0.05,
			model.TicketTypeBug:       0.05,
			model.TicketTypeSpike:     0.85,
			model.TicketTypeMigration: 0.00,
			model.TicketTypeInfra:     0.05,
		},
	},
}

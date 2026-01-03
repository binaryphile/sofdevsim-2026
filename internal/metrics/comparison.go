package metrics

import (
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// ComparisonResult holds the results of comparing two policies
type ComparisonResult struct {
	PolicyA SizingPolicy
	PolicyB SizingPolicy
	Seed    int64

	ResultsA SimulationResult
	ResultsB SimulationResult

	// Per-metric winners
	LeadTimeWinner   SizingPolicy
	DeployFreqWinner SizingPolicy
	MTTRWinner       SizingPolicy
	CFRWinner        SizingPolicy

	// Overall
	OverallWinner SizingPolicy
	WinsA         int
	WinsB         int
}

// SizingPolicy is an alias for cleaner comparison API
type SizingPolicy = model.SizingPolicy

// SimulationResult captures final state of a simulation run
type SimulationResult struct {
	Policy          SizingPolicy
	FinalMetrics    *DORAMetrics
	TicketsComplete int
	IncidentCount   int
	AvgFeverStatus  float64
}

// Compare runs the same scenario under two policies and determines winner
func Compare(resultA, resultB SimulationResult, seed int64) ComparisonResult {
	result := ComparisonResult{
		PolicyA:  resultA.Policy,
		PolicyB:  resultB.Policy,
		Seed:     seed,
		ResultsA: resultA,
		ResultsB: resultB,
	}

	// Compare lead time (lower is better)
	if resultA.FinalMetrics.LeadTimeAvgDays() < resultB.FinalMetrics.LeadTimeAvgDays() {
		result.LeadTimeWinner = resultA.Policy
		result.WinsA++
	} else if resultB.FinalMetrics.LeadTimeAvgDays() < resultA.FinalMetrics.LeadTimeAvgDays() {
		result.LeadTimeWinner = resultB.Policy
		result.WinsB++
	}

	// Compare deploy frequency (higher is better)
	if resultA.FinalMetrics.DeployFrequency > resultB.FinalMetrics.DeployFrequency {
		result.DeployFreqWinner = resultA.Policy
		result.WinsA++
	} else if resultB.FinalMetrics.DeployFrequency > resultA.FinalMetrics.DeployFrequency {
		result.DeployFreqWinner = resultB.Policy
		result.WinsB++
	}

	// Compare MTTR (lower is better)
	if resultA.FinalMetrics.MTTRAvgDays() < resultB.FinalMetrics.MTTRAvgDays() {
		result.MTTRWinner = resultA.Policy
		result.WinsA++
	} else if resultB.FinalMetrics.MTTRAvgDays() < resultA.FinalMetrics.MTTRAvgDays() {
		result.MTTRWinner = resultB.Policy
		result.WinsB++
	}

	// Compare CFR (lower is better)
	if resultA.FinalMetrics.ChangeFailRatePct() < resultB.FinalMetrics.ChangeFailRatePct() {
		result.CFRWinner = resultA.Policy
		result.WinsA++
	} else if resultB.FinalMetrics.ChangeFailRatePct() < resultA.FinalMetrics.ChangeFailRatePct() {
		result.CFRWinner = resultB.Policy
		result.WinsB++
	}

	// Determine overall winner
	if result.WinsA > result.WinsB {
		result.OverallWinner = resultA.Policy
	} else if result.WinsB > result.WinsA {
		result.OverallWinner = resultB.Policy
	}
	// If tied, OverallWinner remains zero value

	return result
}

// IsTie returns true if no clear winner
func (c *ComparisonResult) IsTie() bool {
	return c.WinsA == c.WinsB
}

// WinMargin returns the difference in wins
func (c *ComparisonResult) WinMargin() int {
	if c.WinsA > c.WinsB {
		return c.WinsA - c.WinsB
	}
	return c.WinsB - c.WinsA
}

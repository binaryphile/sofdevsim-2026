package metrics

import "github.com/binaryphile/sofdevsim-2026/internal/model"

// ChildVarianceResult holds decomposition outcome analysis.
type ChildVarianceResult struct {
	ParentID      string
	HighVariance  bool    // any child > 1.3 ratio
	MaxChildRatio float64 // highest child actual/estimate
}

// AnalyzeChildVariance checks if decomposed tickets had high variance.
// Returns results for parents whose children are all completed.
// Calculation: []Ticket → []ChildVarianceResult
func AnalyzeChildVariance(completedTickets []model.Ticket) []ChildVarianceResult {
	// Build lookup map
	byID := make(map[string]model.Ticket)
	for _, t := range completedTickets {
		byID[t.ID] = t
	}

	var results []ChildVarianceResult
	for _, parent := range completedTickets {
		if !parent.HasChildren() {
			continue
		}

		// Check all children completed
		allCompleted := true
		var maxRatio float64
		for _, childID := range parent.ChildIDs {
			child, ok := byID[childID]
			if !ok {
				allCompleted = false
				break
			}
			// Boundary defense: skip if EstimatedDays is zero
			if child.EstimatedDays <= 0 {
				continue
			}
			ratio := child.ActualDays / child.EstimatedDays
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}

		if allCompleted {
			results = append(results, ChildVarianceResult{
				ParentID:      parent.ID,
				HighVariance:  maxRatio > 1.3,
				MaxChildRatio: maxRatio,
			})
		}
	}
	return results
}

// HasHighChildVariance returns true if any decomposed ticket has child ratio > 1.3.
// Calculation: []Ticket → bool
// Note: Uses raw loop instead of fluentfp for early-exit semantics (first match returns).
// This is intentional per FP guide - fluentfp is for transformation pipelines, not early-exit searches.
func HasHighChildVariance(completedTickets []model.Ticket) bool {
	for _, r := range AnalyzeChildVariance(completedTickets) {
		if r.HighVariance {
			return true
		}
	}
	return false
}

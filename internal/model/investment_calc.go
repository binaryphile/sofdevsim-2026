// Pure calculations for UC40 investment moves.
//
// Decision F (FP unified ACD): Calculations layer — pure functions
// over Data values (Budget int + InvestmentOption + InvestmentOptionCost
// map). No I/O, no time dependency, no state mutation. The investment
// controller's Action layer (internal/engine/investment_controller.go)
// orchestrates side effects + emits events.
package model

import (
	"github.com/binaryphile/fluentfp/slice"
)

// ShouldAffordInvestment returns true when the budget is sufficient to
// purchase the option (cost <= budget). Used as a precheck in
// SpendInvestment (commit 8) and as the filter predicate in
// AvailableOptions.
//
// Negative budget is treated as 0 (insufficient for anything except
// free options, which UC40 has none of).
func ShouldAffordInvestment(budget int, option InvestmentOption) bool {
	return InvestmentOptionCost[option] <= budget
}

// AvailableOptions returns the subset of InvestmentOption variants the
// operator can afford given the current Budget. Returns options in
// canonical enum order (Hire, CICDSlot, ReviewTool, VerifyPaydown)
// for deterministic UI rendering + stable test assertions.
//
// Returns an empty slice (not nil) when no options are affordable —
// matches the UC40 ext §1a "Budget exhausted: empty AvailableOptions
// menu" contract.
func AvailableOptions(budget int) []InvestmentOption {
	all := AllInvestmentOptions()
	affordable := func(o InvestmentOption) bool {
		return ShouldAffordInvestment(budget, o)
	}
	return slice.From(all).KeepIf(affordable)
}

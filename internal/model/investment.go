// Investment-move configuration for UC40 (between-sprint investment window).
//
// Co-locates the InvestmentOption iota enum, the 4-option cost map,
// ParseInvestmentOption, and the 3 typed sentinel errors that callers
// use to differentiate investment-spend failure modes via errors.Is
// (Go dev guide §8). UC40 closes the Factorio dynamics program (parent
// epic #15441; UC37+UC38+UC39+UC40 all shipped).
//
// Decision F (FP unified ACD): InvestmentOption + InvestmentOptionCost
// are Data; ParseInvestmentOption is a pure Calculation. The investment
// controller's Action layer lives in internal/engine/investment_controller.go.
package model

import (
	"errors"
	"fmt"
	"strings"
)

// InvestmentOption selects one of UC40's 4 capacity-changing investments.
// Cost is fixed per Phase 1b Decision A (see InvestmentOptionCost below).
type InvestmentOption int

const (
	// InvestHire — adds a new developer with auto-generated ID +
	// default velocity 1.0. Projection handler reuses appendDeveloper
	// helper (shared with DeveloperAdded handler) for the capacity
	// change AND debits Budget atomically (single event per /i pass 1).
	InvestHire InvestmentOption = iota
	// InvestCICDSlot — increments sim.CICDSlots (works with UC38's
	// PhaseWIPCap fallback wiring).
	InvestCICDSlot
	// InvestReviewTool — multiplies sim.ReviewVelocityBonus by 1.2
	// (stacks across investments).
	InvestReviewTool
	// InvestVerifyPaydown — multiplies sim.VerifyVarianceDamping by
	// 0.8 (stacks; lower = less variance).
	InvestVerifyPaydown
)

// String returns the operator-facing kebab-case name for the option.
func (o InvestmentOption) String() string {
	return [...]string{"hire", "cicd-slot", "review-tool", "verify-paydown"}[o]
}

// InvestmentOptionCost is the fixed per-option cost in Budget units.
// Locked per Phase 1b Decision A (Budget=10 starts; lets operator
// afford ~2-3 investments per run).
var InvestmentOptionCost = map[InvestmentOption]int{
	InvestHire:          5,
	InvestCICDSlot:      3,
	InvestReviewTool:    2,
	InvestVerifyPaydown: 2,
}

// AllInvestmentOptions returns the canonical option list in enum order
// (Hire, CICDSlot, ReviewTool, VerifyPaydown). Used by AvailableOptions
// to filter affordable options and by TUI numbered-options rendering.
func AllInvestmentOptions() []InvestmentOption {
	return []InvestmentOption{
		InvestHire,
		InvestCICDSlot,
		InvestReviewTool,
		InvestVerifyPaydown,
	}
}

// Sentinel errors for investment operations. Use errors.Is to
// differentiate; never string-match the message.
var (
	// ErrInsufficientBudget — SpendInvestment called when
	// InvestmentOptionCost[option] > sim.Budget. HTTP 422 from REST.
	ErrInsufficientBudget = errors.New("insufficient budget")

	// ErrInvalidInvestment — ParseInvestmentOption received an
	// unknown option name. HTTP 422 from REST.
	ErrInvalidInvestment = errors.New("invalid investment option")

	// ErrInvestmentWindowClosed — SpendInvestment called when
	// IsInvestmentWindowOpen() returns false (mid-sprint OR before
	// first sprint). HTTP 409 (state conflict) from REST.
	ErrInvestmentWindowClosed = errors.New("investment window closed")
)

// ParseInvestmentOption translates an operator-supplied string to an
// InvestmentOption. Case-insensitive; accepts kebab-case canonical
// names (matches String() output). Returns ErrInvalidInvestment
// (wrapped with the offending input) on unknown values.
//
// Empty string is NOT defaulted (unlike ReleaseMode's parser) — the
// operator must explicitly choose; an empty/missing option is an
// error.
func ParseInvestmentOption(s string) (InvestmentOption, error) {
	switch strings.ToLower(s) {
	case "hire":
		return InvestHire, nil
	case "cicd-slot":
		return InvestCICDSlot, nil
	case "review-tool":
		return InvestReviewTool, nil
	case "verify-paydown":
		return InvestVerifyPaydown, nil
	}
	return 0, fmt.Errorf("%w: %q (valid: hire, cicd-slot, review-tool, verify-paydown)", ErrInvalidInvestment, s)
}

// Investment controller for UC40 between-sprint investment moves.
//
// Action layer per Decision F (FP unified ACD): reads sim state, calls
// pure calculations (model.ShouldAffordInvestment), emits a single
// domain event (InvestmentApplied) whose projection handler does the
// capacity change + Budget debit atomically (per /i pass 1 atomicity
// fix). All side-effecting logic concentrates here; pure calcs live
// in internal/model/investment_calc.go.
//
// Called from:
//   - TUI 'i' / number-key handlers when investment window open
//   - REST POST /simulations/{id}/investments
package engine

import (
	"fmt"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// SpendInvestment debits the Budget by the option's cost and applies the
// capacity change via a single InvestmentApplied event. Returns the
// updated Engine on success.
//
// Validation order (per Decision A + B): window-open first (the
// operator-facing precondition), then affordability (the budget
// precondition). Mismatched window or budget returns the appropriate
// sentinel error; sim state is unchanged.
//
// Returns:
//   - new Engine (immutable pattern)
//   - error: ErrInvestmentWindowClosed (window not open) OR
//     ErrInsufficientBudget (cost > budget) OR nil
func (e Engine) SpendInvestment(option model.InvestmentOption) (Engine, error) {
	state := e.state()

	if !state.IsInvestmentWindowOpen() {
		return e, fmt.Errorf("%w: SprintNumber=%d active=%v",
			model.ErrInvestmentWindowClosed,
			state.SprintNumber,
			state.CurrentSprintOption.IsOk(),
		)
	}

	if !model.ShouldAffordInvestment(state.Budget, option) {
		return e, fmt.Errorf("%w: option=%s cost=%d budget=%d",
			model.ErrInsufficientBudget,
			option.String(),
			model.InvestmentOptionCost[option],
			state.Budget,
		)
	}

	// TargetPhase informational per option (PhaseBacklog = no specific
	// phase target; used for Hire which doesn't target a single phase).
	targetPhase := investmentTargetPhase(option)

	cost := model.InvestmentOptionCost[option]
	evt := events.NewInvestmentApplied(state.ID, state.CurrentTick, option, cost, targetPhase)
	return e.emit(evt)
}

// investmentTargetPhase maps an InvestmentOption to its primary
// targeted phase, for the event's TargetPhase field (informational;
// not used by the projection handler, but useful for CSV export +
// future analytics).
func investmentTargetPhase(option model.InvestmentOption) model.WorkflowPhase {
	switch option {
	case model.InvestCICDSlot:
		return model.PhaseCICD
	case model.InvestReviewTool:
		return model.PhaseReview
	case model.InvestVerifyPaydown:
		return model.PhaseVerify
	}
	// InvestHire: no specific phase target — zero-value (PhaseBacklog)
	return model.PhaseBacklog
}

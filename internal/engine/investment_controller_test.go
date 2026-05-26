package engine_test

import (
	"errors"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// UC40 (#15446): SpendInvestment Action — 6 scenarios per Khorikov
// Controller quadrant. Drives through public Engine surface; assertions
// are output-based on observable state (Budget, CICDSlots, etc.) +
// event-store presence. No internal mocks.

// buildInvestmentEngine constructs an engine in the "investment window
// open" state (1 sprint completed) so happy-path tests can proceed.
func buildInvestmentEngine(t *testing.T) (engine.Engine, events.Store) {
	t.Helper()
	store := events.NewMemoryStore()
	eng := engine.NewEngineWithStore(42, store)
	var err error
	eng, err = eng.EmitCreated("inv-test", 0, events.SimConfig{
		TeamSize:     6,
		SprintLength: 10,
		Seed:         42,
		Policy:       model.PolicyNone,
	})
	if err != nil {
		t.Fatalf("EmitCreated: %v", err)
	}
	// Default-team additions so dev IDs follow the dev-1..dev-6 convention
	// (Hire's projection handler will produce dev-7 from NextDeveloperID).
	for i, id := range []string{"dev-1", "dev-2", "dev-3", "dev-4", "dev-5", "dev-6"} {
		eng, err = eng.AddDeveloper(id, id, 1.0)
		if err != nil {
			t.Fatalf("AddDeveloper %d: %v", i, err)
		}
	}
	// Start + end a sprint via direct events to drive SprintNumber > 0 + window-open state.
	eng, _ = eng.EmitForTest(events.NewSprintStarted("inv-test", 0, 1, 2.0))
	eng, _ = eng.EmitForTest(events.NewSprintEnded("inv-test", 10, 1))
	if !eng.Sim().IsInvestmentWindowOpen() {
		t.Fatalf("setup: window should be open after SprintStarted+SprintEnded; got SprintNumber=%d active=%v",
			eng.Sim().SprintNumber, eng.Sim().CurrentSprintOption.IsOk())
	}
	return eng, store
}

func countInvestmentApplied(store events.Store, simID string) int {
	count := 0
	for _, evt := range store.Replay(simID) { // justified:CF
		if _, ok := evt.(events.InvestmentApplied); ok {
			count++
		}
	}
	return count
}

// Scenario 1: window closed (sim never started a sprint) → ErrInvestmentWindowClosed.
func TestSpendInvestment_WindowClosed_PreFirstSprint(t *testing.T) {
	eng := engine.NewEngine(42)
	var err error
	eng, err = eng.EmitCreated("noprint", 0, events.SimConfig{Seed: 42, Policy: model.PolicyNone})
	if err != nil {
		t.Fatalf("EmitCreated: %v", err)
	}

	_, err = eng.SpendInvestment(model.InvestHire)
	if !errors.Is(err, model.ErrInvestmentWindowClosed) {
		t.Errorf("err = %v; want errors.Is(err, ErrInvestmentWindowClosed)", err)
	}
}

// Scenario 2: insufficient budget → ErrInsufficientBudget.
func TestSpendInvestment_InsufficientBudget(t *testing.T) {
	eng, store := buildInvestmentEngine(t)
	// Manually drain budget below Hire cost (5).
	eng, _ = eng.EmitForTest(events.NewInvestmentApplied("inv-test", 11, model.InvestHire, 5, model.PhaseBacklog))
	eng, _ = eng.EmitForTest(events.NewInvestmentApplied("inv-test", 11, model.InvestReviewTool, 2, model.PhaseReview))
	// Now Budget = 10 - 5 - 2 = 3; Hire costs 5; insufficient.
	if eng.Sim().Budget != 3 {
		t.Fatalf("setup: Budget should be 3 after draining; got %d", eng.Sim().Budget)
	}
	beforeCount := countInvestmentApplied(store, "inv-test")

	_, err := eng.SpendInvestment(model.InvestHire)
	if !errors.Is(err, model.ErrInsufficientBudget) {
		t.Errorf("err = %v; want errors.Is(err, ErrInsufficientBudget)", err)
	}
	// Side-effect-free on failure: no new InvestmentApplied emitted.
	afterCount := countInvestmentApplied(store, "inv-test")
	if afterCount != beforeCount {
		t.Errorf("InvestmentApplied count changed (%d → %d); should be unchanged on rejection",
			beforeCount, afterCount)
	}
}

// Scenario 3 (happy path 1/4): Hire dispatches both Developer append + Budget debit.
func TestSpendInvestment_Hire_HappyPath(t *testing.T) {
	eng, _ := buildInvestmentEngine(t)
	devCountBefore := len(eng.Sim().Developers)
	budgetBefore := eng.Sim().Budget

	var err error
	eng, err = eng.SpendInvestment(model.InvestHire)
	if err != nil {
		t.Fatalf("SpendInvestment(Hire): %v", err)
	}

	st := eng.Sim()
	if st.Budget != budgetBefore-5 {
		t.Errorf("Budget = %d; want %d (-5)", st.Budget, budgetBefore-5)
	}
	if len(st.Developers) != devCountBefore+1 {
		t.Errorf("Developers count = %d; want %d (+1)", len(st.Developers), devCountBefore+1)
	}
	if st.Developers[devCountBefore].ID != "dev-7" {
		t.Errorf("new dev ID = %q; want \"dev-7\" (NextDeveloperID was 7)", st.Developers[devCountBefore].ID)
	}
}

// Scenario 4 (happy path 2/4): CICDSlot increments + debits.
func TestSpendInvestment_CICDSlot_HappyPath(t *testing.T) {
	eng, _ := buildInvestmentEngine(t)
	cicdBefore := eng.Sim().CICDSlots

	var err error
	eng, err = eng.SpendInvestment(model.InvestCICDSlot)
	if err != nil {
		t.Fatalf("SpendInvestment(CICDSlot): %v", err)
	}

	st := eng.Sim()
	if st.Budget != 7 {
		t.Errorf("Budget = %d; want 7 (10 - 3)", st.Budget)
	}
	if st.CICDSlots != cicdBefore+1 {
		t.Errorf("CICDSlots = %d; want %d (+1)", st.CICDSlots, cicdBefore+1)
	}
}

// Scenario 5 (happy path 3/4): ReviewTool multiplies + debits.
func TestSpendInvestment_ReviewTool_HappyPath(t *testing.T) {
	eng, _ := buildInvestmentEngine(t)

	var err error
	eng, err = eng.SpendInvestment(model.InvestReviewTool)
	if err != nil {
		t.Fatalf("SpendInvestment(ReviewTool): %v", err)
	}

	st := eng.Sim()
	if st.Budget != 8 {
		t.Errorf("Budget = %d; want 8 (10 - 2)", st.Budget)
	}
	if st.ReviewVelocityBonus != 1.2 {
		t.Errorf("ReviewVelocityBonus = %v; want 1.2", st.ReviewVelocityBonus)
	}
}

// Scenario 6 (happy path 4/4): VerifyPaydown dampens + debits.
func TestSpendInvestment_VerifyPaydown_HappyPath(t *testing.T) {
	eng, _ := buildInvestmentEngine(t)

	var err error
	eng, err = eng.SpendInvestment(model.InvestVerifyPaydown)
	if err != nil {
		t.Fatalf("SpendInvestment(VerifyPaydown): %v", err)
	}

	st := eng.Sim()
	if st.Budget != 8 {
		t.Errorf("Budget = %d; want 8 (10 - 2)", st.Budget)
	}
	if st.VerifyVarianceDamping != 0.8 {
		t.Errorf("VerifyVarianceDamping = %v; want 0.8", st.VerifyVarianceDamping)
	}
}

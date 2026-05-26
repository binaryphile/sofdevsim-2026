package engine_test

import (
	"testing"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// UC40 (#15446): capacity-multiplier wiring tests. Assert that
// ReviewVelocityBonus and VerifyVarianceDamping are READ by the
// engine work-calc when the corresponding phase is active, and that
// non-default values cause observable change in work-done values.
//
// Approach: build TWO engines with identical setup except for the
// multiplier value (default 1.0 vs investment-modified). Drive 1 Tick
// with a ticket in the target phase. Assert the PhaseEffortSpent
// differs by the expected ratio. Identity-at-default is implied by
// the entire pre-UC40 test suite continuing to pass.

func setupSinglePhaseTicket(t *testing.T, phase model.WorkflowPhase, applyMultiplier func(engine.Engine) engine.Engine) float64 {
	t.Helper()
	eng := engine.NewEngine(42)
	var err error
	eng, err = eng.EmitCreated("wire", 0, events.SimConfig{
		TeamSize: 1, SprintLength: 10, Seed: 42, Policy: model.PolicyNone,
	})
	if err != nil {
		t.Fatalf("EmitCreated: %v", err)
	}
	eng, _ = eng.AddDeveloper("dev-a", "Alice", 1.0)
	eng, _ = eng.AddTicket(model.NewTicket("TKT-1", "test", 3, model.HighUnderstanding))
	eng, _ = eng.AssignTicket("TKT-1", "dev-a")

	// Apply the multiplier (if test wants non-default) BEFORE moving ticket to phase.
	eng = applyMultiplier(eng)

	// Force the ticket into the target phase via direct events.
	eng, _ = eng.EmitForTest(events.NewTicketQueued("wire", 0, "TKT-1", phase, ""))
	eng, _ = eng.EmitForTest(events.NewTicketAssigned("wire", 0, "TKT-1", "dev-a", phase, time.Time{}))

	eng, _, _ = eng.Tick()

	st := eng.Sim()
	idx := st.FindActiveTicketIndex("TKT-1")
	if idx != -1 {
		return st.ActiveTickets[idx].PhaseEffortSpent[phase]
	}
	for _, c := range st.CompletedTickets {
		if c.ID == "TKT-1" {
			return c.PhaseEffortSpent[phase]
		}
	}
	t.Fatalf("ticket TKT-1 not found post-Tick")
	return 0
}

func TestReviewVelocityBonus_AppliesToReviewPhase(t *testing.T) {
	// Baseline: no investment → ReviewVelocityBonus = 1.0 (identity).
	baseline := setupSinglePhaseTicket(t, model.PhaseReview, func(e engine.Engine) engine.Engine {
		return e
	})
	// Boosted: one ReviewTool investment → ReviewVelocityBonus = 1.2.
	boosted := setupSinglePhaseTicket(t, model.PhaseReview, func(e engine.Engine) engine.Engine {
		e, _ = e.EmitForTest(events.NewInvestmentApplied("wire", 0, model.InvestReviewTool, 2, model.PhaseReview))
		return e
	})

	if baseline == 0 || boosted == 0 {
		t.Fatalf("setup: zero work done (baseline=%v boosted=%v); test produces no signal", baseline, boosted)
	}
	// boosted should be ~1.2× baseline (velocity factor; variance + experience
	// multipliers are identical across both runs since seed is fixed).
	ratio := boosted / baseline
	if ratio < 1.18 || ratio > 1.22 {
		t.Errorf("boosted/baseline = %v; want ~1.2 (ReviewVelocityBonus multiplier; baseline=%v boosted=%v)",
			ratio, baseline, boosted)
	}
}

func TestVerifyVarianceDamping_AppliesToVerifyPhase(t *testing.T) {
	// Baseline: no investment → VerifyVarianceDamping = 1.0 (identity).
	baseline := setupSinglePhaseTicket(t, model.PhaseVerify, func(e engine.Engine) engine.Engine {
		return e
	})
	// Dampened: one VerifyPaydown investment → VerifyVarianceDamping = 0.8.
	dampened := setupSinglePhaseTicket(t, model.PhaseVerify, func(e engine.Engine) engine.Engine {
		e, _ = e.EmitForTest(events.NewInvestmentApplied("wire", 0, model.InvestVerifyPaydown, 2, model.PhaseVerify))
		return e
	})

	if baseline == 0 || dampened == 0 {
		t.Fatalf("setup: zero work done (baseline=%v dampened=%v); test produces no signal", baseline, dampened)
	}
	// dampened should be ~0.8× baseline (variance damping; "less variance from
	// work" means work done is lower because variance multiplies workDone).
	ratio := dampened / baseline
	if ratio < 0.78 || ratio > 0.82 {
		t.Errorf("dampened/baseline = %v; want ~0.8 (VerifyVarianceDamping multiplier; baseline=%v dampened=%v)",
			ratio, baseline, dampened)
	}
}

// Identity test: with default multipliers (1.0/1.0), behavior is byte-
// identical to pre-UC40 — verified by all existing engine tests
// continuing to pass. This unit test documents the contract explicitly.
func TestCapacityMultipliers_DefaultIsIdentity(t *testing.T) {
	eng := engine.NewEngine(42)
	var err error
	eng, err = eng.EmitCreated("identity", 0, events.SimConfig{Seed: 42, Policy: model.PolicyNone})
	if err != nil {
		t.Fatalf("EmitCreated: %v", err)
	}
	st := eng.Sim()
	if st.ReviewVelocityBonus != 1.0 {
		t.Errorf("ReviewVelocityBonus default = %v; want 1.0 (identity)", st.ReviewVelocityBonus)
	}
	if st.VerifyVarianceDamping != 1.0 {
		t.Errorf("VerifyVarianceDamping default = %v; want 1.0 (identity)", st.VerifyVarianceDamping)
	}
}

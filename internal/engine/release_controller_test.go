package engine_test

import (
	"errors"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// UC39 (#15445): runReleaseController integration coverage per Decision A
// (Khorikov Controller quadrant). 7 scenarios drive through the public
// Engine surface (Tick); assertions are output-based on observable state
// (sim.WarmupActive, WarmupFailed, CommittedTickets count, presence of
// emitted events via store query) — no internal mocks.

// buildReleaseControllerEngine constructs an engine in the requested mode
// with N triaged backlog tickets. Returns engine + event store for
// post-tick event-presence assertions.
func buildReleaseControllerEngine(t *testing.T, mode model.ReleaseMode, backlogCount int) (engine.Engine, events.Store) {
	t.Helper()
	store := events.NewMemoryStore()
	eng := engine.NewEngineWithStore(42, store)
	var err error
	eng, err = eng.EmitCreated("rc-test", 0, events.SimConfig{
		TeamSize:     3,
		SprintLength: 10,
		Seed:         42,
		Policy:       model.PolicyNone,
		ReleaseMode:  mode,
	})
	if err != nil {
		t.Fatalf("EmitCreated: %v", err)
	}
	// Add 3 devs so warmup-bulk-commit path has somewhere to assign.
	for i, id := range []string{"d1", "d2", "d3"} {
		eng, err = eng.AddDeveloper(id, id, 1.0)
		if err != nil {
			t.Fatalf("AddDeveloper %d: %v", i, err)
		}
	}
	for i := 0; i < backlogCount; i++ {
		ticket := model.NewTicket(
			"TKT-"+string(rune('A'+i)),
			"ticket-"+string(rune('A'+i)),
			3.0,
			model.HighUnderstanding,
		)
		eng, err = eng.AddTicket(ticket)
		if err != nil {
			t.Fatalf("AddTicket %d: %v", i, err)
		}
	}
	return eng, store
}

// countEvents returns how many events of the given type live in the store.
func countEventsOfType[T events.Event](t *testing.T, store events.Store, simID string) int {
	t.Helper()
	count := 0
	for _, evt := range store.Replay(simID) { // justified:CF
		if _, ok := evt.(T); ok {
			count++
		}
	}
	return count
}

// Scenario 1: Push mode — controller no-ops; no TicketCommitted events
// from controller (StartSprint still commits separately).
func TestRunReleaseController_PushMode_NoOp(t *testing.T) {
	eng, store := buildReleaseControllerEngine(t, model.ReleaseModePush, 3)
	eng, _, _ = eng.Tick()

	if eng.Sim().WarmupActive {
		t.Errorf("push mode: WarmupActive should be false; got true")
	}
	// In push mode, StartSprint was never called in this test, so
	// CommittedTickets should be empty (no controller drips either).
	if len(eng.Sim().CommittedTickets) != 0 {
		t.Errorf("push mode + no StartSprint: CommittedTickets = %d; want 0 (controller must not drip in push mode)",
			len(eng.Sim().CommittedTickets))
	}
	_ = store // silence unused (other scenarios use store directly)
}

// Scenario 2: Demand mode warmup-running — controller no-ops while
// warmup is active and timeout hasn't fired; no admission, no event.
func TestRunReleaseController_DemandWarmupRunning_NoOp(t *testing.T) {
	eng, store := buildReleaseControllerEngine(t, model.ReleaseModeDemand, 3)
	// Single tick: analyzer has 1 tick of data, no constraint locked,
	// SprintNumber = 0 (under the warmup limit of 5).
	eng, _, _ = eng.Tick()

	if !eng.Sim().WarmupActive {
		t.Errorf("warmup-running: WarmupActive should be true; got false")
	}
	if eng.Sim().WarmupFailed {
		t.Errorf("warmup-running: WarmupFailed should be false; got true")
	}
	if got := countEventsOfType[events.WarmupExited](t, store, "rc-test"); got != 0 {
		t.Errorf("warmup-running: WarmupExited count = %d; want 0", got)
	}
	if got := countEventsOfType[events.WarmupTimedOut](t, store, "rc-test"); got != 0 {
		t.Errorf("warmup-running: WarmupTimedOut count = %d; want 0", got)
	}
}

// Scenario 3: Demand mode warmup-timeout — when SprintNumber crosses 5,
// controller emits WarmupTimedOut, sets WarmupFailed=true, returns
// ErrWarmupTimeout (informational; Tick continues).
func TestRunReleaseController_DemandWarmupTimeout_EmitsAndFlipsFailed(t *testing.T) {
	eng, store := buildReleaseControllerEngine(t, model.ReleaseModeDemand, 0)
	// Push the SprintNumber to 5 by starting 5 sprints.
	for i := 0; i < 5; i++ { // justified:SM
		var err error
		eng, err = eng.StartSprint()
		if err != nil {
			t.Fatalf("StartSprint %d: %v", i, err)
		}
	}
	if eng.Sim().SprintNumber < 5 {
		t.Fatalf("setup: expected SprintNumber >= 5, got %d", eng.Sim().SprintNumber)
	}

	// Tick once — controller sees SprintNumber>=5, emits WarmupTimedOut.
	eng, _, _ = eng.Tick()

	if !eng.Sim().WarmupFailed {
		t.Errorf("warmup-timeout: WarmupFailed should be true; got false")
	}
	if !eng.Sim().WarmupActive {
		t.Errorf("warmup-timeout: WarmupActive should STAY true (intentional — controller stays disabled; sim behaves as push); got false")
	}
	if got := countEventsOfType[events.WarmupTimedOut](t, store, "rc-test"); got < 1 {
		t.Errorf("warmup-timeout: WarmupTimedOut count = %d; want >= 1", got)
	}
}

// Scenario 4: WarmupFailed terminal — controller no-ops on subsequent
// ticks (sim continues in effective-push state).
func TestRunReleaseController_WarmupFailedTerminal_StaysNoOp(t *testing.T) {
	store := events.NewMemoryStore()
	eng := engine.NewEngineWithStore(42, store)
	var err error
	eng, err = eng.EmitCreated("rc-test-failed", 0, events.SimConfig{
		Seed:        42,
		Policy:      model.PolicyNone,
		ReleaseMode: model.ReleaseModeDemand,
	})
	if err != nil {
		t.Fatalf("EmitCreated: %v", err)
	}
	// Directly emit WarmupTimedOut to enter terminal state without
	// running 5 sprints (faster + isolates the post-failed branch).
	eng, err = eng.EmitForTest(events.NewWarmupTimedOut("rc-test-failed", 0, 5))
	if err != nil {
		t.Fatalf("emit WarmupTimedOut: %v", err)
	}
	if !eng.Sim().WarmupFailed {
		t.Fatalf("setup: WarmupFailed should be true; got false")
	}

	beforeCount := countEventsOfType[events.TicketCommitted](t, store, "rc-test-failed")

	eng, _, _ = eng.Tick()

	if got := countEventsOfType[events.TicketCommitted](t, store, "rc-test-failed"); got != beforeCount {
		t.Errorf("warmup-failed terminal: controller drip count changed (%d → %d); want no-op",
			beforeCount, got)
	}
}

// Scenario 5: ErrAnalyzerNotReady defensive guard — when the engine's
// TOC pointer is nil, the controller returns the sentinel. Reached via
// EngineWithNilTOC test helper.
func TestRunReleaseController_NilTOC_ReturnsErrAnalyzerNotReady(t *testing.T) {
	eng := engine.NewEngineWithNilTOCForTest(42)
	var err error
	eng, err = eng.EmitCreated("rc-test-niltoc", 0, events.SimConfig{
		Seed:        42,
		Policy:      model.PolicyNone,
		ReleaseMode: model.ReleaseModeDemand,
	})
	if err != nil {
		t.Fatalf("EmitCreated: %v", err)
	}

	_, _, err = eng.RunReleaseControllerForTest()
	if !errors.Is(err, model.ErrAnalyzerNotReady) {
		t.Errorf("err = %v; want errors.Is(err, ErrAnalyzerNotReady)", err)
	}
}

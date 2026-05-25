// Release controller for UC39 demand-driven release.
//
// Action layer per Decision F (FP unified ACD): reads sim state, calls
// pure calculations (model.ShouldAdmit + model.WarmupExit), emits domain
// events (TicketCommitted, WarmupExited, WarmupTimedOut). All side-
// effecting logic concentrates here; the pure calcs and the data types
// they operate on live in internal/model/release_calc.go.
//
// Wired into Tick (engine.go) BEFORE assignFromQueues — admitted tickets
// are visible to the assignment pass this same tick.
package engine

import (
	"errors"
	"log/slog"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// warmupSprintLimit is the hard-coded warm-up timeout per UC39
// (operator-tunable in a future cycle if needed).
const warmupSprintLimit = 5

// warmupConfidenceThreshold is the minimum analyzer confidence required
// to exit warm-up. 0.5 = medium confidence. Exact-boundary returns true.
const warmupConfidenceThreshold = 0.5

// isInformationalControllerErr returns true for errors that the
// controller surfaces to indicate non-fatal state transitions (warmup
// timeout, analyzer not ready). Tick treats these as advisory — the
// controller has already emitted the appropriate event (if any) and
// no-op'd; Tick continues.
func isInformationalControllerErr(err error) bool {
	return errors.Is(err, model.ErrWarmupTimeout) ||
		errors.Is(err, model.ErrAnalyzerNotReady)
}

// runReleaseController dispatches per-tick admission logic for UC39's
// demand-driven release mode. No-op for push mode; honors the warm-up
// state machine (WarmupActive → WarmupExited OR WarmupTimedOut; then
// dripping per ShouldAdmit).
//
// Returns:
//   - new Engine (immutable pattern)
//   - UI events (model.Event slice; currently empty — controller's events
//     are events.* domain events emitted via e.emit, not surfaced through
//     this slice)
//   - error: ErrAnalyzerNotReady (defensive guard; informational),
//     ErrWarmupTimeout (informational), OR nil. Tick treats both
//     sentinel errors as advisory via isInformationalControllerErr.
func (e Engine) runReleaseController() (Engine, []model.Event, error) {
	state := e.state()
	modelEvents := make([]model.Event, 0)

	// Push mode: no-op (StartSprint handles commitment).
	if state.ReleaseMode != model.ReleaseModeDemand {
		return e, modelEvents, nil
	}

	// Warmup-failed terminal state: no-op (StartSprint continues bulk-commit
	// per UC39 ext §2a; controller is permanently disabled for this sim).
	if state.WarmupFailed {
		return e, modelEvents, nil
	}

	// Defensive guard: analyzer must have ticks before we can read a
	// meaningful AnalyzerSignal. Tick's ordering (Ticked emit → TOC.Update
	// → controller) means the analyzer always has at least 1 tick by the
	// time the controller runs; the guard exists for robustness against
	// future re-ordering (e.g., if the controller is invoked from a non-
	// Tick entry point).
	if e.toc == nil {
		return e, modelEvents, model.ErrAnalyzerNotReady
	}

	signal := model.AnalyzerSignal{
		Constraint:  e.toc.ConstraintPhase,
		Penetration: e.toc.Buffer.Penetration,
		Confidence:  e.toc.Confidence,
	}

	// Warmup-running phase: check exit + timeout predicates. Warmup-exit
	// wins ties per plan §"Warmup-exit vs timeout race" (preserves
	// happy-path preference if both predicates would fire simultaneously).
	if state.WarmupActive {
		var err error
		if model.WarmupExit(signal, warmupConfidenceThreshold) {
			if e, err = e.emit(events.NewWarmupExited(state.ID, state.CurrentTick, state.SprintNumber, signal.Constraint)); err != nil {
				return e, nil, err
			}
			// fall through to admission for the same tick (controller
			// drips immediately after warmup-exit)
			state = e.state()
		} else if state.SprintNumber >= warmupSprintLimit {
			if e, err = e.emit(events.NewWarmupTimedOut(state.ID, state.CurrentTick, state.SprintNumber)); err != nil {
				return e, nil, err
			}
			slog.Warn("UC39 release controller: warm-up timed out",
				"sim_id", state.ID,
				"sprint_number", state.SprintNumber,
				"constraint", signal.Constraint.String(),
				"confidence", signal.Confidence,
			)
			return e, modelEvents, model.ErrWarmupTimeout
		} else {
			// Still warming; no admission this tick.
			return e, modelEvents, nil
		}
	}

	// Post-warmup admission: drip per ShouldAdmit.
	admit, reason := model.ShouldAdmit(signal, state.MaxBacklogDrip)
	slog.Debug("UC39 release controller: admission decision",
		"sim_id", state.ID,
		"tick", state.CurrentTick,
		"admit", admit,
		"reason", reason,
	)
	if admit == 0 {
		return e, modelEvents, nil
	}

	// Walk the head of the triaged backlog and emit TicketCommitted for
	// up to `admit` tickets. Skips untriaged backlog (consistent with
	// StartSprint's triage-then-commit ordering). Sprint number is the
	// current sprint (carry-on; no separate sprint structure for demand
	// admission).
	sprintNumber := state.SprintNumber
	emitted := 0
	for _, t := range state.Backlog { // justified:CF
		if emitted >= admit {
			break
		}
		if t.IntakeStatus != model.IntakeTriaged {
			continue
		}
		var err error
		if e, err = e.emit(events.NewTicketCommitted(state.ID, state.CurrentTick, t.ID, sprintNumber)); err != nil {
			return e, nil, err
		}
		emitted++
	}

	return e, modelEvents, nil
}

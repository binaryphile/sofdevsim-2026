// Pure calculations for UC39 demand-driven release.
//
// AnalyzerSignal is a value-object snapshot of the TOC analyzer's
// constraint-identification state — the inputs to ShouldAdmit and
// WarmupExit. Decoupling these calcs from internal/metrics.Tracker
// follows Decision A + B (Khorikov): pure functions are unit-tested
// against value-object inputs; the release controller is the Action
// that builds an AnalyzerSignal from the live Tracker and invokes the
// calcs (commit 8).
//
// Decision F (FP unified ACD): this file is Calculations only. No I/O,
// no time dependency, no state mutation. The controller (Action layer)
// orchestrates side effects.
package model

import (
	"fmt"
	"math"
)

// AnalyzerSignal carries the constraint-identification snapshot from
// TOCState that UC39's calcs read. Value type; no pointers — passed by
// value to ShouldAdmit and WarmupExit.
type AnalyzerSignal struct {
	// Constraint is the phase the TOC analyzer has currently identified
	// as the bottleneck. PhaseBacklog (zero-value) signals "no constraint
	// locked yet" — WarmupExit returns false in that case regardless of
	// Confidence.
	Constraint WorkflowPhase
	// Penetration ∈ [0, 1]; 0.0 = constraint buffer full (most headroom),
	// 1.0 = buffer empty (no headroom; red zone). Drives ShouldAdmit's
	// headroom formula.
	Penetration float64
	// Confidence ∈ [0, 1]; analyzer's certainty in the Constraint field.
	// WarmupExit fires when Confidence >= threshold (default 0.5 for
	// medium-confidence lock).
	Confidence float64
}

// ShouldAdmit decides how many backlog tickets the demand controller
// may admit this tick. Demand-mode-only — the controller decides whether
// to invoke based on sim.ReleaseMode (Decision A: don't pass mode to
// avoid testing dead branches in push-mode path).
//
// Headroom formula: floor((1 - Penetration) * drip). When fully green
// (Penetration=0), admits drip tickets per tick. When fully red
// (Penetration=1), throttles to 0 per UC39 ext §3a.
//
// Returns the admit count + a diagnostic reason string that the
// controller slog.Debug-logs for operator visibility (not surfaced in
// TUI or CSV).
func ShouldAdmit(signal AnalyzerSignal, drip int) (int, string) {
	headroom := int(math.Floor((1.0 - signal.Penetration) * float64(drip)))
	if headroom <= 0 {
		return 0, fmt.Sprintf("red-zone throttle: penetration=%.2f", signal.Penetration)
	}
	return headroom, fmt.Sprintf("admit %d (penetration=%.2f, drip=%d)", headroom, signal.Penetration, drip)
}

// WarmupExit returns true when the TOC analyzer has locked a constraint
// with at least the given confidence. Pre-warmup-exit, the demand
// controller no-ops (sim behaves as push for the warmup window).
//
// Boundary semantics: Confidence == threshold returns true (exact-
// boundary inclusion; documented contract per plan §state machine).
func WarmupExit(signal AnalyzerSignal, threshold float64) bool {
	return signal.Constraint != PhaseBacklog && signal.Confidence >= threshold
}

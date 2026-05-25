// Release-mode configuration for UC39 (demand-driven release).
//
// Co-locates the ReleaseMode iota enum, ParseReleaseMode, and the 3
// typed sentinel errors that callers use to differentiate release-mode
// failure modes via errors.Is (Go dev guide §8; anti-pattern absorbed
// in UC37 /c at 9c3cd5e).
//
// Decision F (FP unified ACD): ReleaseMode is Data (immutable per-sim
// in UC39); ParseReleaseMode is a pure Calculation. The release
// controller's Action layer lives in internal/engine/release_controller.go.
package model

import (
	"errors"
	"fmt"
	"strings"
)

// ReleaseMode selects the release-controller behavior for a simulation.
// Zero-value ReleaseModePush is regression-safe — pre-UC39 simulations
// (SimulationCreated pre-v3 events) decode here.
type ReleaseMode int

const (
	// ReleaseModePush is the pre-UC39 commit-then-flow behavior;
	// StartSprint commits to capacity. Zero-value default.
	ReleaseModePush ReleaseMode = iota
	// ReleaseModeDemand is UC39's pull-mode behavior; StartSprint
	// skips bulk-commit (after warmup) and the release controller
	// drips per tick based on constraint-buffer headroom.
	ReleaseModeDemand
)

func (m ReleaseMode) String() string {
	return [...]string{"push", "demand"}[m]
}

// Sentinel errors for release-mode operations. Use errors.Is to
// differentiate; never string-match the message.
var (
	// ErrInvalidReleaseMode — ParseReleaseMode received an unknown
	// mode name. Surfaces as HTTP 422 from REST handlers (UC38-
	// introduced status code for domain-rule violations).
	ErrInvalidReleaseMode = errors.New("invalid release mode")

	// ErrAnalyzerNotReady — runReleaseController called when the
	// TOCState has no analyzer ticks yet. Defensive guard with a real
	// trigger site (controller checks tracker.TOCState.Ticks == 0);
	// reachable when controller runs before first analyzer tick.
	ErrAnalyzerNotReady = errors.New("TOC analyzer not ready")

	// ErrWarmupTimeout — runReleaseController returns this error
	// (non-fatal; informational) alongside emitting the WarmupTimedOut
	// event when warm-up exceeds N=5 sprints without locking a
	// constraint. Test code uses errors.Is to assert the timeout
	// pathway fired.
	ErrWarmupTimeout = errors.New("warm-up exceeded sprint limit without locking constraint")
)

// ParseReleaseMode translates an operator-supplied string to a
// ReleaseMode. Case-insensitive; accepts "push" and "demand".
// Empty string is treated as "push" (zero-value default; regression-safe).
// Returns ErrInvalidReleaseMode (wrapped with the offending input) on
// unknown values.
func ParseReleaseMode(s string) (ReleaseMode, error) {
	switch strings.ToLower(s) {
	case "", "push":
		return ReleaseModePush, nil
	case "demand":
		return ReleaseModeDemand, nil
	}
	return ReleaseModePush, fmt.Errorf("%w: %q (valid: push, demand)", ErrInvalidReleaseMode, s)
}

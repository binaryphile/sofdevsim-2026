// Per-phase WIP cap configuration for UC38.
//
// Co-locates the 4 typed sentinel errors with the ValidatePhaseWIPConfig
// function that produces them. Per Go dev guide §8, callers MUST use
// errors.Is() to differentiate; string-matching the message is forbidden
// (anti-pattern absorbed in UC37 /c pass at 9c3cd5e).
//
// Decision B (Phase 1 upfront guide load): per-violation sentinel; the
// wrapped fmt.Errorf carries operator-diagnostic context (phase, cap).
// Decision C (FP unified ACD): ValidatePhaseWIPConfig is a pure
// Calculation; PhaseWIPConfig is immutable Data per UC38 (UC40 may
// later convert to event-sourced).
package model

import (
	"errors"
	"fmt"
)

// Sentinel errors for per-phase WIP configuration. Use errors.Is to
// differentiate; never string-match the message.
var (
	// ErrCapZero — any cap = 0; would deadlock that phase.
	ErrCapZero = errors.New("phase WIP cap is zero")

	// ErrCapNegative — any cap < 0; semantic error.
	ErrCapNegative = errors.New("phase WIP cap is negative")

	// ErrCapBelowMentorMin — PhaseImplement cap < 2; a mentored junior
	// dev needs the mentor concurrently, so Implement < 2 is infeasible.
	ErrCapBelowMentorMin = errors.New("phase WIP cap below mentor-pair minimum")

	// ErrCapConflict — per-phase cap on any phase in Implement..Review
	// span exceeds RopeConfig.MaxWIP when RopeConfig.Enabled; impossible
	// to satisfy both simultaneously.
	ErrCapConflict = errors.New("phase WIP cap conflicts with aggregate rope ceiling")
)

// mentorPairMinimum is the lower bound for PhaseImplement under the
// existing mentor-pair semantics.
const mentorPairMinimum = 2

// ValidatePhaseWIPConfig validates a per-phase WIP cap configuration.
// Returns the first-violation sentinel wrapped with fmt.Errorf carrying
// the failing phase and cap value for operator diagnostics. Callers use
// errors.Is to differentiate.
//
// Decision A (Khorikov Domain quadrant): pure function; heavy unit
// coverage via table-driven tests over all 4 violation classes.
//
// Iteration order across the map is non-deterministic by Go's map
// semantics; tests construct single-violation cases so the first-found
// match is deterministic per case.
func ValidatePhaseWIPConfig(cfg map[WorkflowPhase]int, rope RopeConfig) error {
	for phase, cap := range cfg {
		switch {
		case cap == 0:
			return fmt.Errorf("%w: phase=%s cap=%d", ErrCapZero, phase, cap)
		case cap < 0:
			return fmt.Errorf("%w: phase=%s cap=%d", ErrCapNegative, phase, cap)
		}
		if phase == PhaseImplement && cap < mentorPairMinimum {
			return fmt.Errorf("%w: phase=%s cap=%d minimum=%d",
				ErrCapBelowMentorMin, phase, cap, mentorPairMinimum)
		}
		if rope.Enabled && rope.MaxWIP > 0 && IsRopeControlledPhase(phase) && cap > rope.MaxWIP {
			return fmt.Errorf("%w: phase=%s cap=%d ropeMaxWIP=%d",
				ErrCapConflict, phase, cap, rope.MaxWIP)
		}
	}
	return nil
}

// PhaseWIPCap and PhaseWIPCount methods on Simulation land in
// simulation.go (alongside the PhaseWIPConfig field that backs them).

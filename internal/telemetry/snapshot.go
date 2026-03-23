// Package telemetry exports simulation metrics via OpenTelemetry.
// Two-layer design: async observable gauges for state, counters+histograms for events.
package telemetry

// Snapshot captures simulation state for thread-safe export.
// Created per tick by the sim's single-threaded loop. Plain scalars only —
// no pointers, no shared backing arrays, safe for concurrent reads.
type Snapshot struct {
	SimID  string
	SimDay int
	Active bool

	// Constraint identification
	ConstraintPhase      string  // phase name or "" if none
	ConstraintConfidence float64 // 0.0-1.0

	// Constraint buffer
	BufferDepth       int     // tickets queued in front of constraint
	BufferPenetration float64 // 0.0 = full, 1.0 = empty

	// Downstream WIP
	DownstreamWIP int

	// Per-phase flow diagnostics (keyed by phase name string)
	PhaseQueueAvg map[string]float64
	PhaseMaxAge   map[string]int

	// Sprint fever
	SprintBufferRatio float64
	SprintFever       string // "Green"/"Yellow"/"Red"
}

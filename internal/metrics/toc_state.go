package metrics

import (
	"sort"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// TOCMode controls which detection method is used.
type TOCMode int

const (
	TOCFlow     TOCMode = iota // default: dwell time + queue aging
	TOCAnalyzer                // core.Analyzer with honest signals only (experimental)
	TOCBoth                    // flow + analyzer, separate outputs
)

// FlowDiagnosis holds per-phase flow metrics from the rolling window.
type FlowDiagnosis struct {
	PhaseDwellMedian map[model.WorkflowPhase]float64 // median dwell time per phase
	PhaseMaxAge      map[model.WorkflowPhase]int      // oldest in-flight ticket age
	PhaseQueueAvg    map[model.WorkflowPhase]float64  // time-weighted avg queue depth
	PhaseArrivals    map[model.WorkflowPhase]int      // total arrivals in window
	PhaseDepartures  map[model.WorkflowPhase]int      // total departures in window
}

// TOCState tracks TOC flow diagnostics via a rolling window over simulation ticks.
type TOCState struct {
	Mode       TOCMode
	WindowSize int // rolling window size in ticks (default 10)

	// Ring buffer of per-tick data
	ring    []*TickData
	ringPos int
	ringLen int

	// Snapshot for state diffing
	prevSnap PhaseSnapshot

	// Hysteresis for constraint identification
	candidate    model.WorkflowPhase
	consecutiveN int

	// Output
	Flow            FlowDiagnosis
	ConstraintPhase model.WorkflowPhase // 0 = no constraint identified
	Confidence      float64
}

const (
	defaultWindowSize       = 10
	hysteresisCount         = 10  // consecutive evaluations to confirm
	dominanceMargin         = 0.1 // 10% lead required
	minDepartures           = 2   // minimum samples for phase to be ranked
)

// NewTOCState creates a TOC state with the given mode and window size.
func NewTOCState(mode TOCMode, windowSize int) *TOCState {
	if windowSize <= 0 {
		windowSize = defaultWindowSize
	}
	return &TOCState{
		Mode:       mode,
		WindowSize: windowSize,
		ring:       make([]*TickData, windowSize),
		Flow: FlowDiagnosis{
			PhaseDwellMedian: make(map[model.WorkflowPhase]float64),
			PhaseMaxAge:      make(map[model.WorkflowPhase]int),
			PhaseQueueAvg:    make(map[model.WorkflowPhase]float64),
			PhaseArrivals:    make(map[model.WorkflowPhase]int),
			PhaseDepartures:  make(map[model.WorkflowPhase]int),
		},
	}
}

// Update processes one tick of simulation state. Call from Tracker.Updated().
func (s *TOCState) Update(sim model.Simulation) {
	currSnap := NewPhaseSnapshot(sim)

	// First tick: just record snapshot, no diff possible
	if len(s.prevSnap.TicketPhases) == 0 && len(s.prevSnap.DevTickets) == 0 {
		s.prevSnap = currSnap
		return
	}

	// Collect tick data from state diff
	td := CollectTickData(s.prevSnap, currSnap, sim.CurrentTick, sim)

	// Add in-flight ages (current snapshot, not diffed)
	// Not stored in ring — computed fresh each tick from current state
	s.prevSnap = currSnap

	// Push to ring buffer
	s.ring[s.ringPos] = td
	s.ringPos = (s.ringPos + 1) % s.WindowSize
	if s.ringLen < s.WindowSize {
		s.ringLen++
	}

	// Compute rolling window metrics
	s.computeFlowDiagnosis(sim)

	// Identify constraint
	s.identifyConstraint()
}

// computeFlowDiagnosis aggregates the rolling window into flow metrics.
func (s *TOCState) computeFlowDiagnosis(sim model.Simulation) {
	// Reset
	s.Flow.PhaseDwellMedian = make(map[model.WorkflowPhase]float64)
	s.Flow.PhaseMaxAge = make(map[model.WorkflowPhase]int)
	s.Flow.PhaseQueueAvg = make(map[model.WorkflowPhase]float64)
	s.Flow.PhaseArrivals = make(map[model.WorkflowPhase]int)
	s.Flow.PhaseDepartures = make(map[model.WorkflowPhase]int)

	// Aggregate ring buffer
	allDwell := make(map[model.WorkflowPhase][]int)
	queueSum := make(map[model.WorkflowPhase]int)

	for i := 0; i < s.ringLen; i++ {
		td := s.ring[i]
		if td == nil {
			continue
		}
		for phase, depth := range td.PhaseQueue {
			queueSum[phase] += depth
		}
		for phase, arrivals := range td.PhaseArrivals {
			s.Flow.PhaseArrivals[phase] += arrivals
		}
		for phase, departures := range td.PhaseDepartures {
			s.Flow.PhaseDepartures[phase] += departures
		}
		for phase, samples := range td.DwellSamples {
			allDwell[phase] = append(allDwell[phase], samples...)
		}
	}

	// Compute medians and averages
	for phase, samples := range allDwell {
		s.Flow.PhaseDwellMedian[phase] = MedianInt(samples)
	}
	if s.ringLen > 0 {
		for phase, sum := range queueSum {
			s.Flow.PhaseQueueAvg[phase] = float64(sum) / float64(s.ringLen)
		}
	}

	// In-flight ages (from current sim state, not ring)
	for _, t := range sim.ActiveTickets {
		if t.Phase == model.PhaseBacklog || t.Phase == model.PhaseDone {
			continue
		}
		age := sim.CurrentTick - t.PhaseEnteredTick
		if age < 0 {
			age = 0
		}
		if age > s.Flow.PhaseMaxAge[t.Phase] {
			s.Flow.PhaseMaxAge[t.Phase] = age
		}
	}
}

// identifyConstraint picks the phase with highest sustained median dwell time.
func (s *TOCState) identifyConstraint() {
	if s.ringLen < s.WindowSize {
		return // not enough data yet
	}

	// Rank phases by median dwell time (minimum departures required)
	type ranked struct {
		phase model.WorkflowPhase
		dwell float64
	}
	var candidates []ranked
	for phase, dwell := range s.Flow.PhaseDwellMedian {
		departures := s.Flow.PhaseDepartures[phase]
		if departures >= minDepartures {
			candidates = append(candidates, ranked{phase, dwell})
		}
	}

	if len(candidates) == 0 {
		s.candidate = 0
		s.consecutiveN = 0
		s.ConstraintPhase = 0
		s.Confidence = 0
		return
	}

	// Sort descending by dwell time, then by phase value for determinism (map iteration is unordered)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].dwell != candidates[j].dwell {
			return candidates[i].dwell > candidates[j].dwell
		}
		return candidates[i].phase < candidates[j].phase
	})

	top := candidates[0]

	// Check dominance margin
	if len(candidates) > 1 {
		second := candidates[1]
		if top.dwell == 0 || (top.dwell-second.dwell)/top.dwell < dominanceMargin {
			// No clear winner — reset hysteresis
			s.candidate = 0
			s.consecutiveN = 0
			s.ConstraintPhase = 0
			s.Confidence = 0
			return
		}
	}

	// Hysteresis
	if top.phase == s.candidate {
		s.consecutiveN++
	} else {
		s.candidate = top.phase
		s.consecutiveN = 1
		// New candidate emerging — decay confidence of prior confirmed constraint
		if s.ConstraintPhase != 0 {
			s.Confidence *= 0.9
			if s.Confidence < 0.1 {
				s.ConstraintPhase = 0
				s.Confidence = 0
			}
		}
	}

	if s.consecutiveN >= hysteresisCount {
		s.ConstraintPhase = top.phase
		// Confidence: scale from 0.5 (just confirmed) to 1.0 (sustained)
		s.Confidence = 0.5 + 0.5*float64(min(s.consecutiveN, hysteresisCount*2))/float64(hysteresisCount*2)
	}
}

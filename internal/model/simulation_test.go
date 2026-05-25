package model

import (
	"math"
	"testing"
)

// UC38 (#15443): coverage for the new PhaseWIPConfig field, its Clone
// deep-copy semantics, and the PhaseWIPCap accessor's 3-tier precedence.

func TestSimulation_Clone_PhaseWIPConfig_DeepCopy(t *testing.T) {
	t.Run("nil source produces nil clone (no map allocation)", func(t *testing.T) {
		s := NewSimulation("sim-1", PolicyNone, 42)
		if s.PhaseWIPConfig != nil {
			t.Fatalf("NewSimulation should leave PhaseWIPConfig nil; got %v", s.PhaseWIPConfig)
		}
		c := s.Clone()
		if c.PhaseWIPConfig != nil {
			t.Errorf("Clone of nil PhaseWIPConfig should remain nil; got %v", c.PhaseWIPConfig)
		}
	})

	t.Run("non-nil source produces independent clone", func(t *testing.T) {
		s := NewSimulation("sim-2", PolicyNone, 42)
		s.PhaseWIPConfig = map[WorkflowPhase]int{
			PhaseImplement: 4,
			PhaseVerify:    2,
			PhaseCICD:      1,
		}

		c := s.Clone()

		// Snapshot original values for change-detection.
		want := map[WorkflowPhase]int{
			PhaseImplement: 4,
			PhaseVerify:    2,
			PhaseCICD:      1,
		}
		for k, v := range want {
			if c.PhaseWIPConfig[k] != v {
				t.Errorf("clone[%v]: want %d, got %d", k, v, c.PhaseWIPConfig[k])
			}
		}

		// Mutate original; clone must NOT observe the change.
		s.PhaseWIPConfig[PhaseImplement] = 999
		if c.PhaseWIPConfig[PhaseImplement] != 4 {
			t.Errorf("clone observed mutation of source: clone[Implement]=%d (want 4)",
				c.PhaseWIPConfig[PhaseImplement])
		}

		// And vice versa — mutate clone; original must NOT observe.
		c.PhaseWIPConfig[PhaseVerify] = 777
		if s.PhaseWIPConfig[PhaseVerify] != 2 {
			t.Errorf("source observed mutation of clone: source[Verify]=%d (want 2)",
				s.PhaseWIPConfig[PhaseVerify])
		}
	})
}

func TestSimulation_PhaseWIPCap_Precedence(t *testing.T) {
	tests := []struct {
		name   string
		config map[WorkflowPhase]int
		cicd   int
		phase  WorkflowPhase
		want   int
	}{
		{
			name:   "explicit entry wins over CICDSlots for CICD phase",
			config: map[WorkflowPhase]int{PhaseCICD: 5},
			cicd:   2,
			phase:  PhaseCICD,
			want:   5,
		},
		{
			name:   "CICDSlots fallback when no explicit CICD entry",
			config: map[WorkflowPhase]int{PhaseImplement: 4},
			cicd:   2,
			phase:  PhaseCICD,
			want:   2,
		},
		{
			name:   "explicit entry wins for non-CICD phase",
			config: map[WorkflowPhase]int{PhaseImplement: 4},
			cicd:   2,
			phase:  PhaseImplement,
			want:   4,
		},
		{
			name:   "math.MaxInt sentinel for non-CICD phase without entry",
			config: map[WorkflowPhase]int{PhaseImplement: 4},
			cicd:   2,
			phase:  PhaseVerify,
			want:   math.MaxInt,
		},
		{
			name:   "nil config + CICD phase falls back to CICDSlots",
			config: nil,
			cicd:   2,
			phase:  PhaseCICD,
			want:   2,
		},
		{
			name:   "nil config + non-CICD phase returns math.MaxInt",
			config: nil,
			cicd:   2,
			phase:  PhaseImplement,
			want:   math.MaxInt,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSimulation("sim", PolicyNone, 0)
			s.PhaseWIPConfig = tc.config
			s.CICDSlots = tc.cicd
			if got := s.PhaseWIPCap(tc.phase); got != tc.want {
				t.Errorf("PhaseWIPCap(%v) = %d; want %d", tc.phase, got, tc.want)
			}
		})
	}
}

func TestSimulation_PhaseWIPCount(t *testing.T) {
	s := NewSimulation("sim", PolicyNone, 0)
	// Two assigned + one queued (AssignedTo "") in Implement; one assigned
	// in Verify. Counter must return assigned-only — queued ticket T4
	// MUST NOT count (it would deadlock the cap gate it's waiting on).
	s.ActiveTickets = []Ticket{
		{ID: "T1", Phase: PhaseImplement, AssignedTo: "dev-a"},
		{ID: "T2", Phase: PhaseImplement, AssignedTo: "dev-b"},
		{ID: "T3", Phase: PhaseVerify, AssignedTo: "dev-c"},
		{ID: "T4", Phase: PhaseImplement, AssignedTo: ""}, // queued
	}
	s.PhaseQueues = map[WorkflowPhase][]string{
		PhaseImplement: {"T4"},
	}

	tests := []struct {
		phase WorkflowPhase
		want  int
	}{
		{PhaseImplement, 2}, // T1, T2 assigned; T4 queued (not counted)
		{PhaseVerify, 1},    // T3 assigned
		{PhaseCICD, 0},
		{PhaseReview, 0},
	}
	for _, tc := range tests {
		if got := s.PhaseWIPCount(tc.phase); got != tc.want {
			t.Errorf("PhaseWIPCount(%v) = %d; want %d", tc.phase, got, tc.want)
		}
	}
}

// UC39 (#15445): scalar/enum fields (ReleaseMode, WarmupActive,
// WarmupFailed, MaxBacklogDrip) are trivially preserved by Clone's
// value-copy semantics. Test documents the contract explicitly so a
// future refactor that switches Clone to selective field-copying can't
// silently drop them.
func TestSimulation_Clone_ReleaseModeFields_Preserve(t *testing.T) {
	s := NewSimulation("sim-uc39", PolicyNone, 99)
	s.ReleaseMode = ReleaseModeDemand
	s.WarmupActive = true
	s.WarmupFailed = false
	s.MaxBacklogDrip = 3 // non-default to detect zero-value short-circuits

	c := s.Clone()

	if c.ReleaseMode != ReleaseModeDemand {
		t.Errorf("clone.ReleaseMode = %v; want ReleaseModeDemand", c.ReleaseMode)
	}
	if !c.WarmupActive {
		t.Errorf("clone.WarmupActive = false; want true")
	}
	if c.WarmupFailed {
		t.Errorf("clone.WarmupFailed = true; want false")
	}
	if c.MaxBacklogDrip != 3 {
		t.Errorf("clone.MaxBacklogDrip = %d; want 3", c.MaxBacklogDrip)
	}

	// Mutate clone; source must not observe (value-copy invariant).
	c.WarmupActive = false
	c.ReleaseMode = ReleaseModePush
	if !s.WarmupActive || s.ReleaseMode != ReleaseModeDemand {
		t.Errorf("source observed mutation of clone: WarmupActive=%v ReleaseMode=%v",
			s.WarmupActive, s.ReleaseMode)
	}
}

func TestNewSimulation_MaxBacklogDripDefault(t *testing.T) {
	s := NewSimulation("sim", PolicyNone, 0)
	if s.MaxBacklogDrip != 1 {
		t.Errorf("NewSimulation MaxBacklogDrip = %d; want 1 (UC38 CICDSlots-default precedent)",
			s.MaxBacklogDrip)
	}
	if s.ReleaseMode != ReleaseModePush {
		t.Errorf("NewSimulation ReleaseMode = %v; want ReleaseModePush (zero-value)", s.ReleaseMode)
	}
	if s.WarmupActive {
		t.Error("NewSimulation WarmupActive = true; want false (only true when projection sees ReleaseMode==Demand)")
	}
}

package model

import (
	"strings"
	"testing"
)

// UC39 (#15445): table-driven coverage of the demand-only ShouldAdmit
// and WarmupExit pure calculations per Decision A + B (Khorikov Domain
// quadrant: heavy unit-test with table-driven cases over input space).

func TestShouldAdmit(t *testing.T) {
	tests := []struct {
		name        string
		penetration float64
		drip        int
		wantAdmit   int
		wantReason  string // substring assertion
	}{
		// Penetration=0 (fully green) — admits full drip
		{"green Penetration=0 drip=1 → admit 1", 0.0, 1, 1, "admit 1"},
		{"green Penetration=0 drip=3 → admit 3", 0.0, 3, 3, "admit 3"},
		// Mid penetration — floor cuts in
		{"mid Penetration=0.5 drip=1 → admit 0 (floor(0.5)=0; red-zone)", 0.5, 1, 0, "red-zone throttle"},
		{"mid Penetration=0.5 drip=3 → admit 1 (floor(1.5)=1)", 0.5, 3, 1, "admit 1"},
		{"mid Penetration=0.7 drip=1 → admit 0", 0.7, 1, 0, "red-zone throttle"},
		{"mid Penetration=0.7 drip=3 → admit 0 (floor(0.9)=0)", 0.7, 3, 0, "red-zone throttle"},
		// Penetration=1 (fully red) — always throttles to 0
		{"red Penetration=1.0 drip=1 → admit 0", 1.0, 1, 0, "red-zone throttle"},
		{"red Penetration=1.0 drip=3 → admit 0", 1.0, 3, 0, "red-zone throttle"},
		// MaxBacklogDrip=0 edge (should never happen post-projection-default fix,
		// but the calc itself must handle it sanely — red-zone reason)
		{"drip=0 edge → admit 0", 0.0, 0, 0, "red-zone throttle"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			signal := AnalyzerSignal{Penetration: tc.penetration}
			admit, reason := ShouldAdmit(signal, tc.drip)
			if admit != tc.wantAdmit {
				t.Errorf("admit = %d; want %d", admit, tc.wantAdmit)
			}
			if !strings.Contains(reason, tc.wantReason) {
				t.Errorf("reason = %q; want substring %q", reason, tc.wantReason)
			}
		})
	}
}

func TestWarmupExit(t *testing.T) {
	tests := []struct {
		name       string
		constraint WorkflowPhase
		confidence float64
		threshold  float64
		want       bool
	}{
		// No constraint locked — always false regardless of confidence
		{"no constraint + high confidence → false", PhaseBacklog, 1.0, 0.5, false},
		{"no constraint + 0 confidence → false", PhaseBacklog, 0.0, 0.5, false},
		// Constraint locked but confidence too low
		{"constraint + 0.0 confidence < 0.5 → false", PhaseImplement, 0.0, 0.5, false},
		{"constraint + 0.49 confidence < 0.5 → false", PhaseImplement, 0.49, 0.5, false},
		// Exact boundary (Confidence == threshold) must return true (documented contract)
		{"constraint + 0.5 confidence == 0.5 → true (boundary)", PhaseImplement, 0.5, 0.5, true},
		// Above threshold
		{"constraint + 0.7 confidence > 0.5 → true", PhaseImplement, 0.7, 0.5, true},
		{"constraint + 1.0 confidence > 0.5 → true", PhaseImplement, 1.0, 0.5, true},
		// Different constraint phases — all valid as long as != PhaseBacklog
		{"constraint=Verify + 0.5 → true", PhaseVerify, 0.5, 0.5, true},
		{"constraint=Review + 0.5 → true", PhaseReview, 0.5, 0.5, true},
		// Tuned threshold (UC40 may make this configurable)
		{"high threshold 0.9 not met by 0.7 → false", PhaseImplement, 0.7, 0.9, false},
		{"high threshold 0.9 met by 0.9 → true", PhaseImplement, 0.9, 0.9, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			signal := AnalyzerSignal{
				Constraint: tc.constraint,
				Confidence: tc.confidence,
			}
			if got := WarmupExit(signal, tc.threshold); got != tc.want {
				t.Errorf("WarmupExit = %v; want %v", got, tc.want)
			}
		})
	}
}

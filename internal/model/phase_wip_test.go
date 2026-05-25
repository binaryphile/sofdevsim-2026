package model

import (
	"errors"
	"testing"
)

// UC38 (#15443): Khorikov Domain quadrant; heavy unit coverage of the
// pure validator over all 4 sentinel cases + happy paths.
// errors.Is per sentinel (Decision B Go dev §8).

func TestValidatePhaseWIPConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[WorkflowPhase]int
		rope    RopeConfig
		wantErr error // sentinel; nil for happy
	}{
		// Happy paths
		{
			name:    "nil config is unlimited everywhere; no violation",
			cfg:     nil,
			wantErr: nil,
		},
		{
			name:    "empty config is unlimited everywhere; no violation",
			cfg:     map[WorkflowPhase]int{},
			wantErr: nil,
		},
		{
			name: "Implement cap = 2 (mentor-pair min) is allowed",
			cfg: map[WorkflowPhase]int{
				PhaseImplement: 2,
				PhaseVerify:    1,
			},
			wantErr: nil,
		},
		{
			name: "non-rope-controlled phase exceeding rope MaxWIP is allowed",
			cfg: map[WorkflowPhase]int{
				PhaseResearch: 10,
				PhaseSizing:   10,
			},
			rope:    RopeConfig{Enabled: true, MaxWIP: 5},
			wantErr: nil,
		},
		{
			name: "rope-controlled cap equal to rope.MaxWIP is allowed (only > is conflict)",
			cfg: map[WorkflowPhase]int{
				PhaseImplement: 5,
			},
			rope:    RopeConfig{Enabled: true, MaxWIP: 5},
			wantErr: nil,
		},
		{
			name: "rope disabled — cap > would-be MaxWIP is allowed",
			cfg: map[WorkflowPhase]int{
				PhaseImplement: 100,
			},
			rope:    RopeConfig{Enabled: false, MaxWIP: 1},
			wantErr: nil,
		},

		// ErrCapZero
		{
			name: "Verify cap = 0 returns ErrCapZero",
			cfg: map[WorkflowPhase]int{
				PhaseVerify: 0,
			},
			wantErr: ErrCapZero,
		},
		{
			name: "CICD cap = 0 returns ErrCapZero (matches UC38 §Extensions §4a)",
			cfg: map[WorkflowPhase]int{
				PhaseCICD: 0,
			},
			wantErr: ErrCapZero,
		},

		// ErrCapNegative
		{
			name: "Review cap = -1 returns ErrCapNegative",
			cfg: map[WorkflowPhase]int{
				PhaseReview: -1,
			},
			wantErr: ErrCapNegative,
		},

		// ErrCapBelowMentorMin (Implement only)
		{
			name: "Implement cap = 1 returns ErrCapBelowMentorMin",
			cfg: map[WorkflowPhase]int{
				PhaseImplement: 1,
			},
			wantErr: ErrCapBelowMentorMin,
		},

		// ErrCapConflict (rope-controlled cap > rope.MaxWIP when rope.Enabled)
		{
			name: "Implement cap > rope.MaxWIP returns ErrCapConflict when rope enabled",
			cfg: map[WorkflowPhase]int{
				PhaseImplement: 10,
			},
			rope:    RopeConfig{Enabled: true, MaxWIP: 5},
			wantErr: ErrCapConflict,
		},
		{
			name: "Verify cap > rope.MaxWIP returns ErrCapConflict when rope enabled",
			cfg: map[WorkflowPhase]int{
				PhaseVerify: 7,
			},
			rope:    RopeConfig{Enabled: true, MaxWIP: 5},
			wantErr: ErrCapConflict,
		},
		{
			name: "Review cap > rope.MaxWIP returns ErrCapConflict when rope enabled",
			cfg: map[WorkflowPhase]int{
				PhaseReview: 8,
			},
			rope:    RopeConfig{Enabled: true, MaxWIP: 5},
			wantErr: ErrCapConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePhaseWIPConfig(tc.cfg, tc.rope)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("got err %v; want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got err %v; want errors.Is(err, %v)", err, tc.wantErr)
			}
		})
	}
}

// errors.Is across the unwrap chain — guards against future refactors
// that might forget to wrap with %w.
func TestValidatePhaseWIPConfig_ErrorsIsChain(t *testing.T) {
	err := ValidatePhaseWIPConfig(
		map[WorkflowPhase]int{PhaseImplement: 0},
		RopeConfig{},
	)
	if !errors.Is(err, ErrCapZero) {
		t.Fatalf("ValidatePhaseWIPConfig returned unwrappable err: %v", err)
	}
	// Must NOT erroneously match a different sentinel.
	if errors.Is(err, ErrCapNegative) {
		t.Errorf("err matched ErrCapNegative; should only match ErrCapZero: %v", err)
	}
}

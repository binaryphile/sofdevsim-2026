package lessons

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestHasRedBufferWithLowTicket(t *testing.T) {
	tests := []struct {
		name        string
		feverStatus model.FeverStatus
		tickets     []model.Ticket
		want        bool
	}{
		{"red + low = true", model.FeverRed, []model.Ticket{{UnderstandingLevel: model.LowUnderstanding}}, true},
		{"red + high = false", model.FeverRed, []model.Ticket{{UnderstandingLevel: model.HighUnderstanding}}, false},
		{"red + mixed = true", model.FeverRed, []model.Ticket{{UnderstandingLevel: model.HighUnderstanding}, {UnderstandingLevel: model.LowUnderstanding}}, true},
		{"yellow + low = false", model.FeverYellow, []model.Ticket{{UnderstandingLevel: model.LowUnderstanding}}, false},
		{"green + low = false", model.FeverGreen, []model.Ticket{{UnderstandingLevel: model.LowUnderstanding}}, false},
		{"red + empty = false", model.FeverRed, []model.Ticket{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasRedBufferWithLowTicket(tt.feverStatus, tt.tickets); got != tt.want {
				t.Errorf("HasRedBufferWithLowTicket() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasRedBufferWithLowTicketFromStrings(t *testing.T) {
	tests := []struct {
		name           string
		isRed          bool
		understandings []string
		want           bool
	}{
		{"red + LOW = true", true, []string{"LOW"}, true},
		{"red + HIGH = false", true, []string{"HIGH"}, false},
		{"red + mixed = true", true, []string{"HIGH", "LOW"}, true},
		{"not red + LOW = false", false, []string{"LOW"}, false},
		{"red + empty = false", true, []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasRedBufferWithLowTicketFromStrings(tt.isRed, tt.understandings); got != tt.want {
				t.Errorf("HasRedBufferWithLowTicketFromStrings() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHasElevationWithoutExploitation — UC40 ext §3a predicate (#18517).
// Table-driven over the (flag × current-vs-snapshot) state space including
// the boundary cases (nil toc, ConstraintPhase=0, LastSprintConstraintPhase=0).
func TestHasElevationWithoutExploitation(t *testing.T) {
	tests := []struct {
		name string
		toc  *metrics.TOCState
		want bool
	}{
		{"nil toc returns false", nil, false},
		{"flag false returns false",
			&metrics.TOCState{InvestmentOccurredThisCycle: false, ConstraintPhase: model.PhaseImplement, LastSprintConstraintPhase: model.PhaseImplement},
			false},
		{"flag true + constraint unchanged returns TRUE",
			&metrics.TOCState{InvestmentOccurredThisCycle: true, ConstraintPhase: model.PhaseImplement, LastSprintConstraintPhase: model.PhaseImplement},
			true},
		{"flag true + constraint moved returns false",
			&metrics.TOCState{InvestmentOccurredThisCycle: true, ConstraintPhase: model.PhaseReview, LastSprintConstraintPhase: model.PhaseImplement},
			false},
		{"flag true + ConstraintPhase=0 (no constraint identified) returns false",
			&metrics.TOCState{InvestmentOccurredThisCycle: true, ConstraintPhase: 0, LastSprintConstraintPhase: model.PhaseImplement},
			false},
		{"flag true + LastSprintConstraintPhase=0 (no prior constraint) returns false",
			&metrics.TOCState{InvestmentOccurredThisCycle: true, ConstraintPhase: model.PhaseImplement, LastSprintConstraintPhase: 0},
			false},
		{"flag true + both 0 returns false",
			&metrics.TOCState{InvestmentOccurredThisCycle: true, ConstraintPhase: 0, LastSprintConstraintPhase: 0},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasElevationWithoutExploitation(tt.toc); got != tt.want {
				t.Errorf("HasElevationWithoutExploitation() = %v, want %v", got, tt.want)
			}
		})
	}
}

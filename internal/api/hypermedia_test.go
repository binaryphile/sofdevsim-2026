package api

import (
	"testing"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestLinksFor(t *testing.T) {
	tests := []struct {
		name            string
		sprintOption    option.Option[model.Sprint]
		backlogCount    int
		wantTick        bool
		wantStartSprint bool
		wantAssign      bool
	}{
		{
			name:            "sprint active with backlog has tick and assign links",
			sprintOption:    option.Of(model.Sprint{EndDay: 10}),
			backlogCount:    5,
			wantTick:        true,
			wantStartSprint: false,
			wantAssign:      true,
		},
		{
			name:            "sprint active with empty backlog has tick but no assign",
			sprintOption:    option.Of(model.Sprint{EndDay: 10}),
			backlogCount:    0,
			wantTick:        true,
			wantStartSprint: false,
			wantAssign:      false,
		},
		{
			name:            "no sprint with backlog has start-sprint and assign links",
			sprintOption:    option.Option[model.Sprint]{},
			backlogCount:    5,
			wantTick:        false,
			wantStartSprint: true,
			wantAssign:      true, // UC11: assign available for sprint planning
		},
		{
			name:            "no sprint with empty backlog has start-sprint only",
			sprintOption:    option.Option[model.Sprint]{},
			backlogCount:    0,
			wantTick:        false,
			wantStartSprint: true,
			wantAssign:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := SimulationState{
				ID:                  "sim-42",
				CurrentSprintOption: tt.sprintOption,
				BacklogCount:        tt.backlogCount,
			}

			links := LinksFor(state)

			// Self link always present
			if links["self"] == "" {
				t.Error("self link should always be present")
			}
			if links["self"] != "/simulations/sim-42" {
				t.Errorf("self link: got %q, want %q", links["self"], "/simulations/sim-42")
			}

			if gotTick := links["tick"] != ""; gotTick != tt.wantTick {
				t.Errorf("tick link: got %v, want %v", gotTick, tt.wantTick)
			}
			if gotStartSprint := links["start-sprint"] != ""; gotStartSprint != tt.wantStartSprint {
				t.Errorf("start-sprint link: got %v, want %v", gotStartSprint, tt.wantStartSprint)
			}
			if gotAssign := links["assign"] != ""; gotAssign != tt.wantAssign {
				t.Errorf("assign link: got %v, want %v", gotAssign, tt.wantAssign)
			}
		})
	}
}

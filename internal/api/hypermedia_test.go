package api

import (
	"testing"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestLinksFor(t *testing.T) {
	tests := []struct {
		name            string
		sprintOption    option.Basic[model.Sprint]
		wantTick        bool
		wantStartSprint bool
	}{
		{
			name:            "sprint active has tick link",
			sprintOption:    option.Of(model.Sprint{EndDay: 10}),
			wantTick:        true,
			wantStartSprint: false,
		},
		{
			name:            "no sprint has start-sprint link",
			sprintOption:    option.Basic[model.Sprint]{},
			wantTick:        false,
			wantStartSprint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := SimulationState{
				ID:                  "sim-42",
				CurrentSprintOption: tt.sprintOption,
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
		})
	}
}

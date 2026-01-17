package api

import (
	"encoding/json"
	"fmt"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// SimulationState is the JSON-friendly representation of a simulation.
// Uses option.Basic for optional Sprint (value semantics).
type SimulationState struct {
	ID                   string                     `json:"id"`
	CurrentTick          int                        `json:"currentTick"`
	CurrentSprintOption  option.Basic[model.Sprint] `json:"-"` // Custom marshaling
	SprintNumber         int                        `json:"sprintNumber"`
	BacklogCount         int                        `json:"backlogCount"`
	ActiveTicketCount    int                        `json:"activeTicketCount"`
	CompletedTicketCount int                        `json:"completedTicketCount"`
}

// MarshalJSON implements custom JSON marshaling for SimulationState.
// Converts option.Basic[Sprint] to sprintActive bool + sprint object.
func (s SimulationState) MarshalJSON() ([]byte, error) {
	type jsonState struct {
		ID                   string        `json:"id"`
		CurrentTick          int           `json:"currentTick"`
		SprintActive         bool          `json:"sprintActive"`
		Sprint               *model.Sprint `json:"sprint,omitempty"`
		SprintNumber         int           `json:"sprintNumber"`
		BacklogCount         int           `json:"backlogCount"`
		ActiveTicketCount    int           `json:"activeTicketCount"`
		CompletedTicketCount int           `json:"completedTicketCount"`
	}

	out := jsonState{
		ID:                   s.ID,
		CurrentTick:          s.CurrentTick,
		SprintNumber:         s.SprintNumber,
		BacklogCount:         s.BacklogCount,
		ActiveTicketCount:    s.ActiveTicketCount,
		CompletedTicketCount: s.CompletedTicketCount,
	}

	if sprint, ok := s.CurrentSprintOption.Get(); ok {
		out.SprintActive = true
		out.Sprint = &sprint
	}

	return json.Marshal(out)
}

// HALResponse wraps SimulationState with HAL-style links.
type HALResponse struct {
	State SimulationState   `json:"simulation"`
	Links map[string]string `json:"_links"`
}

// ToState converts model.Simulation to SimulationState.
func ToState(sim model.Simulation) SimulationState {
	return SimulationState{
		ID:                   fmt.Sprintf("sim-%d", sim.Seed),
		CurrentTick:          sim.CurrentTick,
		CurrentSprintOption:  sim.CurrentSprintOption,
		SprintNumber:         sim.SprintNumber,
		BacklogCount:         len(sim.Backlog),
		ActiveTicketCount:    len(sim.ActiveTickets),
		CompletedTicketCount: len(sim.CompletedTickets),
	}
}

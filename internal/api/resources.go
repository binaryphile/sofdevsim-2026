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

// CompareRequest is the input for POST /comparisons.
type CompareRequest struct {
	Seed    int64 `json:"seed"`
	Sprints int   `json:"sprints"`
}

// CompareResponse is the HAL+JSON output for POST /comparisons.
type CompareResponse struct {
	Seed    int64             `json:"seed"`
	Sprints int               `json:"sprints"`
	PolicyA PolicyResult      `json:"policyA"`
	PolicyB PolicyResult      `json:"policyB"`
	Winners MetricWinners     `json:"winners"`
	WinsA   int               `json:"winsA"`
	WinsB   int               `json:"winsB"`
	Links   map[string]string `json:"_links"`
}

// PolicyResult represents one policy's simulation results.
type PolicyResult struct {
	Name            string       `json:"name"`
	TicketsComplete int          `json:"ticketsComplete"`
	IncidentCount   int          `json:"incidentCount"`
	Metrics         DORAResponse `json:"metrics"`
}

// DORAResponse is the JSON-friendly DORA metrics.
type DORAResponse struct {
	LeadTimeAvgDays   float64 `json:"leadTimeAvgDays"`
	DeployFrequency   float64 `json:"deployFrequency"`
	MTTRAvgDays       float64 `json:"mttrAvgDays"`
	ChangeFailRatePct float64 `json:"changeFailRatePct"`
}

// MetricWinners shows which policy won each metric.
type MetricWinners struct {
	LeadTime        string `json:"leadTime"`
	DeployFrequency string `json:"deployFrequency"`
	MTTR            string `json:"mttr"`
	ChangeFailRate  string `json:"changeFailRate"`
	Overall         string `json:"overall"`
}

// LessonResponse is the JSON representation of a lesson.
type LessonResponse struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tips    []string `json:"tips,omitempty"`
}

// LessonsResponse is returned by GET /simulations/{id}/lessons.
type LessonsResponse struct {
	CurrentLesson LessonResponse    `json:"currentLesson"`
	Progress      string            `json:"progress"`
	Links         map[string]string `json:"_links"`
}

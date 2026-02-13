package api

import (
	"encoding/json"
	"fmt"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// TicketState is the JSON-friendly representation of a ticket.
type TicketState struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Size          float64  `json:"size"`
	Understanding string   `json:"understanding"`
	Progress      float64  `json:"progress,omitempty"`
	AssignedTo    string   `json:"assignedTo,omitempty"`
	Phase         string   `json:"phase,omitempty"`
	ActualDays    float64  `json:"actualDays,omitempty"`
	ParentID      string   `json:"parentId,omitempty"`
	ChildIDs      []string `json:"childIds,omitempty"`
	EstimatedDays float64  `json:"estimatedDays,omitempty"`
}

// ToTicketState converts model.Ticket to TicketState.
func ToTicketState(t model.Ticket) TicketState {
	progress := 0.0
	if t.ActualDays > 0 {
		progress = (t.ActualDays - t.RemainingEffort) / t.ActualDays * 100
	}
	return TicketState{
		ID:            t.ID,
		Title:         t.Title,
		Size:          t.EstimatedDays,
		Understanding: t.UnderstandingLevel.String(),
		Progress:      progress,
		AssignedTo:    "", // Set by caller for active tickets
		Phase:         t.Phase.String(),
		ActualDays:    t.ActualDays,
		ParentID:      t.ParentID,
		ChildIDs:      t.ChildIDs,
		EstimatedDays: t.EstimatedDays,
	}
}

// DeveloperState is the JSON-friendly representation of a developer.
type DeveloperState struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Velocity      float64 `json:"velocity"`
	CurrentTicket string  `json:"currentTicket,omitempty"`
	IsIdle        bool    `json:"isIdle"`
}

// ToDeveloperState converts model.Developer to DeveloperState.
func ToDeveloperState(d model.Developer) DeveloperState {
	return DeveloperState{
		ID:            d.ID,
		Name:          d.Name,
		Velocity:      d.Velocity,
		CurrentTicket: d.CurrentTicket,
		IsIdle:        d.IsIdle(),
	}
}

// SimulationState is the JSON-friendly representation of a simulation.
// Uses option.Basic for optional Sprint (value semantics).
type SimulationState struct {
	ID                   string                     `json:"id"`
	Seed                 int64                      `json:"seed"`
	CurrentTick          int                        `json:"currentTick"`
	CurrentSprintOption  option.Basic[model.Sprint] `json:"-"` // Custom marshaling
	SprintNumber         int                        `json:"sprintNumber"`
	SizingPolicy         string                     `json:"sizingPolicy"`
	BacklogCount         int                        `json:"backlogCount"`
	ActiveTicketCount    int                        `json:"activeTicketCount"`
	CompletedTicketCount int                        `json:"completedTicketCount"`
	TotalIncidents       int                        `json:"totalIncidents"`
	Backlog              []TicketState              `json:"backlog"`
	Developers           []DeveloperState           `json:"developers"`
	ActiveTickets        []TicketState              `json:"activeTickets"`
	CompletedTickets     []TicketState              `json:"completedTickets"`
	Metrics              DORAResponse               `json:"metrics"`
}

// MarshalJSON implements custom JSON marshaling for SimulationState.
// Converts option.Basic[Sprint] to sprintActive bool + sprint object.
func (s SimulationState) MarshalJSON() ([]byte, error) {
	type jsonState struct {
		ID                   string           `json:"id"`
		Seed                 int64            `json:"seed"`
		CurrentTick          int              `json:"currentTick"`
		SprintActive         bool             `json:"sprintActive"`
		Sprint               *model.Sprint    `json:"sprint,omitempty"`
		SprintNumber         int              `json:"sprintNumber"`
		SizingPolicy         string           `json:"sizingPolicy"`
		BacklogCount         int              `json:"backlogCount"`
		ActiveTicketCount    int              `json:"activeTicketCount"`
		CompletedTicketCount int              `json:"completedTicketCount"`
		TotalIncidents       int              `json:"totalIncidents"`
		Backlog              []TicketState    `json:"backlog"`
		Developers           []DeveloperState `json:"developers"`
		ActiveTickets        []TicketState    `json:"activeTickets"`
		CompletedTickets     []TicketState    `json:"completedTickets"`
		Metrics              DORAResponse     `json:"metrics"`
	}

	out := jsonState{
		ID:                   s.ID,
		Seed:                 s.Seed,
		CurrentTick:          s.CurrentTick,
		SprintNumber:         s.SprintNumber,
		SizingPolicy:         s.SizingPolicy,
		BacklogCount:         s.BacklogCount,
		ActiveTicketCount:    s.ActiveTicketCount,
		CompletedTicketCount: s.CompletedTicketCount,
		TotalIncidents:       s.TotalIncidents,
		Backlog:              s.Backlog,
		Developers:           s.Developers,
		ActiveTickets:        s.ActiveTickets,
		CompletedTickets:     s.CompletedTickets,
		Metrics:              s.Metrics,
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

// ToState converts model.Simulation and tracker to SimulationState.
// Tracker may be nil for simulations without metrics tracking.
func ToState(sim model.Simulation, tracker metrics.Tracker) SimulationState {
	// Convert backlog tickets
	backlog := slice.MapTo[TicketState](sim.Backlog).Map(ToTicketState)

	// Convert developers
	developers := slice.MapTo[DeveloperState](sim.Developers).Map(ToDeveloperState)

	// Convert active tickets with assigned developer info
	activeTickets := make([]TicketState, len(sim.ActiveTickets))
	for i, t := range sim.ActiveTickets {
		ts := ToTicketState(t)
		// Find assigned developer
		for _, d := range sim.Developers {
			if d.CurrentTicket == t.ID {
				ts.AssignedTo = d.ID
				break
			}
		}
		activeTickets[i] = ts
	}

	// Convert completed tickets
	completedTickets := slice.MapTo[TicketState](sim.CompletedTickets).Map(ToTicketState)

	// Compute metrics from tracker (safe even for zero tracker - returns zero metrics)
	result := tracker.GetResult(sim.SizingPolicy, sim)

	// Convert DORA history for sparklines
	history := slice.MapTo[DORAHistoryPoint](tracker.DORA.History).Map(toDORAHistoryPoint)

	metricsState := DORAResponse{
		LeadTimeAvgDays:   result.FinalMetrics.LeadTimeAvgDays(),
		DeployFrequency:   result.FinalMetrics.DeployFrequency,
		MTTRAvgDays:       result.FinalMetrics.MTTRAvgDays(),
		ChangeFailRatePct: result.FinalMetrics.ChangeFailRatePct(),
		History:           history,
	}

	return SimulationState{
		ID:                   fmt.Sprintf("sim-%d", sim.Seed),
		Seed:                 sim.Seed,
		CurrentTick:          sim.CurrentTick,
		CurrentSprintOption:  sim.CurrentSprintOption,
		SprintNumber:         sim.SprintNumber,
		SizingPolicy:         sim.SizingPolicy.String(),
		BacklogCount:         len(sim.Backlog),
		ActiveTicketCount:    len(sim.ActiveTickets),
		CompletedTicketCount: len(sim.CompletedTickets),
		TotalIncidents:       sim.TotalIncidents(),
		Backlog:              backlog,
		Developers:           developers,
		ActiveTickets:        activeTickets,
		CompletedTickets:     completedTickets,
		Metrics:              metricsState,
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
	LeadTimeAvgDays   float64           `json:"leadTimeAvgDays"`
	DeployFrequency   float64           `json:"deployFrequency"`
	MTTRAvgDays       float64           `json:"mttrAvgDays"`
	ChangeFailRatePct float64           `json:"changeFailRatePct"`
	History           []DORAHistoryPoint `json:"history,omitempty"`
}

// DORAHistoryPoint is a single point in DORA metrics history.
type DORAHistoryPoint struct {
	LeadTimeAvg    float64 `json:"leadTimeAvg"`
	DeployFrequency float64 `json:"deployFrequency"`
	MTTR           float64 `json:"mttr"`
	ChangeFailRate float64 `json:"changeFailRate"`
}

// toDORAHistoryPoint converts metrics.DORASnapshot to DORAHistoryPoint.
func toDORAHistoryPoint(s metrics.DORASnapshot) DORAHistoryPoint {
	return DORAHistoryPoint{
		LeadTimeAvg:    s.LeadTimeAvg,
		DeployFrequency: s.DeployFrequency,
		MTTR:           s.MTTR,
		ChangeFailRate: s.ChangeFailRate,
	}
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

// DeveloperAnimationState is the JSON representation of a developer's animation state.
type DeveloperAnimationState struct {
	DevID     string `json:"devId"`
	DevName   string `json:"devName"`
	State     string `json:"state"`
	ColorName string `json:"colorName"`
	TicketID  string `json:"ticketId,omitempty"`
}

// StateTransitionResponse is the JSON representation of a state transition.
type StateTransitionResponse struct {
	DevID     string `json:"devId"`
	FromState string `json:"fromState"`
	ToState   string `json:"toState"`
	Tick      int    `json:"tick"`
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason,omitempty"`
}

// OfficeResponse is returned by GET /simulations/{id}/office.
// Provides both rendered output and structured data for Claude vision.
type OfficeResponse struct {
	// Rendered output (what TUI displays)
	RenderedOutput string `json:"renderedOutput"`
	RenderedPlain  string `json:"renderedPlain"` // ANSI stripped

	// Structured data for semantic understanding
	Developers  []DeveloperAnimationState `json:"developers"`
	Transitions []StateTransitionResponse `json:"recentTransitions"`

	// Layout info
	Width  int `json:"width"`
	Height int `json:"height"`

	// Current state
	CurrentTick int `json:"currentTick"`

	Links map[string]string `json:"_links"`
}

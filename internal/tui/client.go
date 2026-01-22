package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Client provides HTTP communication with the simulation API server.
// Uses 5s timeout and X-Request-ID header for request deduplication.
// Value type: contains *http.Client reference but doesn't mutate own fields.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new HTTP client for the given base URL.
// Returns value type - the *http.Client inside is shared across copies.
func NewClient(baseURL string) Client {
	return Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CreateSimulationRequest is the request body for creating a simulation.
type CreateSimulationRequest struct {
	Seed   int64  `json:"seed"`
	Policy string `json:"policy,omitempty"`
}

// TicketState mirrors api.TicketState for client-side decoding.
type TicketState struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Size          float64 `json:"size"`
	Understanding string  `json:"understanding"`
	Progress      float64 `json:"progress,omitempty"`
	AssignedTo    string  `json:"assignedTo,omitempty"`
	Phase         string  `json:"phase,omitempty"`
	ActualDays    float64 `json:"actualDays,omitempty"`
}

// DeveloperState mirrors api.DeveloperState for client-side decoding.
type DeveloperState struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Velocity      float64 `json:"velocity"`
	CurrentTicket string  `json:"currentTicket,omitempty"`
	IsIdle        bool    `json:"isIdle"`
}

// MetricsState mirrors api.DORAResponse for client-side decoding.
type MetricsState struct {
	LeadTimeAvgDays   float64             `json:"leadTimeAvgDays"`
	DeployFrequency   float64             `json:"deployFrequency"`
	MTTRAvgDays       float64             `json:"mttrAvgDays"`
	ChangeFailRatePct float64             `json:"changeFailRatePct"`
	History           []DORAHistoryPoint  `json:"history,omitempty"`
}

// DORAHistoryPoint mirrors api.DORAHistoryPoint for sparklines.
type DORAHistoryPoint struct {
	LeadTimeAvg    float64 `json:"leadTimeAvg"`
	DeployFrequency float64 `json:"deployFrequency"`
	MTTR           float64 `json:"mttr"`
	ChangeFailRate float64 `json:"changeFailRate"`
}

// SprintState mirrors model.Sprint for client-side decoding.
type SprintState struct {
	Number         int     `json:"number"`
	StartDay       int     `json:"startDay"`
	EndDay         int     `json:"endDay"`
	DurationDays   int     `json:"durationDays"`
	BufferDays     float64 `json:"bufferDays"`
	BufferConsumed float64 `json:"bufferConsumed"`
}

// SimulationState mirrors api.SimulationState for client-side decoding.
type SimulationState struct {
	ID                   string           `json:"id"`
	Seed                 int64            `json:"seed"`
	CurrentTick          int              `json:"currentTick"`
	SprintActive         bool             `json:"sprintActive"`
	Sprint               *SprintState     `json:"sprint,omitempty"`
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
	Metrics              MetricsState     `json:"metrics"`
}

// HALResponse mirrors api.HALResponse for client-side decoding.
type HALResponse struct {
	Simulation SimulationState   `json:"simulation"`
	Links      map[string]string `json:"_links"`
}

// CreateSimulationResponse is the response from creating a simulation.
type CreateSimulationResponse struct {
	Simulation SimulationState   `json:"simulation"`
	Links      map[string]string `json:"_links"`
}

// CreateSimulation creates a new simulation via HTTP API.
func (c Client) CreateSimulation(seed int64, policy string) (*CreateSimulationResponse, error) {
	body := CreateSimulationRequest{Seed: seed, Policy: policy}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/simulations", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result CreateSimulationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// TickResponse is the response from advancing a simulation tick.
type TickResponse struct {
	Simulation SimulationState   `json:"simulation"`
	Links      map[string]string `json:"_links"`
}

// Tick advances the simulation by one tick.
func (c Client) Tick(simID string) (*TickResponse, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/simulations/"+simID+"/tick", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result TickResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// AssignRequest is the request body for ticket assignment.
type AssignRequest struct {
	TicketID    string `json:"ticketId"`
	DeveloperID string `json:"developerId,omitempty"`
}

// Assign assigns a ticket to a developer.
func (c Client) Assign(simID, ticketID, devID string) error {
	body := AssignRequest{TicketID: ticketID, DeveloperID: devID}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/simulations/"+simID+"/assignments", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// StartSprint starts a new sprint for the simulation.
func (c Client) StartSprint(simID string) (*HALResponse, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/simulations/"+simID+"/sprints", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result HALResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// GetSimulation fetches the current state of a simulation.
func (c Client) GetSimulation(simID string) (*HALResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/simulations/"+simID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result HALResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// UpdateSimulationRequest is the request body for updating simulation settings.
type UpdateSimulationRequest struct {
	Policy string `json:"policy,omitempty"`
}

// SetPolicy updates the sizing policy for a simulation.
func (c Client) SetPolicy(simID string, policy string) (*HALResponse, error) {
	body := UpdateSimulationRequest{Policy: policy}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("PATCH", c.baseURL+"/simulations/"+simID, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result HALResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// DecomposeRequest is the request body for ticket decomposition.
type DecomposeRequest struct {
	TicketID string `json:"ticketId"`
}

// DecomposeResponse is the response from decomposing a ticket.
type DecomposeResponse struct {
	Decomposed bool              `json:"decomposed"`
	Children   []TicketState     `json:"children,omitempty"`
	Simulation SimulationState   `json:"simulation"`
	Links      map[string]string `json:"_links"`
}

// Decompose attempts to decompose a ticket into smaller tasks.
func (c Client) Decompose(simID, ticketID string) (*DecomposeResponse, error) {
	body := DecomposeRequest{TicketID: ticketID}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/simulations/"+simID+"/decompose", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result DecomposeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

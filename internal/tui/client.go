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
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new HTTP client for the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
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

// SimulationState mirrors api.SimulationState for client-side decoding.
type SimulationState struct {
	ID          string `json:"id"`
	CurrentTick int    `json:"currentTick"`
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
func (c *Client) CreateSimulation(seed int64, policy string) (*CreateSimulationResponse, error) {
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
func (c *Client) Tick(simID string) (*TickResponse, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/simulations/"+simID+"/tick", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
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
func (c *Client) Assign(simID, ticketID, devID string) error {
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
func (c *Client) StartSprint(simID string) error {
	req, err := http.NewRequest("POST", c.baseURL+"/simulations/"+simID+"/sprints", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
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

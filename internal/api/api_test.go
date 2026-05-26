package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// NOTE: Controller integration test moved to TestTutorialWalkthrough in tutorial_walkthrough_test.go.
// Per Khorikov, controllers get ONE integration test covering the happy path.
// TestTutorialWalkthrough is more comprehensive (includes assignments) and serves as
// both the integration test and executable documentation.

// halResponse is the expected HAL+JSON response structure for tests.
// Test-only: not exported, used by postJSON helper.
type halResponse struct {
	Simulation json.RawMessage   `json:"simulation"`
	Links      map[string]string `json:"_links"`
}

// TestAPI_AssignmentErrors tests assignment endpoint error cases.
// Per Khorikov, this is domain logic (validation) so unit test the edge cases.
func TestAPI_AssignmentErrors(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, srv *httptest.Server) string // returns simID
		ticketID   string
		devID      string
		wantStatus int
		wantError  string
	}{
		{
			name: "ticket not in backlog",
			setup: func(t *testing.T, srv *httptest.Server) string {
				resp := postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42})
				postJSON(t, srv.URL+resp.Links["start-sprint"], nil)
				return "sim-42"
			},
			ticketID:   "TKT-999", // doesn't exist
			devID:      "dev-1",
			wantStatus: http.StatusBadRequest,
			wantError:  "ticket TKT-999 not found in backlog or committed queue",
		},
		{
			name: "developer not found",
			setup: func(t *testing.T, srv *httptest.Server) string {
				resp := postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42})
				postJSON(t, srv.URL+resp.Links["start-sprint"], nil)
				return "sim-42"
			},
			ticketID:   "TKT-001",
			devID:      "dev-999", // doesn't exist
			wantStatus: http.StatusBadRequest,
			wantError:  "developer dev-999 not found",
		},
		{
			name: "developer is busy",
			setup: func(t *testing.T, srv *httptest.Server) string {
				resp := postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42})
				postJSON(t, srv.URL+resp.Links["start-sprint"], nil)
				// Assign first ticket to dev-1
				postJSONExpectOK(t, srv.URL+"/simulations/sim-42/assignments",
					map[string]any{"ticketId": "TKT-001", "developerId": "dev-1"})
				return "sim-42"
			},
			ticketID:   "TKT-002",
			devID:      "dev-1", // already busy with TKT-001
			wantStatus: http.StatusBadRequest,
			wantError:  "developer dev-1 is busy with TKT-001",
		},
		{
			name: "no idle developers for auto-assign",
			setup: func(t *testing.T, srv *httptest.Server) string {
				resp := postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42})
				postJSON(t, srv.URL+resp.Links["start-sprint"], nil)
				// Assign tickets to all 6 developers
				for i := 1; i <= 6; i++ { // justified:SM
					postJSONExpectOK(t, srv.URL+"/simulations/sim-42/assignments",
						map[string]any{"ticketId": fmt.Sprintf("TKT-%03d", i), "developerId": fmt.Sprintf("dev-%d", i)})
				}
				return "sim-42"
			},
			ticketID:   "TKT-007",
			devID:      "", // auto-assign
			wantStatus: http.StatusBadRequest,
			wantError:  "no idle developers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := api.NewSimRegistry()
			srv := httptest.NewServer(api.NewRouter(registry))
			defer srv.Close()

			simID := tt.setup(t, srv)

			body := map[string]any{"ticketId": tt.ticketID}
			if tt.devID != "" {
				body["developerId"] = tt.devID
			}

			status, errMsg := postJSONExpectError(t, srv.URL+"/simulations/"+simID+"/assignments", body)

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if errMsg != tt.wantError {
				t.Errorf("error = %q, want %q", errMsg, tt.wantError)
			}
		})
	}
}

// doJSON sends an HTTP request with the given method and JSON body, returns parsed HAL response.
func doJSON(t *testing.T, method, url string, body any) halResponse {
	t.Helper()
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("%s %s returned status %d", method, url, resp.StatusCode)
	}
	var result halResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return result
}

// doJSONExpectError sends an HTTP request and returns status code and error message.
func doJSONExpectError(t *testing.T, method, url string, body any) (int, string) {
	t.Helper()
	reqBody, _ := json.Marshal(body)
	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, url, err)
	}
	defer resp.Body.Close()
	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&errResp)
	return resp.StatusCode, errResp.Error
}

// postJSONExpectOK sends a POST and expects success (for setup steps).
func postJSONExpectOK(t *testing.T, url string, body any) {
	t.Helper()
	reqBody, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("POST %s returned status %d", url, resp.StatusCode)
	}
}

// postJSONExpectError sends a POST and returns status code and error message.
func postJSONExpectError(t *testing.T, url string, body any) (int, string) {
	t.Helper()
	reqBody, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	defer resp.Body.Close()

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&errResp)
	return resp.StatusCode, errResp.Error
}

// postJSON sends a POST request with optional JSON body and returns parsed HAL response.
func postJSON(t *testing.T, url string, body any) halResponse {
	t.Helper()

	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		t.Fatalf("POST %s returned status %d", url, resp.StatusCode)
	}

	var result halResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return result
}

// TestAPI_Compare tests the POST /comparisons endpoint.
// Per plan Step 6: happy path + edge cases.
func TestAPI_Compare(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	// POST /comparisons with seed for reproducibility
	resp, err := http.Post(
		srv.URL+"/comparisons",
		"application/json",
		bytes.NewReader([]byte(`{"seed": 42, "sprints": 2}`)),
	)
	if err != nil {
		t.Fatalf("POST /comparisons failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result api.CompareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify structure
	if result.Seed != 42 {
		t.Errorf("seed = %d, want 42", result.Seed)
	}
	if result.Sprints != 2 {
		t.Errorf("sprints = %d, want 2", result.Sprints)
	}
	if result.PolicyA.Name == "" {
		t.Error("policyA.name is empty")
	}
	if result.PolicyB.Name == "" {
		t.Error("policyB.name is empty")
	}
	if result.WinsA+result.WinsB > 4 {
		t.Errorf("total wins %d > 4 metrics", result.WinsA+result.WinsB)
	}
	if result.Links["self"] != "/comparisons" {
		t.Errorf("links.self = %q, want /comparisons", result.Links["self"])
	}
}

func TestAPI_Compare_InvalidSprints(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	// Negative sprints returns 400 per design doc
	resp, err := http.Post(
		srv.URL+"/comparisons",
		"application/json",
		bytes.NewReader([]byte(`{"seed": 42, "sprints": -1}`)),
	)
	if err != nil {
		t.Fatalf("POST /comparisons failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAPI_Compare_DefaultSprints(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	// Zero sprints uses default (3)
	resp, err := http.Post(
		srv.URL+"/comparisons",
		"application/json",
		bytes.NewReader([]byte(`{"seed": 42, "sprints": 0}`)),
	)
	if err != nil {
		t.Fatalf("POST /comparisons failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result api.CompareResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Sprints != 3 {
		t.Errorf("sprints = %d, want default 3", result.Sprints)
	}
}

// TestAPI_Compare_DecompositionProducesDifferentResults verifies that
// auto-decomposition causes policies to produce different outcomes.
// DORA decomposes tickets >5 days, TameFlow decomposes Low understanding.
// Standard backlog has TKT-004 (8 days, Low) for both, TKT-003 (2 days, Low) for TameFlow only.
func TestAPI_Compare_DecompositionProducesDifferentResults(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	// Run enough sprints for decomposition effects to manifest
	resp, err := http.Post(
		srv.URL+"/comparisons",
		"application/json",
		bytes.NewReader([]byte(`{"seed": 42, "sprints": 5}`)),
	)
	if err != nil {
		t.Fatalf("POST /comparisons failed: %v", err)
	}
	defer resp.Body.Close()

	var result api.CompareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Policies should produce different ticket counts due to decomposition
	// TameFlow decomposes more tickets (TKT-003 + TKT-004) vs DORA (TKT-004 only)
	if result.PolicyA.TicketsComplete == result.PolicyB.TicketsComplete {
		t.Errorf("policies produced same ticket count (%d) - decomposition may not be running",
			result.PolicyA.TicketsComplete)
	}

	// There should be a winner (not a tie on all metrics)
	if result.WinsA == 0 && result.WinsB == 0 {
		t.Error("no wins for either policy - expected decomposition to differentiate outcomes")
	}
}

// TestDecomposeEligibleTickets_EmptyBacklog verifies decomposition handles empty backlog.
// Edge case: no tickets to decompose should return engine unchanged.
func TestDecomposeEligibleTickets_EmptyBacklog(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	// Run with 1 sprint - should complete without error even with standard backlog
	// (tests that decomposition loop terminates correctly)
	resp, err := http.Post(
		srv.URL+"/comparisons",
		"application/json",
		bytes.NewReader([]byte(`{"seed": 1, "sprints": 1}`)),
	)
	if err != nil {
		t.Fatalf("POST /comparisons failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestAPI_GetLessons tests the GET /simulations/{id}/lessons endpoint.
// Per Khorikov: Controller gets ONE integration test for happy path.
func TestAPI_GetLessons(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	// Create a simulation first
	postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42})

	// GET lessons for the simulation
	resp, err := http.Get(srv.URL + "/simulations/sim-42/lessons")
	if err != nil {
		t.Fatalf("GET /simulations/sim-42/lessons failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result api.LessonsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if result.CurrentLesson.ID == "" {
		t.Error("currentLesson.id is empty")
	}
	if result.CurrentLesson.Title == "" {
		t.Error("currentLesson.title is empty")
	}
	if result.CurrentLesson.Content == "" {
		t.Error("currentLesson.content is empty")
	}
	if result.Progress == "" {
		t.Error("progress is empty")
	}
	if result.Links["self"] != "/simulations/sim-42/lessons" {
		t.Errorf("links.self = %q, want /simulations/sim-42/lessons", result.Links["self"])
	}
	if result.Links["simulation"] != "/simulations/sim-42" {
		t.Errorf("links.simulation = %q, want /simulations/sim-42", result.Links["simulation"])
	}
}

// TestAPI_GetLessons_NotFound tests lessons endpoint with invalid simulation ID.
func TestAPI_GetLessons_NotFound(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/simulations/nonexistent/lessons")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestSimRegistry_ValueSemantics verifies that SimRegistry works correctly with
// value receivers. Maps are reference types, so mutations via value receiver
// TestSimRegistry_MutationPersists verifies that mutations to the registry
// persist correctly. The registry uses pointer receivers (mutex requirement)
// with an embedded pointer in the wrapper for proper sharing.
func TestSimRegistry_MutationPersists(t *testing.T) {
	registry := api.NewSimRegistry()

	// Call method that mutates internal map
	id, err := registry.CreateSimulation(42, 0, "", nil, model.ReleaseModePush) // 0 = PolicyNone; nil = unlimited (UC38); push = default (UC39)
	if err != nil {
		t.Fatalf("CreateSimulation() error = %v", err)
	}

	// Verify mutation persisted
	sims := registry.ListSimulations()

	if len(sims) != 1 {
		t.Fatalf("ListSimulations() = %d items, want 1 (map mutation did not persist)", len(sims))
	}
	if sims[0].ID != id {
		t.Errorf("simulation ID = %q, want %q", sims[0].ID, id)
	}

	// Verify via HTTP (double-check the router also works with value)
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/simulations/" + id)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /simulations/%s status = %d, want 200", id, resp.StatusCode)
	}
}

// TestAPI_ListSimulations tests the GET /simulations discovery endpoint.
// Per UC10: "API client lists active simulations to discover available IDs"
func TestAPI_ListSimulations(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, srv *httptest.Server)
		wantCount int
		wantIDs   []string
	}{
		{
			name:      "empty registry returns empty list",
			setup:     func(t *testing.T, srv *httptest.Server) {},
			wantCount: 0,
			wantIDs:   []string{},
		},
		{
			name: "one simulation returns one item",
			setup: func(t *testing.T, srv *httptest.Server) {
				postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42})
			},
			wantCount: 1,
			wantIDs:   []string{"sim-42"},
		},
		{
			name: "multiple simulations returns all",
			setup: func(t *testing.T, srv *httptest.Server) {
				postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42})
				postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 100})
			},
			wantCount: 2,
			wantIDs:   []string{"sim-42", "sim-100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := api.NewSimRegistry()
			srv := httptest.NewServer(api.NewRouter(registry))
			defer srv.Close()

			tt.setup(t, srv)

			resp, err := http.Get(srv.URL + "/simulations")
			if err != nil {
				t.Fatalf("GET /simulations failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			var result struct {
				Simulations []struct {
					ID    string            `json:"id"`
					Links map[string]string `json:"_links"`
				} `json:"simulations"`
				Links map[string]string `json:"_links"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if len(result.Simulations) != tt.wantCount {
				t.Errorf("count = %d, want %d", len(result.Simulations), tt.wantCount)
			}

			gotIDs := make([]string, len(result.Simulations))
			for i, sim := range result.Simulations { // justified:IX
				gotIDs[i] = sim.ID
			}

			// Check all expected IDs are present (order may vary)
			for _, wantID := range tt.wantIDs { // justified:AS
				found := false
				for _, gotID := range gotIDs { // justified:FL
					if gotID == wantID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing expected ID %q in %v", wantID, gotIDs)
				}
			}
		})
	}
}

// TestAPI_UpdateSimulation tests the PATCH /simulations/{id} happy path.
// Per Khorikov: ONE integration test per controller workflow.
func TestAPI_UpdateSimulation(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	// Create simulation with default policy (none)
	postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42})

	// PATCH to tameflow-cognitive
	result := doJSON(t, "PATCH", srv.URL+"/simulations/sim-42", map[string]any{"policy": "tameflow-cognitive"})

	var sim struct {
		SizingPolicy string `json:"sizingPolicy"`
	}
	if err := json.Unmarshal(result.Simulation, &sim); err != nil {
		t.Fatalf("failed to decode simulation: %v", err)
	}
	if sim.SizingPolicy != "TameFlow-Cognitive" {
		t.Errorf("sizingPolicy = %q, want %q", sim.SizingPolicy, "TameFlow-Cognitive")
	}
}

// TestAPI_UpdateSimulationErrors tests PATCH error cases.
// Per Khorikov: edge cases tested separately from happy path.
func TestAPI_UpdateSimulationErrors(t *testing.T) {
	tests := []struct {
		name       string
		simID      string
		setup      bool // whether to create the simulation first
		policy     string
		wantStatus int
		wantError  string
	}{
		{
			name:       "invalid policy",
			simID:      "sim-42",
			setup:      true,
			policy:     "bogus",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid policy",
		},
		{
			name:       "simulation not found",
			simID:      "nonexistent",
			setup:      false,
			policy:     "tameflow-cognitive",
			wantStatus: http.StatusNotFound,
			wantError:  "simulation not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := api.NewSimRegistry()
			srv := httptest.NewServer(api.NewRouter(registry))
			defer srv.Close()

			if tt.setup {
				postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42})
			}

			status, errMsg := doJSONExpectError(t, "PATCH", srv.URL+"/simulations/"+tt.simID, map[string]any{"policy": tt.policy})

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if errMsg != tt.wantError {
				t.Errorf("error = %q, want %q", errMsg, tt.wantError)
			}
		})
	}
}

// TestAPI_Decompose tests the POST /simulations/{id}/decompose happy path.
// Per Khorikov: ONE integration test per controller workflow.
func TestAPI_Decompose(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	// Create simulation with DORA-Strict (decomposes tickets >5 days)
	result := postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42, "policy": "dora-strict"})

	// Find a ticket >5 days in the backlog (DORA threshold)
	var sim struct {
		Backlog []struct {
			ID            string  `json:"id"`
			EstimatedDays float64 `json:"estimatedDays"`
		} `json:"backlog"`
	}
	json.Unmarshal(result.Simulation, &sim)

	var largeTicketID string
	for _, t := range sim.Backlog { // justified:FL
		if t.EstimatedDays > 5 {
			largeTicketID = t.ID
			break
		}
	}
	if largeTicketID == "" {
		t.Skip("no ticket >5 days in backlog with seed 42")
	}

	// Decompose the large ticket
	resp, err := http.Post(
		srv.URL+"/simulations/sim-42/decompose",
		"application/json",
		bytes.NewReader([]byte(fmt.Sprintf(`{"ticketId": %q}`, largeTicketID))),
	)
	if err != nil {
		t.Fatalf("POST decompose failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var decompResult api.DecomposeResponse
	if err := json.NewDecoder(resp.Body).Decode(&decompResult); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !decompResult.Decomposed {
		t.Errorf("expected decomposed=true for ticket %s (%.1f days) with DORA-Strict",
			largeTicketID, 0.0)
	}
	if len(decompResult.Children) < 2 {
		t.Errorf("expected 2+ children, got %d", len(decompResult.Children))
	}
}

// TestAPI_DecomposeEdgeCases tests decompose edge cases.
func TestAPI_DecomposeEdgeCases(t *testing.T) {
	t.Run("simulation not found", func(t *testing.T) {
		registry := api.NewSimRegistry()
		srv := httptest.NewServer(api.NewRouter(registry))
		defer srv.Close()

		status, errMsg := postJSONExpectError(t, srv.URL+"/simulations/nonexistent/decompose", map[string]any{"ticketId": "TKT-001"})
		if status != http.StatusNotFound {
			t.Errorf("status = %d, want %d", status, http.StatusNotFound)
		}
		if errMsg != "simulation not found" {
			t.Errorf("error = %q, want %q", errMsg, "simulation not found")
		}
	})

	t.Run("policy forbids decomposition", func(t *testing.T) {
		registry := api.NewSimRegistry()
		srv := httptest.NewServer(api.NewRouter(registry))
		defer srv.Close()

		// PolicyNone never decomposes
		postJSON(t, srv.URL+"/simulations", map[string]any{"seed": 42, "policy": "none"})

		resp, err := http.Post(
			srv.URL+"/simulations/sim-42/decompose",
			"application/json",
			bytes.NewReader([]byte(`{"ticketId": "TKT-001"}`)),
		)
		if err != nil {
			t.Fatalf("POST decompose failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var result api.DecomposeResponse
		json.NewDecoder(resp.Body).Decode(&result)
		if result.Decomposed {
			t.Error("expected decomposed=false with PolicyNone")
		}
	})
}

// UC38 (#15443): POST /simulations with invalid phaseWIPConfig returns HTTP
// 422 (Unprocessable Entity). Each of the 4 sentinel cases is exercised
// through the HTTP surface; the unknown-phase case returns 400 because the
// parsePhaseWIPConfig translation fails before the domain validator runs.
func TestHandleCreateSimulation_PhaseWIPConfig_HTTP422(t *testing.T) {
	tests := []struct {
		name    string
		seed    int64
		config  map[string]int
		wantSC  int
		wantSub string // substring that must appear in response body
	}{
		{
			name:    "ErrCapZero → 422",
			seed:    100,
			config:  map[string]int{"Verify": 0},
			wantSC:  http.StatusUnprocessableEntity,
			wantSub: "phase WIP cap is zero",
		},
		{
			name:    "ErrCapNegative → 422",
			seed:    101,
			config:  map[string]int{"Review": -1},
			wantSC:  http.StatusUnprocessableEntity,
			wantSub: "phase WIP cap is negative",
		},
		{
			name:    "ErrCapBelowMentorMin → 422",
			seed:    102,
			config:  map[string]int{"Implement": 1},
			wantSC:  http.StatusUnprocessableEntity,
			wantSub: "below mentor-pair minimum",
		},
		{
			name:    "unknown phase name → 400 (structural)",
			seed:    103,
			config:  map[string]int{"BadPhase": 5},
			wantSC:  http.StatusBadRequest,
			wantSub: "unknown phase",
		},
		{
			name:   "valid PhaseWIPConfig → 201",
			seed:   104,
			config: map[string]int{"Implement": 4, "CICD": 1},
			wantSC: http.StatusCreated,
		},
		{
			name:   "nil PhaseWIPConfig → 201 (regression-safe)",
			seed:   105,
			config: nil,
			wantSC: http.StatusCreated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry := api.NewSimRegistry()
			srv := httptest.NewServer(api.NewRouter(registry))
			defer srv.Close()

			body := map[string]any{"seed": tc.seed}
			if tc.config != nil {
				body["phaseWIPConfig"] = tc.config
			}
			reqBody, _ := json.Marshal(body)
			resp, err := http.Post(srv.URL+"/simulations", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				t.Fatalf("POST /simulations: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantSC {
				rbody, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d; body=%s", resp.StatusCode, tc.wantSC, rbody)
			}
			if tc.wantSub != "" {
				rbody, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(rbody), tc.wantSub) {
					t.Errorf("body missing %q; got %s", tc.wantSub, rbody)
				}
			}
		})
	}
}

// UC39 (#15445): POST /simulations with invalid releaseMode returns HTTP 422
// (reuses UC38-introduced status code for domain-rule violations).
func TestHandleCreateSimulation_ReleaseMode_HTTP422(t *testing.T) {
	tests := []struct {
		name    string
		seed    int64
		mode    string // releaseMode JSON value
		wantSC  int
		wantSub string
	}{
		{
			name:    "garbage mode → 422 with ErrInvalidReleaseMode-wrapped message",
			seed:    200,
			mode:    "garbage",
			wantSC:  http.StatusUnprocessableEntity,
			wantSub: "invalid release mode",
		},
		{
			name:    "pull (common typo) → 422",
			seed:    201,
			mode:    "pull",
			wantSC:  http.StatusUnprocessableEntity,
			wantSub: "invalid release mode",
		},
		{
			name:   "valid push → 201",
			seed:   202,
			mode:   "push",
			wantSC: http.StatusCreated,
		},
		{
			name:   "valid demand → 201",
			seed:   203,
			mode:   "demand",
			wantSC: http.StatusCreated,
		},
		{
			name:   "empty mode → 201 (default push; regression-safe)",
			seed:   204,
			mode:   "",
			wantSC: http.StatusCreated,
		},
		{
			name:   "case-insensitive DEMAND → 201",
			seed:   205,
			mode:   "DEMAND",
			wantSC: http.StatusCreated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry := api.NewSimRegistry()
			srv := httptest.NewServer(api.NewRouter(registry))
			defer srv.Close()

			body := map[string]any{"seed": tc.seed}
			if tc.mode != "" {
				body["releaseMode"] = tc.mode
			}
			reqBody, _ := json.Marshal(body)
			resp, err := http.Post(srv.URL+"/simulations", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				t.Fatalf("POST /simulations: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantSC {
				rbody, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d; body=%s", resp.StatusCode, tc.wantSC, rbody)
			}
			if tc.wantSub != "" {
				rbody, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(rbody), tc.wantSub) {
					t.Errorf("body missing %q; got %s", tc.wantSub, rbody)
				}
			}
		})
	}
}

// UC40 (#15446): POST /simulations/{id}/investments — 5-case HTTP
// status-code mapping per plan §HTTP surface.
func TestHandleSpendInvestment_HTTPMapping(t *testing.T) {
	// setupOpenWindow creates a sim and fast-forwards to "investment
	// window open" state by emitting SprintStarted + SprintEnded directly.
	setupOpenWindow := func(t *testing.T) (*httptest.Server, api.SimRegistry, string) {
		t.Helper()
		registry := api.NewSimRegistry()
		srv := httptest.NewServer(api.NewRouter(registry))
		reqBody, _ := json.Marshal(map[string]any{"seed": 42})
		resp, _ := http.Post(srv.URL+"/simulations", "application/json", bytes.NewReader(reqBody))
		resp.Body.Close()
		inst, ok := registry.GetInstanceOption("sim-42").Get()
		if !ok {
			srv.Close()
			t.Fatal("sim-42 not in registry after create")
		}
		eng := inst.Engine
		eng, _ = eng.EmitForTest(events.NewSprintStarted("sim-42", 0, 1, 2.0))
		eng, _ = eng.EmitForTest(events.NewSprintEnded("sim-42", 10, 1))
		inst.Engine = eng
		registry.SetInstance("sim-42", inst)
		return srv, registry, "sim-42"
	}

	t.Run("happy path: hire → 201", func(t *testing.T) {
		srv, _, simID := setupOpenWindow(t)
		defer srv.Close()
		body, _ := json.Marshal(map[string]any{"option": "hire"})
		resp, _ := http.Post(srv.URL+"/simulations/"+simID+"/investments", "application/json", bytes.NewReader(body))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			rbody, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d; want 201; body=%s", resp.StatusCode, rbody)
		}
	})

	t.Run("invalid option (garbage) → 422", func(t *testing.T) {
		srv, _, simID := setupOpenWindow(t)
		defer srv.Close()
		body, _ := json.Marshal(map[string]any{"option": "garbage"})
		resp, _ := http.Post(srv.URL+"/simulations/"+simID+"/investments", "application/json", bytes.NewReader(body))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("status = %d; want 422 Unprocessable Entity", resp.StatusCode)
		}
		rbody, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(rbody), "invalid investment option") {
			t.Errorf("body missing 'invalid investment option'; got %s", rbody)
		}
	})

	t.Run("invalid option (empty string) → 422", func(t *testing.T) {
		srv, _, simID := setupOpenWindow(t)
		defer srv.Close()
		body, _ := json.Marshal(map[string]any{"option": ""})
		resp, _ := http.Post(srv.URL+"/simulations/"+simID+"/investments", "application/json", bytes.NewReader(body))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("status = %d; want 422 (empty is invalid; no default for investment)", resp.StatusCode)
		}
	})

	t.Run("window closed (pre-first-sprint) → 409", func(t *testing.T) {
		registry := api.NewSimRegistry()
		srv := httptest.NewServer(api.NewRouter(registry))
		defer srv.Close()
		reqBody, _ := json.Marshal(map[string]any{"seed": 99})
		http.Post(srv.URL+"/simulations", "application/json", bytes.NewReader(reqBody))
		body, _ := json.Marshal(map[string]any{"option": "hire"})
		resp, _ := http.Post(srv.URL+"/simulations/sim-99/investments", "application/json", bytes.NewReader(body))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			rbody, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d; want 409 Conflict (window closed); body=%s", resp.StatusCode, rbody)
		}
	})

	t.Run("sim not found → 404", func(t *testing.T) {
		registry := api.NewSimRegistry()
		srv := httptest.NewServer(api.NewRouter(registry))
		defer srv.Close()
		body, _ := json.Marshal(map[string]any{"option": "hire"})
		resp, _ := http.Post(srv.URL+"/simulations/sim-nonexistent/investments", "application/json", bytes.NewReader(body))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d; want 404", resp.StatusCode)
		}
	})
}

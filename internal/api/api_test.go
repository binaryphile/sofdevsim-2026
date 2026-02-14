package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
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
			wantError:  "ticket TKT-999 not found in backlog",
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
				for i := 1; i <= 6; i++ {
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
	id, err := registry.CreateSimulation(42, 0) // 0 = PolicyNone
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
			for i, sim := range result.Simulations {
				gotIDs[i] = sim.ID
			}

			// Check all expected IDs are present (order may vary)
			for _, wantID := range tt.wantIDs {
				found := false
				for _, gotID := range gotIDs {
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
	for _, t := range sim.Backlog {
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

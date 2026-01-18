package api_test

import (
	"bytes"
	"encoding/json"
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
				// Assign tickets to all 3 developers
				postJSONExpectOK(t, srv.URL+"/simulations/sim-42/assignments",
					map[string]any{"ticketId": "TKT-001", "developerId": "dev-1"})
				postJSONExpectOK(t, srv.URL+"/simulations/sim-42/assignments",
					map[string]any{"ticketId": "TKT-002", "developerId": "dev-2"})
				postJSONExpectOK(t, srv.URL+"/simulations/sim-42/assignments",
					map[string]any{"ticketId": "TKT-003", "developerId": "dev-3"})
				return "sim-42"
			},
			ticketID:   "TKT-004",
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

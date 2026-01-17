package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
	"github.com/google/go-cmp/cmp"
)

// TestAPI_SprintLifecycle tests the complete simulation lifecycle through HTTP.
// Per Khorikov, controllers get ONE integration test covering the happy path.
// This test verifies HATEOAS behavior: links change based on simulation state.
//
// The test creates a simulation, starts a sprint, ticks until the sprint ends,
// and verifies that the tick link disappears while the start-sprint link appears.
// This validates the hypermedia-driven API contract without testing internal state.
//
// Dependencies use in-memory replacements:
// - httptest.NewServer for HTTP (random port, no conflicts)
// - Fresh SimRegistry per test (no shared state)
//
// No external boundaries to mock (no DB, no email, no third-party APIs).
// Test is parallel-safe: each test case has isolated state.
func TestAPI_SprintLifecycle(t *testing.T) {
	type want struct {
		HasTickLink        bool
		HasStartSprintLink bool
	}

	tests := []struct {
		name string
		seed int64
		want want
	}{
		{
			name: "sprint lifecycle ends correctly",
			seed: 42,
			want: want{HasTickLink: false, HasStartSprintLink: true},
		},
		{
			name: "different seed same behavior",
			seed: 99,
			want: want{HasTickLink: false, HasStartSprintLink: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := api.NewSimRegistry()
			srv := httptest.NewServer(api.NewRouter(registry))
			defer srv.Close()

			// Create simulation
			resp := postJSON(t, srv.URL+"/simulations", map[string]any{"seed": tt.seed})

			// Start sprint
			resp = postJSON(t, srv.URL+resp.Links["start-sprint"], nil)

			// Tick until sprint ends (tick link disappears)
			for resp.Links["tick"] != "" {
				resp = postJSON(t, srv.URL+resp.Links["tick"], nil)
			}

			got := want{
				HasTickLink:        resp.Links["tick"] != "",
				HasStartSprintLink: resp.Links["start-sprint"] != "",
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("lifecycle mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

// halResponse is the expected HAL+JSON response structure for tests.
// Test-only: not exported, used by postJSON helper.
type halResponse struct {
	Simulation json.RawMessage   `json:"simulation"`
	Links      map[string]string `json:"_links"`
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

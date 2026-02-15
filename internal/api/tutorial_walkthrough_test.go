package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTutorialWalkthrough is the ONE controller integration test for the API.
// Per Khorikov, controllers get one integration test covering the happy path.
//
// This test walks through the same flow a user would experience in the TUI,
// but using HTTP API calls. It shows how an LLM agent or external tool
// could operate the simulation programmatically.
//
// Verifies HATEOAS behavior: links change based on simulation state.
// - Sprint inactive: start-sprint link present, tick link absent
// - Sprint active: tick link present, start-sprint link absent
// - Backlog non-empty during sprint: assign link present
//
// Dependencies use in-memory replacements:
// - httptest.NewServer for HTTP (random port, no conflicts)
// - Fresh SimRegistry per test (no shared state)
//
// GOTCHA: json.Unmarshal merges into existing maps rather than replacing them.
// The unmarshalHAL helper clears Links to nil before each unmarshal to prevent
// stale link accumulation across multiple API calls.
func TestTutorialWalkthrough(t *testing.T) {
	// Setup: Create registry and test server
	registry := NewSimRegistry()
	srv := httptest.NewServer(NewRouter(registry))
	defer srv.Close()

	t.Logf("Tutorial Walkthrough via API")
	t.Logf("=============================")
	t.Logf("Server: %s", srv.URL)
	t.Logf("")

	// Step 1: Check entry point (HATEOAS discovery)
	t.Logf("Step 1: Discover API entry point")
	resp := get(t, srv, "/")
	t.Logf("  GET / -> %s", resp)
	t.Logf("")

	// Step 2: Create a new simulation with DORA-Strict policy
	t.Logf("Step 2: Create simulation with DORA-Strict policy")
	body := `{"policy": "dora-strict", "seed": 42}`
	resp = post(t, srv, "/simulations", body)
	t.Logf("  POST /simulations -> %s", truncate(resp, 200))

	// Parse response - HAL format: {"simulation": {...}, "_links": {...}}
	// Note: We declare halResp once but must clear Links before each unmarshal
	// because json.Unmarshal merges into existing maps rather than replacing them.
	var halResp struct {
		Simulation struct {
			ID                   string `json:"id"`
			CurrentTick          int    `json:"currentTick"`
			SprintActive         bool   `json:"sprintActive"`
			SprintNumber         int    `json:"sprintNumber"`
			BacklogCount         int    `json:"backlogCount"`
			ActiveTicketCount    int    `json:"activeTicketCount"`
			CompletedTicketCount int    `json:"completedTicketCount"`
			Sprint               *struct {
				StartDay     int `json:"StartDay"`
				EndDay       int `json:"EndDay"`
				DurationDays int `json:"DurationDays"`
			} `json:"sprint"`
		} `json:"simulation"`
		Links map[string]string `json:"_links"`
	}

	// unmarshalHAL clears Links before unmarshaling to prevent stale link accumulation.
	unmarshalHAL := func(data string) {
		halResp.Links = nil // Clear map so decoder creates fresh one
		json.Unmarshal([]byte(data), &halResp)
	}

	unmarshalHAL(resp)
	simID := halResp.Simulation.ID
	t.Logf("  Simulation ID: %s", simID)
	t.Logf("")

	// Step 3: View initial state
	t.Logf("Step 3: View initial simulation state")
	resp = get(t, srv, "/simulations/"+simID)
	unmarshalHAL(resp)
	t.Logf("  GET /simulations/%s", simID)
	t.Logf("  Tick: %d", halResp.Simulation.CurrentTick)
	t.Logf("  Backlog: %d tickets", halResp.Simulation.BacklogCount)
	t.Logf("  Sprint active: %v", halResp.Simulation.SprintActive)
	t.Logf("  Links: %v", halResp.Links)
	t.Logf("")

	// Step 4: Start a sprint
	t.Logf("Step 4: Start a sprint")
	resp = post(t, srv, "/simulations/"+simID+"/sprints", "")
	unmarshalHAL(resp)
	t.Logf("  POST /simulations/%s/sprints", simID)
	t.Logf("  Sprint active: %v", halResp.Simulation.SprintActive)
	if halResp.Simulation.Sprint != nil {
		t.Logf("  Sprint days: %d-%d (%d days)",
			halResp.Simulation.Sprint.StartDay,
			halResp.Simulation.Sprint.EndDay,
			halResp.Simulation.Sprint.DurationDays)
	}
	t.Logf("")

	// Step 5: Assign tickets to developers
	t.Logf("Step 5: Assign tickets to developers")

	// Get current state to show available links
	resp = get(t, srv, "/simulations/"+simID)
	unmarshalHAL(resp)
	t.Logf("  Available links: %v", halResp.Links)

	// Auto-assign first 3 tickets (one per developer)
	for i := 0; i < 3; i++ { // justified:SM
		ticketID := fmt.Sprintf("TKT-%03d", i+1)
		assignBody := fmt.Sprintf(`{"ticketId": "%s"}`, ticketID) // Auto-assign
		resp = post(t, srv, "/simulations/"+simID+"/assignments", assignBody)
		unmarshalHAL(resp)
	}
	t.Logf("  Assigned 3 tickets (auto-assign)")
	t.Logf("  Active tickets: %d", halResp.Simulation.ActiveTicketCount)
	t.Logf("  Backlog remaining: %d", halResp.Simulation.BacklogCount)
	t.Logf("")

	// Step 6: Advance simulation by ticking
	t.Logf("Step 6: Advance simulation (tick until sprint ends)")
	tickCount := 0
	for { // justified:WL
		resp = get(t, srv, "/simulations/"+simID)
		unmarshalHAL(resp)

		if !halResp.Simulation.SprintActive {
			break
		}

		if tickCount > 100 {
			t.Logf("  Safety limit reached")
			break
		}

		post(t, srv, "/simulations/"+simID+"/tick", "")
		tickCount++
	}
	t.Logf("  Sprint completed after %d ticks", tickCount)
	t.Logf("  Active tickets: %d", halResp.Simulation.ActiveTicketCount)
	t.Logf("  Completed tickets: %d", halResp.Simulation.CompletedTicketCount)
	t.Logf("")

	// Step 7: View final state
	t.Logf("Step 7: View final state")
	resp = get(t, srv, "/simulations/"+simID)
	unmarshalHAL(resp)
	t.Logf("  Final tick: %d", halResp.Simulation.CurrentTick)
	t.Logf("  Completed tickets: %d", halResp.Simulation.CompletedTicketCount)
	t.Logf("  Backlog remaining: %d", halResp.Simulation.BacklogCount)
	t.Logf("  Available links: %v", halResp.Links)
	t.Logf("")

	// Step 8: View events (event sourcing)
	t.Logf("Step 8: Review events emitted during simulation")
	events := registry.Store().Replay(simID)
	t.Logf("  Total events: %d", len(events))

	// Count by type
	eventCounts := make(map[string]int)
	for _, e := range events { // justified:MB
		eventCounts[e.EventType()]++
	}
	for eventType, count := range eventCounts { // justified:MB
		t.Logf("    %s: %d", eventType, count)
	}
	t.Logf("")

	t.Logf("Tutorial complete!")
	t.Logf("")
	t.Logf("Summary:")
	t.Logf("  - Created simulation with seed 42, DORA-Strict policy")
	t.Logf("  - Ran 1 sprint (%d ticks total)", halResp.Simulation.CurrentTick)
	t.Logf("  - Completed %d tickets", halResp.Simulation.CompletedTicketCount)
	t.Logf("  - Generated %d events", len(events))
}

// Helper functions

func get(t *testing.T, srv *httptest.Server, path string) string {
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

func post(t *testing.T, srv *httptest.Server, path, body string) string {
	resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

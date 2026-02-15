package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// TestSprintWalkthrough is the second controller integration test for the API.
// Per Khorikov §8.1.3, this covers a different business scenario than
// TestTutorialWalkthrough: office animation milestones via /office endpoint,
// whereas the tutorial test covers HATEOAS link presence.
//
// The /office endpoint is the observation mechanism, not the subject under test.
// Phase 7 excluded HandleGetOffice from standalone handler testing (Q3).
// This test is different: it tests office state *transitions across a workflow*
// (conference → working → frustrated/idle → conference).
//
// 7 milestones from design doc (UC9+UC10+UC35 intersection):
//  1. Create simulation — backlog > 0, sprint inactive
//  2. Initial office — all 6 devs in "conference"
//  3. Assign tickets — devs transition to "working" with ticketIds
//  4. Start sprint — sprintActive=true, tick link present
//  5. Tick through sprint — observe "frustrated" or "idle" states
//  6. Sprint ends — all devs return to "conference"
//  7. Final state — completedTicketCount > 0, events emitted
//
// Live mode: Set SOFDEVSIM_URL to target a running server.
// Enables UC10 scenario: operator watches TUI while test drives via REST.
func TestSprintWalkthrough(t *testing.T) {
	ts := setupServer(t)

	t.Logf("Sprint Walkthrough via API")
	t.Logf("==========================")
	t.Logf("Server: %s", ts.url)
	t.Logf("")

	// ── Milestone 1: Create or discover simulation ──
	t.Logf("Milestone 1: Create or discover simulation")

	var hal struct {
		Simulation json.RawMessage   `json:"simulation"`
		Links      map[string]string `json:"_links"`
	}
	unmarshalHAL := func(data string) {
		hal.Links = nil
		json.Unmarshal([]byte(data), &hal)
	}

	resp, sim := discoverOrCreateSim(t, ts)
	unmarshalHAL(resp)

	if sim.BacklogCount == 0 {
		t.Fatal("expected backlog > 0")
	}
	if sim.SprintActive {
		t.Fatal("expected sprint inactive")
	}
	if _, ok := hal.Links["start-sprint"]; !ok {
		t.Fatal("expected start-sprint link")
	}
	t.Logf("  backlogCount=%d, sprintActive=%v, links=%v", sim.BacklogCount, sim.SprintActive, hal.Links)
	maybeDelay(ts, t, "Milestone 1: Create simulation")

	// ── Milestone 2: Initial office ──
	t.Logf("Milestone 2: Initial office state")
	// In live mode, TUI sim must be in initial state
	if sim.SprintActive {
		t.Fatal("live mode: simulation has active sprint — restart TUI fresh")
	}
	officeResp := getOffice(t, ts, sim.ID)

	for _, dev := range officeResp.Developers { // justified:AS
		if dev.State != "conference" {
			t.Fatalf("expected dev %s in conference, got %q", dev.DevID, dev.State)
		}
		if dev.TicketID != "" {
			t.Fatalf("expected no ticketId for dev %s, got %q", dev.DevID, dev.TicketID)
		}
	}
	if len(officeResp.Developers) != 6 {
		t.Fatalf("expected 6 devs, got %d", len(officeResp.Developers))
	}
	t.Logf("  All 6 devs in conference, no tickets assigned")
	maybeDelay(ts, t, "Milestone 2: Initial office")

	// ── Milestone 3: Assign tickets ──
	t.Logf("Milestone 3: Assign tickets to all 6 developers")
	for i := 1; i <= 6; i++ { // justified:SM
		body := fmt.Sprintf(`{"ticketId": "TKT-%03d"}`, i) // auto-assign
		resp = post(t, ts.srv(), "/simulations/"+sim.ID+"/assignments", body)
		unmarshalHAL(resp)
	}

	maybeDelay(ts, t, "Milestone 3: Assign tickets")

	officeResp = getOffice(t, ts, sim.ID)
	workingCount := 0
	for _, dev := range officeResp.Developers { // justified:AS
		if dev.State == "working" {
			workingCount++
			if dev.TicketID == "" {
				t.Fatalf("working dev %s has no ticketId", dev.DevID)
			}
		}
	}
	if workingCount != 6 {
		t.Fatalf("expected 6 working devs, got %d", workingCount)
	}
	t.Logf("  All 6 devs working with tickets assigned")

	// ── Milestone 4: Start sprint ──
	t.Logf("Milestone 4: Start sprint")
	resp = post(t, ts.srv(), "/simulations/"+sim.ID+"/sprints", "")
	unmarshalHAL(resp)
	sim = parseSim(t, hal.Simulation)

	if !sim.SprintActive {
		t.Fatal("expected sprintActive=true")
	}
	if _, ok := hal.Links["tick"]; !ok {
		t.Fatal("expected tick link")
	}
	if _, ok := hal.Links["start-sprint"]; ok {
		t.Fatal("expected no start-sprint link during active sprint")
	}
	t.Logf("  sprintActive=true, tick link present")
	maybeDelay(ts, t, "Milestone 4: Start sprint")

	// ── Milestone 5: Tick through sprint ──
	t.Logf("Milestone 5: Tick through sprint")
	observedStates := map[string]bool{}
	tickCount := 0

	for { // justified:WL
		resp = post(t, ts.srv(), "/simulations/"+sim.ID+"/tick", "")
		unmarshalHAL(resp)
		sim = parseSim(t, hal.Simulation)
		tickCount++

		officeResp = getOffice(t, ts, sim.ID)
		for _, dev := range officeResp.Developers { // justified:MB
			observedStates[dev.State] = true
		}

		if !sim.SprintActive {
			break
		}

		// 200ms paces ticks to ~5/sec — fast enough to feel responsive,
		// slow enough for TUI's per-event Bubble Tea render cycle to keep up.
		// Tunable: increase if TUI still desyncs on slow terminals.
		if ts.registry == nil {
			time.Sleep(200 * time.Millisecond)
		}

		if tickCount > 100 {
			t.Fatal("safety limit: sprint did not end within 100 ticks")
		}
	}

	// Assert at least one state transition was observed during sprint
	sawFrustrated := observedStates["frustrated"]
	sawIdle := observedStates["idle"]
	if !sawFrustrated && !sawIdle {
		t.Fatalf("expected at least one dev to reach frustrated or idle during sprint, observed states: %v", observedStates)
	}
	t.Logf("  Sprint completed after %d ticks", tickCount)
	t.Logf("  Observed states: %v", observedStates)
	t.Logf("  Frustrated: %v, Idle: %v", sawFrustrated, sawIdle)
	maybeDelay(ts, t, "Milestone 5: Tick through sprint")

	// ── Milestone 6: Sprint ends ──
	t.Logf("Milestone 6: Sprint ends — devs return to conference")
	maybeDelay(ts, t, "Milestone 6: Sprint ends")

	officeResp = getOffice(t, ts, sim.ID)

	for _, dev := range officeResp.Developers { // justified:AS
		if dev.State != "conference" {
			t.Fatalf("expected dev %s in conference after sprint, got %q", dev.DevID, dev.State)
		}
	}

	resp = get(t, ts.srv(), "/simulations/"+sim.ID)
	unmarshalHAL(resp)
	if _, ok := hal.Links["start-sprint"]; !ok {
		t.Fatal("expected start-sprint link after sprint ends")
	}
	t.Logf("  All 6 devs in conference, start-sprint link reappeared")

	// ── Milestone 7: Final state ──
	t.Logf("Milestone 7: Final state")
	sim = parseSim(t, hal.Simulation)

	if sim.CompletedTicketCount == 0 {
		t.Fatal("expected completedTicketCount > 0")
	}
	t.Logf("  completedTicketCount=%d", sim.CompletedTicketCount)

	// Event assertion: only in httptest mode (registry accessible)
	if ts.registry != nil {
		events := ts.registry.Store().Replay(sim.ID)
		if len(events) == 0 {
			t.Fatal("expected events to be emitted")
		}
		t.Logf("  Total events: %d", len(events))
	} else {
		t.Logf("  (live mode: skipping event store assertions)")
	}

	t.Logf("")
	t.Logf("Sprint walkthrough complete!")
}

// testServer abstracts httptest vs live server.
type testServer struct {
	url      string
	registry *SimRegistry // nil in live mode
	httpSrv  *httptest.Server
}

// srv returns an httptest.Server-compatible value for get/post helpers.
// In live mode, returns a minimal struct with just the URL.
func (ts testServer) srv() *httptest.Server {
	if ts.httpSrv != nil {
		return ts.httpSrv
	}
	// Live mode: create a fake httptest.Server with just the URL.
	// The get/post helpers only use srv.URL.
	return &httptest.Server{URL: ts.url}
}

// setupServer creates either an httptest server or connects to a live server.
func setupServer(t *testing.T) testServer {
	if liveURL := os.Getenv("SOFDEVSIM_URL"); liveURL != "" {
		t.Logf("Live mode: targeting %s", liveURL)
		return testServer{url: liveURL}
	}

	reg := NewSimRegistry()
	srv := httptest.NewServer(NewRouter(reg))
	t.Cleanup(srv.Close)

	return testServer{
		url:      srv.URL,
		registry: &reg,
		httpSrv:  srv,
	}
}

// maybeDelay pauses in live mode for TUI observation.
func maybeDelay(ts testServer, t *testing.T, label string) {
	if ts.registry == nil {
		t.Logf("  [live mode] Pausing 2s: %s", label)
		time.Sleep(2 * time.Second)
	}
}

// simState holds the fields we need from the HAL simulation response.
type simState struct {
	ID                   string `json:"id"`
	BacklogCount         int    `json:"backlogCount"`
	ActiveTicketCount    int    `json:"activeTicketCount"`
	CompletedTicketCount int    `json:"completedTicketCount"`
	SprintActive         bool   `json:"sprintActive"`
}

// parseSim extracts simState from halResponse.Simulation raw JSON.
func parseSim(t *testing.T, raw json.RawMessage) simState {
	t.Helper()
	var s simState
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("failed to parse simulation: %v", err)
	}
	return s
}

// officeState is a minimal view of OfficeResponse for assertions.
type officeState struct {
	Developers []struct {
		DevID    string `json:"devId"`
		DevName  string `json:"devName"`
		State    string `json:"state"`
		TicketID string `json:"ticketId"`
	} `json:"developers"`
}

// getOffice fetches and parses the office endpoint.
func getOffice(t *testing.T, ts testServer, simID string) officeState {
	t.Helper()
	resp := get(t, ts.srv(), "/simulations/"+simID+"/office")
	var office officeState
	if err := json.Unmarshal([]byte(resp), &office); err != nil {
		t.Fatalf("failed to parse office: %v", err)
	}
	return office
}

// discoverOrCreateSim returns the HAL response body and parsed sim state.
// httptest mode: creates sim-42. Live mode: discovers TUI's existing sim.
func discoverOrCreateSim(t *testing.T, ts testServer) (string, simState) {
	t.Helper()

	if ts.registry != nil {
		// httptest mode: create sim-42 as before
		resp := post(t, ts.srv(), "/simulations", `{"policy": "dora-strict", "seed": 42}`)
		var hal struct {
			Simulation json.RawMessage `json:"simulation"`
		}
		json.Unmarshal([]byte(resp), &hal)
		return resp, parseSim(t, hal.Simulation)
	}

	// Live mode: discover existing TUI simulation via GET /simulations
	listBody := get(t, ts.srv(), "/simulations")
	var listResp struct {
		Simulations []struct {
			ID string `json:"id"`
		} `json:"simulations"`
	}
	if err := json.Unmarshal([]byte(listBody), &listResp); err != nil {
		t.Fatalf("failed to parse simulations list: %v", err)
	}
	if len(listResp.Simulations) == 0 {
		t.Fatal("live mode: no simulations found — start TUI first")
	}

	simID := listResp.Simulations[0].ID
	t.Logf("  Live mode: discovered simulation %s", simID)

	// Get full HAL response for this sim (same format as POST /simulations)
	resp := get(t, ts.srv(), "/simulations/"+simID)
	var hal struct {
		Simulation json.RawMessage `json:"simulation"`
	}
	json.Unmarshal([]byte(resp), &hal)
	return resp, parseSim(t, hal.Simulation)
}

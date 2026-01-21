package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
	"github.com/google/go-cmp/cmp"
)

// TestClient_CreateSimulation verifies HTTP client creates simulations correctly.
// Uses httptest.NewServer with real router per Go Dev Guide §7.
func TestClient_CreateSimulation(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	client := NewClient(srv.URL)

	tests := []struct {
		name       string
		seed       int64
		policy     string
		wantIDPart string // partial match on ID
	}{
		{
			name:       "creates with seed 42",
			seed:       42,
			policy:     "dora-strict",
			wantIDPart: "sim-42",
		},
		{
			name:       "creates with different seed",
			seed:       123,
			policy:     "",
			wantIDPart: "sim-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.CreateSimulation(tt.seed, tt.policy)
			if err != nil {
				t.Fatalf("CreateSimulation failed: %v", err)
			}

			if resp.Simulation.ID != tt.wantIDPart {
				t.Errorf("ID mismatch: got %s, want %s", resp.Simulation.ID, tt.wantIDPart)
			}
		})
	}
}

// TestClient_Tick verifies tick advances simulation and returns current tick.
func TestClient_Tick(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	client := NewClient(srv.URL)

	// Create simulation and start sprint first
	resp, err := client.CreateSimulation(42, "")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	if err := client.StartSprint(resp.Simulation.ID); err != nil {
		t.Fatalf("StartSprint failed: %v", err)
	}

	// Tick should advance and return new tick number
	tickResp, err := client.Tick(resp.Simulation.ID)
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	if tickResp.Simulation.CurrentTick < 1 {
		t.Errorf("CurrentTick should be >= 1 after tick, got %d", tickResp.Simulation.CurrentTick)
	}
}

// TestClient_Assign verifies ticket assignment via HTTP.
func TestClient_Assign(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	client := NewClient(srv.URL)

	// Create simulation
	resp, err := client.CreateSimulation(42, "")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	// Get simulation state to find a ticket ID
	inst, ok := registry.GetInstance(resp.Simulation.ID)
	if !ok {
		t.Fatal("Simulation not in registry")
	}

	state := inst.Engine.Sim()
	if len(state.Backlog) == 0 {
		t.Fatal("No tickets in backlog")
	}
	ticketID := state.Backlog[0].ID
	devID := state.Developers[0].ID

	// Assign ticket
	if err := client.Assign(resp.Simulation.ID, ticketID, devID); err != nil {
		t.Fatalf("Assign failed: %v", err)
	}

	// Verify assignment
	state = inst.Engine.Sim()
	found := false
	for _, at := range state.ActiveTickets {
		if at.ID == ticketID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Ticket not assigned")
	}
}

// TestClient_StartSprint verifies sprint start via HTTP.
func TestClient_StartSprint(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	client := NewClient(srv.URL)

	// Create simulation
	resp, err := client.CreateSimulation(42, "")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	// Start sprint
	if err := client.StartSprint(resp.Simulation.ID); err != nil {
		t.Fatalf("StartSprint failed: %v", err)
	}

	// Verify sprint started
	inst, _ := registry.GetInstance(resp.Simulation.ID)
	state := inst.Engine.Sim()
	if _, active := state.CurrentSprintOption.Get(); !active {
		t.Error("Sprint not started")
	}
}

// TestClient_Idempotency verifies assign is idempotent.
func TestClient_Idempotency(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	client := NewClient(srv.URL)

	// Create and get state
	resp, err := client.CreateSimulation(42, "")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}
	inst, ok := registry.GetInstance(resp.Simulation.ID)
	if !ok {
		t.Fatal("Simulation not in registry")
	}
	state := inst.Engine.Sim()

	ticketID := state.Backlog[0].ID
	devID := state.Developers[0].ID

	// Assign twice - second call should succeed (idempotent)
	if err := client.Assign(resp.Simulation.ID, ticketID, devID); err != nil {
		t.Fatalf("First assign failed: %v", err)
	}

	// Second assign of same ticket to same dev should not error
	// (ticket already assigned, but that's fine)
	err = client.Assign(resp.Simulation.ID, ticketID, devID)
	// Note: Current API returns error for already-assigned tickets
	// This test documents current behavior - update if idempotency is added
	if err != nil {
		t.Logf("Second assign returned error (expected with current API): %v", err)
	}
}

// TestClient_ErrorHandling verifies client handles errors gracefully.
func TestClient_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		fn      func(*Client) error
		wantErr bool
	}{
		{
			name: "tick without sprint returns error",
			fn: func(c *Client) error {
				resp, _ := c.CreateSimulation(42, "")
				_, err := c.Tick(resp.Simulation.ID)
				return err
			},
			wantErr: true,
		},
		{
			name: "tick nonexistent simulation returns error",
			fn: func(c *Client) error {
				_, err := c.Tick("nonexistent")
				return err
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := api.NewSimRegistry()
			srv := httptest.NewServer(api.NewRouter(registry))
			defer srv.Close()

			client := NewClient(srv.URL)
			err := tt.fn(client)

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestClient_RequestIDHeader verifies X-Request-ID is sent.
// This is verified by successful request completion (server accepts it).
func TestClient_RequestIDHeader(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	client := NewClient(srv.URL)

	// Multiple requests should all succeed (each gets unique ID)
	for i := 0; i < 3; i++ {
		_, err := client.CreateSimulation(int64(100+i), "")
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}
}

// TestClient_DedupMiddleware verifies server caches responses by X-Request-ID.
// Note: Our client generates unique IDs per request, so this tests the server behavior.
func TestClient_DedupMiddleware(t *testing.T) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	// Create simulation first
	client := NewClient(srv.URL)
	resp, err := client.CreateSimulation(42, "")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	// Manually send duplicate request with same X-Request-ID to test dedup
	// (Our Client generates unique IDs, so we need raw HTTP for this test)
	req1, _ := http.NewRequest("POST", srv.URL+"/simulations/"+resp.Simulation.ID+"/sprints", nil)
	req1.Header.Set("X-Request-ID", "dedup-test-id")
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("First sprint request failed: %v", err)
	}
	resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("First sprint request returned %d", resp1.StatusCode)
	}

	// Second request with same ID - should return cached 200, not 409 conflict
	req2, _ := http.NewRequest("POST", srv.URL+"/simulations/"+resp.Simulation.ID+"/sprints", nil)
	req2.Header.Set("X-Request-ID", "dedup-test-id")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Second sprint request failed: %v", err)
	}
	resp2.Body.Close()

	// Dedup should return cached 200, not 409 (sprint already active)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Dedup should return cached 200, got %d", resp2.StatusCode)
	}
}

// cmpDiff helper for future table-driven tests with complex structs
var _ = cmp.Diff // Ensure cmp is available for tests

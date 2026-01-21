package tui

import (
	"net/http/httptest"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
)

// BenchmarkClient_CreateSimulation measures round-trip latency for simulation creation.
// Target: < 1ms for local operations per Go Dev Guide §8.
func BenchmarkClient_CreateSimulation(b *testing.B) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	client := NewClient(srv.URL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.CreateSimulation(int64(i), "")
		if err != nil {
			b.Fatalf("CreateSimulation failed: %v", err)
		}
	}
}

// BenchmarkClient_Tick measures hot-path performance for tick operations.
// Target: < 1ms for local operations per Go Dev Guide §8.
func BenchmarkClient_Tick(b *testing.B) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	client := NewClient(srv.URL)

	// Create simulation and start sprint
	resp, err := client.CreateSimulation(42, "")
	if err != nil {
		b.Fatalf("CreateSimulation failed: %v", err)
	}
	if err := client.StartSprint(resp.Simulation.ID); err != nil {
		b.Fatalf("StartSprint failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Tick(resp.Simulation.ID)
		if err != nil {
			// Sprint may end, restart it
			client.StartSprint(resp.Simulation.ID)
		}
	}
}

// BenchmarkClient_Assign measures assignment operation latency.
func BenchmarkClient_Assign(b *testing.B) {
	registry := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(registry))
	defer srv.Close()

	client := NewClient(srv.URL)

	// Create simulation
	resp, err := client.CreateSimulation(42, "")
	if err != nil {
		b.Fatalf("CreateSimulation failed: %v", err)
	}

	// Get ticket and dev IDs
	inst, _ := registry.GetInstance(resp.Simulation.ID)
	state := inst.Engine.Sim()
	devID := state.Developers[0].ID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Re-read state to get new backlog tickets
		state = inst.Engine.Sim()
		if len(state.Backlog) > 0 {
			ticketID := state.Backlog[0].ID
			client.Assign(resp.Simulation.ID, ticketID, devID)
		}
	}
}

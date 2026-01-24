package api

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestSharedAccess_TUISimulationAccessibleViaAPI(t *testing.T) {
	registry := NewSimRegistry()

	// Simulate TUI creating a simulation via RegisterSimulation
	sim := model.NewSimulation("sim-42", model.PolicyDORAStrict, 42)
	sim.Developers = append(sim.Developers, model.NewDeveloper("dev-1", "Alice", 1.0))
	tracker := metrics.NewTracker()

	eng, _ := registry.RegisterSimulation(sim, tracker)

	// Verify simulation is accessible via registry (API's access method)
	inst, ok := registry.GetInstance("sim-42")
	if !ok {
		t.Fatal("Simulation not found in registry after TUI registration")
	}

	// Verify it's the same simulation (by ID since Simulation is now a value type)
	if inst.Sim.ID != sim.ID {
		t.Error("Registry returned different simulation instance")
	}

	// Verify events are emitted to shared store
	// EmitLoadedState emits SimulationCreated + DeveloperAdded for each developer
	evts := registry.Store().Replay("sim-42")
	if len(evts) != 2 {
		t.Fatalf("Expected 2 events (SimulationCreated + DeveloperAdded), got %d", len(evts))
	}
	if evts[0].EventType() != "SimulationCreated" {
		t.Errorf("Expected SimulationCreated first, got %s", evts[0].EventType())
	}

	// TUI starts sprint via engine - must update inst.Engine since engine is immutable
	eng, _ = eng.StartSprint()
	inst.Engine = eng
	registry.SetInstance("sim-42", inst)

	// API should see the sprint started via engine projection (not sim directly)
	inst, _ = registry.GetInstance("sim-42")
	if _, active := inst.Engine.Sim().CurrentSprintOption.Get(); !active {
		t.Error("API does not see sprint started by TUI")
	}

	// Verify SprintStarted event in shared store
	evts = registry.Store().Replay("sim-42")
	found := false
	for _, e := range evts {
		if e.EventType() == "SprintStarted" {
			found = true
			break
		}
	}
	if !found {
		t.Error("SprintStarted event not found in shared store")
	}
}

func TestSharedAccess_APIChangesVisibleToTUI(t *testing.T) {
	registry := NewSimRegistry()

	// TUI registers simulation
	sim := model.NewSimulation("sim-42", model.PolicyDORAStrict, 42)
	sim.Developers = append(sim.Developers, model.NewDeveloper("dev-1", "Alice", 1.0))
	sim.Backlog = append(sim.Backlog, model.NewTicket("TKT-001", "Test", 3, model.HighUnderstanding))
	tracker := metrics.NewTracker()

	_, _ = registry.RegisterSimulation(sim, tracker)

	// API gets the simulation instance and modifies it
	inst, _ := registry.GetInstance("sim-42")
	inst.Engine, _ = inst.Engine.StartSprint()
	inst.Engine, _ = inst.Engine.AssignTicket("TKT-001", "dev-1")

	// TUI should see the changes via engine projection (not sim pointer)
	state := inst.Engine.Sim()
	if len(state.ActiveTickets) != 1 {
		t.Errorf("TUI doesn't see ticket assigned by API: active=%d", len(state.ActiveTickets))
	}

	// Both should see events in shared store
	evts := registry.Store().Replay("sim-42")
	eventTypes := make([]string, len(evts))
	for i, e := range evts {
		eventTypes[i] = e.EventType()
	}

	// Should have: SimulationCreated, SprintStarted, TicketAssigned
	if len(evts) < 3 {
		t.Errorf("Expected at least 3 events, got %d: %v", len(evts), eventTypes)
	}
}

func TestSharedAccess_BothCanSubscribe(t *testing.T) {
	registry := NewSimRegistry()

	// TUI registers simulation
	sim := model.NewSimulation("sim-42", model.PolicyDORAStrict, 42)
	sim.Developers = append(sim.Developers, model.NewDeveloper("dev-1", "Alice", 1.0))
	tracker := metrics.NewTracker()

	eng, _ := registry.RegisterSimulation(sim, tracker)

	// Both TUI and API can subscribe to the shared store
	tuiCh := registry.Store().Subscribe("sim-42")
	apiCh := registry.Store().Subscribe("sim-42")

	// Engine emits event
	_, _ = eng.StartSprint()

	// Both should receive the event
	select {
	case e := <-tuiCh:
		if e.EventType() != "SprintStarted" {
			t.Errorf("TUI received wrong event: %s", e.EventType())
		}
	default:
		t.Error("TUI did not receive SprintStarted event")
	}

	select {
	case e := <-apiCh:
		if e.EventType() != "SprintStarted" {
			t.Errorf("API received wrong event: %s", e.EventType())
		}
	default:
		t.Error("API did not receive SprintStarted event")
	}

	// Cleanup
	registry.Store().Unsubscribe("sim-42", tuiCh)
	registry.Store().Unsubscribe("sim-42", apiCh)
}

func TestSharedAccess_SimulationCreatedHasCorrectTeamSize(t *testing.T) {
	registry := NewSimRegistry()

	// TUI creates simulation with team BEFORE registering
	sim := model.NewSimulation("sim-42", model.PolicyDORAStrict, 42)
	sim.Developers = append(sim.Developers, model.NewDeveloper("dev-1", "Alice", 1.0))
	sim.Developers = append(sim.Developers, model.NewDeveloper("dev-2", "Bob", 0.8))
	sim.Developers = append(sim.Developers, model.NewDeveloper("dev-3", "Carol", 1.2))
	tracker := metrics.NewTracker()

	_, _ = registry.RegisterSimulation(sim, tracker)

	// Verify SimulationCreated has correct team size
	evts := registry.Store().Replay("sim-42")
	if len(evts) == 0 {
		t.Fatal("No events found")
	}

	created := evts[0].(events.SimulationCreated)
	if created.Config.TeamSize != 3 {
		t.Errorf("SimulationCreated.Config.TeamSize = %d, want 3", created.Config.TeamSize)
	}
}

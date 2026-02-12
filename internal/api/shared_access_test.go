package api

import (
	"testing"

	"github.com/binaryphile/fluentfp/must"
	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestSharedAccess_TUISimulationAccessibleViaAPI(t *testing.T) {
	reg := NewSimRegistry()

	// Simulate TUI creating and populating simulation, then calling SetInstance
	sim := model.NewSimulation("sim-42", model.PolicyDORAStrict, 42)
	tracker := metrics.NewTracker()

	eng := engine.NewEngineWithStore(sim.Seed, reg.Store())
	eng = must.Get(eng.EmitCreated(sim.ID, 0, events.SimConfig{
		TeamSize:     1,
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
	}))
	eng = must.Get(eng.AddDeveloper("dev-1", "Alice", 1.0))

	// Store fully-populated engine (design invariant)
	reg.SetInstance("sim-42", SimInstance{
		Sim:     sim,
		Engine:  eng,
		Tracker: tracker,
	})

	// Verify simulation is accessible via registry (API's access method)
	inst, ok := reg.GetInstanceOption("sim-42").Get()
	if !ok {
		t.Fatal("Simulation not found in registry after TUI registration")
	}

	// Verify it's the same simulation (by ID since Simulation is now a value type)
	if inst.Sim.ID != sim.ID {
		t.Error("Registry returned different simulation instance")
	}

	// Verify events are emitted to shared store
	evts := reg.Store().Replay("sim-42")
	if len(evts) != 2 {
		t.Fatalf("Expected 2 events (SimulationCreated + DeveloperAdded), got %d", len(evts))
	}
	if evts[0].EventType() != "SimulationCreated" {
		t.Errorf("Expected SimulationCreated first, got %s", evts[0].EventType())
	}

	// TUI starts sprint via engine - must update inst.Engine since engine is immutable
	eng, _ = eng.StartSprint()
	inst.Engine = eng
	reg.SetInstance("sim-42", inst)

	// API should see the sprint started via engine projection (not sim directly)
	inst = reg.GetInstanceOption("sim-42").OrZero()
	if _, active := inst.Engine.Sim().CurrentSprintOption.Get(); !active {
		t.Error("API does not see sprint started by TUI")
	}

	// Verify SprintStarted event in shared store
	evts = reg.Store().Replay("sim-42")
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
	reg := NewSimRegistry()

	// TUI creates and populates simulation
	sim := model.NewSimulation("sim-42", model.PolicyDORAStrict, 42)
	tracker := metrics.NewTracker()

	eng := engine.NewEngineWithStore(sim.Seed, reg.Store())
	eng = must.Get(eng.EmitCreated(sim.ID, 0, events.SimConfig{
		TeamSize:     1,
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
	}))
	eng = must.Get(eng.AddDeveloper("dev-1", "Alice", 1.0))
	eng = must.Get(eng.AddTicket(model.NewTicket("TKT-001", "Test", 3, model.HighUnderstanding)))

	reg.SetInstance("sim-42", SimInstance{
		Sim:     sim,
		Engine:  eng,
		Tracker: tracker,
	})

	// API gets the simulation instance and modifies it
	inst := reg.GetInstanceOption("sim-42").OrZero()
	inst.Engine, _ = inst.Engine.StartSprint()
	inst.Engine, _ = inst.Engine.AssignTicket("TKT-001", "dev-1")

	// TUI should see the changes via engine projection (not sim pointer)
	state := inst.Engine.Sim()
	if len(state.ActiveTickets) != 1 {
		t.Errorf("TUI doesn't see ticket assigned by API: active=%d", len(state.ActiveTickets))
	}

	// Both should see events in shared store
	evts := reg.Store().Replay("sim-42")
	eventTypes := make([]string, len(evts))
	for i, e := range evts {
		eventTypes[i] = e.EventType()
	}

	// Should have: SimulationCreated, DeveloperAdded, TicketCreated, SprintStarted, TicketAssigned
	if len(evts) < 5 {
		t.Errorf("Expected at least 5 events, got %d: %v", len(evts), eventTypes)
	}
}

func TestSharedAccess_BothCanSubscribe(t *testing.T) {
	reg := NewSimRegistry()

	// TUI creates and populates simulation
	sim := model.NewSimulation("sim-42", model.PolicyDORAStrict, 42)
	tracker := metrics.NewTracker()

	eng := engine.NewEngineWithStore(sim.Seed, reg.Store())
	eng = must.Get(eng.EmitCreated(sim.ID, 0, events.SimConfig{
		TeamSize:     1,
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
	}))
	eng = must.Get(eng.AddDeveloper("dev-1", "Alice", 1.0))

	reg.SetInstance("sim-42", SimInstance{
		Sim:     sim,
		Engine:  eng,
		Tracker: tracker,
	})

	// Both TUI and API can subscribe to the shared store
	tuiCh := reg.Store().Subscribe("sim-42")
	apiCh := reg.Store().Subscribe("sim-42")

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
	reg.Store().Unsubscribe("sim-42", tuiCh)
	reg.Store().Unsubscribe("sim-42", apiCh)
}

func TestSharedAccess_SimulationCreatedHasCorrectTeamSize(t *testing.T) {
	reg := NewSimRegistry()

	// TUI creates simulation with explicit team size in EmitCreated
	sim := model.NewSimulation("sim-42", model.PolicyDORAStrict, 42)
	tracker := metrics.NewTracker()

	eng := engine.NewEngineWithStore(sim.Seed, reg.Store())
	eng = must.Get(eng.EmitCreated(sim.ID, 0, events.SimConfig{
		TeamSize:     3, // Explicit team size
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
	}))
	eng = must.Get(eng.AddDeveloper("dev-1", "Alice", 1.0))
	eng = must.Get(eng.AddDeveloper("dev-2", "Bob", 0.8))
	eng = must.Get(eng.AddDeveloper("dev-3", "Carol", 1.2))

	reg.SetInstance("sim-42", SimInstance{
		Sim:     sim,
		Engine:  eng,
		Tracker: tracker,
	})

	// Verify SimulationCreated has correct team size
	evts := reg.Store().Replay("sim-42")
	if len(evts) == 0 {
		t.Fatal("No events found")
	}

	created := evts[0].(events.SimulationCreated)
	if created.Config.TeamSize != 3 {
		t.Errorf("SimulationCreated.Config.TeamSize = %d, want 3", created.Config.TeamSize)
	}
}

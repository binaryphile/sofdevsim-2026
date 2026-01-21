package tui

import (
	"testing"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/registry"
)

// TestNewAppWithSeed_Reproducibility verifies same seed produces identical initial state.
func TestNewAppWithSeed_Reproducibility(t *testing.T) {
	app1 := NewAppWithSeed(42)
	app2 := NewAppWithSeed(42)

	sim1 := app1.engine.Sim()
	sim2 := app2.engine.Sim()

	if sim1.Backlog[0].ID != sim2.Backlog[0].ID {
		t.Errorf("Seed 42 should produce identical backlogs, got %s and %s",
			sim1.Backlog[0].ID, sim2.Backlog[0].ID)
	}

	if sim1.Seed != sim2.Seed {
		t.Errorf("Seed should be stored identically, got %d and %d",
			sim1.Seed, sim2.Seed)
	}
}

// TestNewAppWithSeed_ZeroUsesRandomSeed verifies seed 0 produces different results.
func TestNewAppWithSeed_ZeroUsesRandomSeed(t *testing.T) {
	app1 := NewAppWithSeed(0)
	time.Sleep(time.Nanosecond) // Ensure different time
	app2 := NewAppWithSeed(0)

	sim1 := app1.engine.Sim()
	sim2 := app2.engine.Sim()

	// Different seeds should (almost always) produce different backlogs
	// This is probabilistic but failure is astronomically unlikely
	if sim1.Seed == sim2.Seed {
		t.Errorf("Seed 0 should use current time, producing different seeds")
	}
}

// TestSprintEndsWhenDurationReached verifies sprint is cleared after end day.
func TestSprintEndsWhenDurationReached(t *testing.T) {
	app := NewAppWithSeed(42)
	app.engine.StartSprint() // Use engine to emit events
	app.paused = false              // Enable tick processing
	app.currentView = ViewExecution // Required for tick processing

	// Get sprint end day from engine projection
	sim := app.engine.Sim()
	sprint, ok := sim.CurrentSprintOption.Get()
	if !ok {
		t.Fatal("Expected sprint to be started")
	}

	// Run ticks until sprint ends (engine handles everything via events)
	for i := 0; i < sprint.DurationDays+1; i++ {
		app.Update(tickMsg(time.Now()))
	}

	// Sprint should be cleared in projection
	sim = app.engine.Sim()
	if _, ok := sim.CurrentSprintOption.Get(); ok {
		t.Error("Sprint should be cleared after end day reached")
	}
	if !app.paused {
		t.Error("Should be paused after sprint ends")
	}
}

// TestNewAppWithRegistry_SubscribesToEvents verifies TUI subscribes to event store.
func TestNewAppWithRegistry_SubscribesToEvents(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)

	// TUI should have a subscription channel
	if app.eventSub == nil {
		t.Fatal("Expected eventSub channel to be set")
	}

	// TUI should be registered in the shared registry
	// (accessible via API)
	sim := app.engine.Sim()
	evts := reg.Store().Replay(sim.ID)
	if len(evts) == 0 {
		t.Error("Expected SimulationCreated event in shared store")
	}
}

// TestNewAppWithSeed_ProjectionHasInitialState verifies projection has devs and tickets.
func TestNewAppWithSeed_ProjectionHasInitialState(t *testing.T) {
	app := NewAppWithSeed(42)

	// Projection should have the developers
	sim := app.engine.Sim()
	if len(sim.Developers) != 3 {
		t.Errorf("Projection should have 3 developers, got %d", len(sim.Developers))
	}

	// Projection should have the backlog
	if len(sim.Backlog) != 12 {
		t.Errorf("Projection should have 12 tickets in backlog, got %d", len(sim.Backlog))
	}

	// First developer should be Alice
	if sim.Developers[0].Name != "Alice" {
		t.Errorf("First developer should be Alice, got %s", sim.Developers[0].Name)
	}
}

// TestTUI_ReceivesExternalEvents verifies TUI receives events from API actions.
func TestTUI_ReceivesExternalEvents(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)

	// Simulate API starting a sprint (external to TUI)
	// This goes through the same engine, which emits to shared store
	app.engine.StartSprint()

	// TUI should receive the event via subscription
	select {
	case evt := <-app.eventSub:
		if evt.EventType() != "SprintStarted" {
			t.Errorf("Expected SprintStarted event, got %s", evt.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timed out waiting for SprintStarted event")
	}
}

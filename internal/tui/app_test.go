package tui

import (
	"testing"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
)

// TestNewAppWithSeed_Reproducibility verifies same seed produces identical initial state.
func TestNewAppWithSeed_Reproducibility(t *testing.T) {
	app1 := NewAppWithSeed(42)
	app2 := NewAppWithSeed(42)

	if app1.sim.Backlog[0].ID != app2.sim.Backlog[0].ID {
		t.Errorf("Seed 42 should produce identical backlogs, got %s and %s",
			app1.sim.Backlog[0].ID, app2.sim.Backlog[0].ID)
	}

	if app1.sim.Seed != app2.sim.Seed {
		t.Errorf("Seed should be stored identically, got %d and %d",
			app1.sim.Seed, app2.sim.Seed)
	}
}

// TestNewAppWithSeed_ZeroUsesRandomSeed verifies seed 0 produces different results.
func TestNewAppWithSeed_ZeroUsesRandomSeed(t *testing.T) {
	app1 := NewAppWithSeed(0)
	time.Sleep(time.Nanosecond) // Ensure different time
	app2 := NewAppWithSeed(0)

	// Different seeds should (almost always) produce different backlogs
	// This is probabilistic but failure is astronomically unlikely
	if app1.sim.Seed == app2.sim.Seed {
		t.Errorf("Seed 0 should use current time, producing different seeds")
	}
}

// TestSprintEndsWhenDurationReached verifies sprint is cleared after end day.
func TestSprintEndsWhenDurationReached(t *testing.T) {
	app := NewAppWithSeed(42)
	app.engine.StartSprint() // Use engine to emit events
	app.paused = false                // Enable tick processing
	app.currentView = ViewExecution   // Required for tick processing

	// Get sprint end day
	sprint, ok := app.sim.CurrentSprintOption.Get()
	if !ok {
		t.Fatal("Expected sprint to be started")
	}

	// Move to end day
	app.sim.CurrentTick = sprint.EndDay

	// Simulate tick that should end sprint
	app.Update(tickMsg(time.Now()))

	// Sprint should be cleared
	if _, ok := app.sim.CurrentSprintOption.Get(); ok {
		t.Error("Sprint should be cleared after end day reached")
	}
	if !app.paused {
		t.Error("Should be paused after sprint ends")
	}
}

// TestNewAppWithRegistry_SubscribesToEvents verifies TUI subscribes to event store.
func TestNewAppWithRegistry_SubscribesToEvents(t *testing.T) {
	registry := api.NewSimRegistry()
	app := NewAppWithRegistry(42, registry)

	// TUI should have a subscription channel
	if app.eventSub == nil {
		t.Fatal("Expected eventSub channel to be set")
	}

	// TUI should be registered in the shared registry
	// (accessible via API)
	evts := registry.Store().Replay(app.sim.ID)
	if len(evts) == 0 {
		t.Error("Expected SimulationCreated event in shared store")
	}
}

// TestTUI_ReceivesExternalEvents verifies TUI receives events from API actions.
func TestTUI_ReceivesExternalEvents(t *testing.T) {
	registry := api.NewSimRegistry()
	app := NewAppWithRegistry(42, registry)

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

package tui

import (
	"testing"
	"time"
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
	app.sim.StartSprint()
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

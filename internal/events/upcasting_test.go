package events_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
)

func TestUpcaster_Apply_NoTransform(t *testing.T) {
	// Apply event to DefaultUpcaster with no transforms registered
	// Should return event unchanged (same EventID)
	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})

	result := events.DefaultUpcaster.Apply(evt)

	if result.EventID() != evt.EventID() {
		t.Errorf("EventID changed: got %s, want %s", result.EventID(), evt.EventID())
	}
	if result.EventType() != evt.EventType() {
		t.Errorf("EventType changed: got %s, want %s", result.EventType(), evt.EventType())
	}
}

func TestUpcaster_Apply_WithTransform(t *testing.T) {
	// Create custom Upcaster with transform that modifies event
	// Apply event, assert transformation occurred
	originalEvt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})

	// Transform that changes the seed to 99 and bumps version to 2
	// transformSimCreated modifies SimulationCreated seed to 99.
	transformSimCreated := func(evt events.Event) events.Event {
		sc := evt.(events.SimulationCreated)
		sc.Config.Seed = 99
		sc.Header.Version = 2 // bump version to exit upcasting loop
		return sc
	}

	upcaster := events.NewUpcasterWithTransforms(map[string]func(events.Event) events.Event{
		"SimulationCreated:v1": transformSimCreated,
	})

	result := upcaster.Apply(originalEvt)

	// Verify transform was applied
	resultSC, ok := result.(events.SimulationCreated)
	if !ok {
		t.Fatalf("Expected SimulationCreated, got %T", result)
	}
	if resultSC.Config.Seed != 99 {
		t.Errorf("Transform not applied: seed = %d, want 99", resultSC.Config.Seed)
	}
}

func TestUpcaster_Apply_VersionedKey(t *testing.T) {
	// TDD: Upcaster should dispatch by "Type:vN" format
	// Register transform for SimulationCreated:v1 that bumps to v2 with seed 99
	transformV1ToV2 := func(evt events.Event) events.Event {
		sc := evt.(events.SimulationCreated)
		sc.Config.Seed = 99
		sc.Header.Version = 2 // bump version to exit upcasting loop
		return sc
	}

	upcaster := events.NewUpcasterWithTransforms(map[string]func(events.Event) events.Event{
		"SimulationCreated:v1": transformV1ToV2,
	})

	// Create v1 event (Version: 1)
	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})

	result := upcaster.Apply(evt)

	resultSC, ok := result.(events.SimulationCreated)
	if !ok {
		t.Fatalf("Expected SimulationCreated, got %T", result)
	}
	if resultSC.Config.Seed != 99 {
		t.Errorf("Transform not applied: seed = %d, want 99", resultSC.Config.Seed)
	}
}

func TestUpcaster_Apply_CycleDetection(t *testing.T) {
	// TDD: Upcaster should panic on cycle (v1→v2→v1)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on cycle, got none")
		}
	}()

	// Create transforms that form a cycle
	v1ToV2 := func(evt events.Event) events.Event {
		sc := evt.(events.SimulationCreated)
		sc.Header.Version = 2 // bump to v2
		return sc
	}
	v2ToV1 := func(evt events.Event) events.Event {
		sc := evt.(events.SimulationCreated)
		sc.Header.Version = 1 // back to v1 (cycle!)
		return sc
	}

	upcaster := events.NewUpcasterWithTransforms(map[string]func(events.Event) events.Event{
		"SimulationCreated:v1": v1ToV2,
		"SimulationCreated:v2": v2ToV1,
	})

	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})

	// This should panic due to cycle
	_ = upcaster.Apply(evt)
}

// BenchmarkUpcaster_Apply_NoTransform measures baseline: Apply with no matching transform.
// Target: < 100ns/op (map lookup only).
func BenchmarkUpcaster_Apply_NoTransform(b *testing.B) {
	upcaster := events.DefaultUpcaster
	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = upcaster.Apply(evt)
	}
}

// BenchmarkUpcaster_Apply_WithTransform measures overhead: Apply with transform lookup + invocation.
func BenchmarkUpcaster_Apply_WithTransform(b *testing.B) {
	// v1ToV2 increments version to exit transform loop (minimal transform work).
	v1ToV2 := func(evt events.Event) events.Event {
		sc := evt.(events.SimulationCreated)
		sc.Header.Version = 2
		return sc
	}

	upcaster := events.NewUpcasterWithTransforms(map[string]func(events.Event) events.Event{
		"SimulationCreated:v1": v1ToV2, // Versioned key per Phase 21
	})
	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = upcaster.Apply(evt)
	}
}

// BenchmarkUpcaster_Apply_TransitiveChain measures transitive upcasting: v1→v2→v3.
// Tests cycle detection overhead (seen map) and multiple transform invocations.
// Per ES Guide §11: "Transform on Read" approach with DAG validation.
func BenchmarkUpcaster_Apply_TransitiveChain(b *testing.B) {
	v1ToV2 := func(evt events.Event) events.Event {
		sc := evt.(events.SimulationCreated)
		sc.Header.Version = 2
		return sc
	}
	v2ToV3 := func(evt events.Event) events.Event {
		sc := evt.(events.SimulationCreated)
		sc.Header.Version = 3
		return sc
	}

	upcaster := events.NewUpcasterWithTransforms(map[string]func(events.Event) events.Event{
		"SimulationCreated:v1": v1ToV2,
		"SimulationCreated:v2": v2ToV3,
	})
	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = upcaster.Apply(evt)
	}
}

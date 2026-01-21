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

	// Transform that changes the seed to 99
	// transformSimCreated modifies SimulationCreated seed to 99.
	transformSimCreated := func(evt events.Event) events.Event {
		sc := evt.(events.SimulationCreated)
		sc.Config.Seed = 99
		return sc
	}

	upcaster := events.NewUpcasterWithTransforms(map[string]func(events.Event) events.Event{
		"SimulationCreated": transformSimCreated,
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
	// identityTransform returns event unchanged (minimal transform work).
	identityTransform := func(evt events.Event) events.Event { return evt }

	upcaster := events.NewUpcasterWithTransforms(map[string]func(events.Event) events.Event{
		"SimulationCreated": identityTransform,
	})
	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = upcaster.Apply(evt)
	}
}

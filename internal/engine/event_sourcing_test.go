package engine

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestEngine_EmitsSimulationCreatedEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()

	eng := NewEngineWithStore(sim, store)
	eng.EmitCreated()

	evts := store.Replay(sim.ID)
	if len(evts) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(evts))
	}

	if evts[0].EventType() != "SimulationCreated" {
		t.Errorf("Expected SimulationCreated event, got %s", evts[0].EventType())
	}

	// Verify TeamSize is correct (should be 1, not 0)
	created := evts[0].(events.SimulationCreated)
	if created.Config.TeamSize != 1 {
		t.Errorf("Expected TeamSize 1, got %d", created.Config.TeamSize)
	}
}

func TestEngine_EmitsTickedEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim, store)
	eng.EmitCreated()

	// Initial event count (SimulationCreated)
	initial := store.EventCount(sim.ID)

	eng.Tick()

	evts := store.Replay(sim.ID)
	if len(evts) <= initial {
		t.Fatal("Expected Ticked event after Tick()")
	}

	// Last event should be Ticked
	lastEvt := evts[len(evts)-1]
	if lastEvt.EventType() != "Ticked" {
		t.Errorf("Expected Ticked event, got %s", lastEvt.EventType())
	}

	tickedEvt, ok := lastEvt.(events.Ticked)
	if !ok {
		t.Fatalf("Event is not events.Ticked type")
	}
	if tickedEvt.Tick != 1 {
		t.Errorf("Expected tick 1, got %d", tickedEvt.Tick)
	}
}

func TestEngine_EmitsSprintStartedEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim, store)
	eng.EmitCreated()

	eng.StartSprint()

	// Find SprintStarted event
	evts := store.Replay(sim.ID)
	found := false
	for _, e := range evts {
		if e.EventType() == "SprintStarted" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected SprintStarted event not found")
	}
}

func TestEngine_EmitsTicketAssignedEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim, store)
	eng.EmitCreated()

	// Add a ticket to backlog
	sim.Backlog = append(sim.Backlog, model.Ticket{
		ID:            "TKT-001",
		EstimatedDays: 3,
	})

	err := eng.AssignTicket("TKT-001", "DEV-001")
	if err != nil {
		t.Fatalf("AssignTicket failed: %v", err)
	}

	// Find TicketAssigned event
	evts := store.Replay(sim.ID)
	found := false
	for _, e := range evts {
		if e.EventType() == "TicketAssigned" {
			assigned, ok := e.(events.TicketAssigned)
			if ok && assigned.TicketID == "TKT-001" && assigned.DeveloperID == "DEV-001" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("Expected TicketAssigned event not found")
	}
}

func TestEngine_EmitsTicketCompletedEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim, store)
	eng.EmitCreated()

	// Setup a ticket that will complete in one tick
	ticket := model.NewTicket("TKT-001", "Test Ticket", 1, model.HighUnderstanding)
	ticket.AssignedTo = "DEV-001"
	ticket.Phase = model.PhaseDone - 1 // Last phase before done
	ticket.RemainingEffort = 0.1       // Almost done
	sim.ActiveTickets = append(sim.ActiveTickets, ticket)
	sim.Developers[0] = sim.Developers[0].WithTicket("TKT-001")

	eng.Tick()

	// Find TicketCompleted event
	evts := store.Replay(sim.ID)
	found := false
	for _, e := range evts {
		if e.EventType() == "TicketCompleted" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected TicketCompleted event not found")
	}
}

func TestEngine_TracingAppliedToEvents(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim, store)

	// Set trace context before emitting events
	tc := events.NewTraceContext()
	eng.SetTrace(tc)

	eng.EmitCreated()
	eng.Tick()

	evts := store.Replay(sim.ID)
	if len(evts) < 2 {
		t.Fatalf("Expected at least 2 events, got %d", len(evts))
	}

	// All events should have the trace context
	for _, e := range evts {
		if e.TraceID() != tc.TraceID {
			t.Errorf("Event %s TraceID = %s, want %s", e.EventType(), e.TraceID(), tc.TraceID)
		}
		if e.SpanID() != tc.SpanID {
			t.Errorf("Event %s SpanID = %s, want %s", e.EventType(), e.SpanID(), tc.SpanID)
		}
	}
}

func TestEngine_ClearTraceRemovesContext(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim, store)

	// Set and then clear trace
	tc := events.NewTraceContext()
	eng.SetTrace(tc)
	eng.ClearTrace()

	eng.EmitCreated()

	evts := store.Replay(sim.ID)
	if len(evts) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(evts))
	}

	// Event should NOT have trace context
	if evts[0].TraceID() != "" {
		t.Errorf("TraceID should be empty after ClearTrace, got %s", evts[0].TraceID())
	}
}

func TestEngine_ChildSpanTracking(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim, store)

	// Create parent trace
	parentTC := events.NewTraceContext()
	eng.SetTrace(parentTC)
	eng.EmitCreated()

	// Create child span for tick operation
	childTC := parentTC.NewChildSpan()
	eng.SetTrace(childTC)
	eng.Tick()

	evts := store.Replay(sim.ID)
	if len(evts) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(evts))
	}

	// First event (SimulationCreated) should have parent span
	if evts[0].SpanID() != parentTC.SpanID {
		t.Errorf("SimulationCreated SpanID = %s, want %s", evts[0].SpanID(), parentTC.SpanID)
	}

	// Second event (Ticked) should have child span with parent reference
	if evts[1].SpanID() != childTC.SpanID {
		t.Errorf("Ticked SpanID = %s, want %s", evts[1].SpanID(), childTC.SpanID)
	}
	if evts[1].ParentSpanID() != parentTC.SpanID {
		t.Errorf("Ticked ParentSpanID = %s, want %s", evts[1].ParentSpanID(), parentTC.SpanID)
	}

	// Both should share the same trace ID
	if evts[0].TraceID() != evts[1].TraceID() {
		t.Errorf("Events should share TraceID: %s vs %s", evts[0].TraceID(), evts[1].TraceID())
	}
}

func TestEngine_CurrentTraceReturnsContext(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim, store)

	// Initially empty
	if !eng.CurrentTrace().IsEmpty() {
		t.Error("CurrentTrace should be empty initially")
	}

	tc := events.NewTraceContext()
	eng.SetTrace(tc)

	if eng.CurrentTrace().TraceID != tc.TraceID {
		t.Errorf("CurrentTrace().TraceID = %s, want %s", eng.CurrentTrace().TraceID, tc.TraceID)
	}
}

// createTestSimulation creates a minimal simulation for testing.
func createTestSimulation() *model.Simulation {
	sim := model.NewSimulation(model.PolicyNone, 42)
	sim.ID = "test-sim"
	sim.Developers = []model.Developer{
		{ID: "DEV-001", Name: "Test Dev", Velocity: 1.0},
	}
	return sim
}

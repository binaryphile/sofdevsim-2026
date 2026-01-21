package engine

import (
	"fmt"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestEngine_EmitsSimulationCreatedEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()

	eng := NewEngineWithStore(sim.Seed, store)
	emitCreatedFromSim(eng, sim)

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
	eng := NewEngineWithStore(sim.Seed, store)
	emitCreatedFromSim(eng, sim)

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
	eng := NewEngineWithStore(sim.Seed, store)
	emitCreatedFromSim(eng, sim)

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
	eng := NewEngineWithStore(sim.Seed, store)
	eng.EmitLoadedState(*sim) // Syncs all sim state (including developers) to projection

	// Add a ticket through engine (emits TicketCreated event)
	eng.AddTicket(model.Ticket{
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

	// Setup a ticket that will complete in one tick - BEFORE creating engine
	ticket := model.NewTicket("TKT-001", "Test Ticket", 1, model.HighUnderstanding)
	ticket.AssignedTo = "DEV-001"
	ticket.Phase = model.PhaseDone - 1 // Last phase before done
	ticket.RemainingEffort = 0.1       // Almost done
	sim.ActiveTickets = append(sim.ActiveTickets, ticket)
	sim.Developers[0] = sim.Developers[0].WithTicket("TKT-001")

	eng := NewEngineWithStore(sim.Seed, store)
	eng.EmitLoadedState(*sim) // Sync all state including ActiveTickets to projection

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
	eng := NewEngineWithStore(sim.Seed, store)

	// Set trace context before emitting events
	tc := events.NewTraceContext()
	eng.SetTrace(tc)

	emitCreatedFromSim(eng, sim)
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
	eng := NewEngineWithStore(sim.Seed, store)

	// Set and then clear trace
	tc := events.NewTraceContext()
	eng.SetTrace(tc)
	eng.ClearTrace()

	emitCreatedFromSim(eng, sim)

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
	eng := NewEngineWithStore(sim.Seed, store)

	// Create parent trace
	parentTC := events.NewTraceContext()
	eng.SetTrace(parentTC)
	emitCreatedFromSim(eng, sim)

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
	eng := NewEngineWithStore(sim.Seed, store)

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

func TestEngine_HasProjectionField(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim.Seed, store)

	// Projection should be initialized (version 0)
	if eng.proj.Version() != 0 {
		t.Errorf("New engine proj.Version() = %d, want 0", eng.proj.Version())
	}
}

func TestEngine_EmitUpdatesProjection(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim.Seed, store)

	// Before EmitCreated, projection should be empty
	if eng.proj.Version() != 0 {
		t.Errorf("Before EmitCreated, proj.Version() = %d, want 0", eng.proj.Version())
	}

	emitCreatedFromSim(eng, sim)

	// After EmitCreated, projection should have applied the event
	if eng.proj.Version() != 1 {
		t.Errorf("After EmitCreated, proj.Version() = %d, want 1", eng.proj.Version())
	}

	// Verify state was set correctly
	state := eng.proj.State()
	if state.ID != sim.ID {
		t.Errorf("proj.State().ID = %s, want %s", state.ID, sim.ID)
	}
}

func TestEngine_SimReturnsProjectionState(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim.Seed, store)
	emitCreatedFromSim(eng, sim)

	// Sim() should return state derived from projection
	state := eng.Sim()

	if state.ID != sim.ID {
		t.Errorf("Sim().ID = %s, want %s", state.ID, sim.ID)
	}
	if state.Seed != sim.Seed {
		t.Errorf("Sim().Seed = %d, want %d", state.Seed, sim.Seed)
	}
}

func TestEngine_AddDeveloperEmitsEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := model.NewSimulation(model.PolicyNone, 42)
	sim.ID = "test-sim"
	eng := NewEngineWithStore(sim.Seed, store)
	emitCreatedFromSim(eng, sim)

	eng.AddDeveloper("dev-1", "Alice", 1.0)

	// Find DeveloperAdded event
	evts := store.Replay(sim.ID)
	var found *events.DeveloperAdded
	for _, e := range evts {
		if e.EventType() == "DeveloperAdded" {
			da := e.(events.DeveloperAdded)
			found = &da
			break
		}
	}

	if found == nil {
		t.Fatal("Expected DeveloperAdded event not found")
	}

	if found.DeveloperID != "dev-1" {
		t.Errorf("DeveloperAdded.DeveloperID = %s, want dev-1", found.DeveloperID)
	}
	if found.Name != "Alice" {
		t.Errorf("DeveloperAdded.Name = %s, want Alice", found.Name)
	}

	// Verify projection has the developer
	state := eng.Sim()
	if len(state.Developers) != 1 {
		t.Fatalf("Sim().Developers length = %d, want 1", len(state.Developers))
	}
	if state.Developers[0].ID != "dev-1" {
		t.Errorf("Sim().Developers[0].ID = %s, want dev-1", state.Developers[0].ID)
	}
}

func TestEngine_AddTicketEmitsEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := model.NewSimulation(model.PolicyNone, 42)
	sim.ID = "test-sim"
	eng := NewEngineWithStore(sim.Seed, store)
	emitCreatedFromSim(eng, sim)

	ticket := model.NewTicket("TKT-001", "Test Ticket", 3.0, model.HighUnderstanding)
	eng.AddTicket(ticket)

	// Find TicketCreated event
	evts := store.Replay(sim.ID)
	var found *events.TicketCreated
	for _, e := range evts {
		if e.EventType() == "TicketCreated" {
			tc := e.(events.TicketCreated)
			found = &tc
			break
		}
	}

	if found == nil {
		t.Fatal("Expected TicketCreated event not found")
	}

	if found.TicketID != "TKT-001" {
		t.Errorf("TicketCreated.TicketID = %s, want TKT-001", found.TicketID)
	}

	// Verify projection has the ticket in backlog
	state := eng.Sim()
	if len(state.Backlog) != 1 {
		t.Fatalf("Sim().Backlog length = %d, want 1", len(state.Backlog))
	}
	if state.Backlog[0].ID != "TKT-001" {
		t.Errorf("Sim().Backlog[0].ID = %s, want TKT-001", state.Backlog[0].ID)
	}
}

func TestEngine_EmitsWorkProgressedEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()

	// Setup a ticket that has work remaining (won't complete in one tick) - BEFORE creating engine
	ticket := model.NewTicket("TKT-001", "Test Ticket", 5, model.HighUnderstanding)
	ticket.AssignedTo = "DEV-001"
	ticket.Phase = model.PhaseResearch
	ticket.RemainingEffort = 5.0
	sim.ActiveTickets = append(sim.ActiveTickets, ticket)
	sim.Developers[0] = sim.Developers[0].WithTicket("TKT-001")

	eng := NewEngineWithStore(sim.Seed, store)
	eng.EmitLoadedState(*sim) // Sync all state including ActiveTickets to projection

	eng.Tick()

	// Find WorkProgressed event
	evts := store.Replay(sim.ID)
	var found *events.WorkProgressed
	for _, e := range evts {
		if e.EventType() == "WorkProgressed" {
			wp := e.(events.WorkProgressed)
			found = &wp
			break
		}
	}

	if found == nil {
		t.Fatal("Expected WorkProgressed event not found")
	}

	if found.TicketID != "TKT-001" {
		t.Errorf("WorkProgressed.TicketID = %s, want TKT-001", found.TicketID)
	}
	if found.EffortApplied <= 0 {
		t.Errorf("WorkProgressed.EffortApplied = %v, want > 0", found.EffortApplied)
	}
}

func TestEngine_EmitsTicketPhaseChangedEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()

	// Setup a ticket that will complete current phase (but not entire ticket) - BEFORE creating engine
	ticket := model.NewTicket("TKT-001", "Test Ticket", 5, model.HighUnderstanding)
	ticket.AssignedTo = "DEV-001"
	ticket.Phase = model.PhaseResearch
	ticket.RemainingEffort = 0.1 // Will complete this phase in one tick
	sim.ActiveTickets = append(sim.ActiveTickets, ticket)
	sim.Developers[0] = sim.Developers[0].WithTicket("TKT-001")

	eng := NewEngineWithStore(sim.Seed, store)
	eng.EmitLoadedState(*sim) // Sync all state including ActiveTickets to projection

	eng.Tick()

	// Find TicketPhaseChanged event
	evts := store.Replay(sim.ID)
	var found *events.TicketPhaseChanged
	for _, e := range evts {
		if e.EventType() == "TicketPhaseChanged" {
			pc := e.(events.TicketPhaseChanged)
			found = &pc
			break
		}
	}

	if found == nil {
		t.Fatal("Expected TicketPhaseChanged event not found")
	}

	if found.TicketID != "TKT-001" {
		t.Errorf("TicketPhaseChanged.TicketID = %s, want TKT-001", found.TicketID)
	}
	if found.OldPhase != model.PhaseResearch {
		t.Errorf("TicketPhaseChanged.OldPhase = %v, want PhaseResearch", found.OldPhase)
	}
	if found.NewPhase != model.PhaseSizing {
		t.Errorf("TicketPhaseChanged.NewPhase = %v, want PhaseSizing", found.NewPhase)
	}
}

func TestEngine_EmitsSprintEndedEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim.Seed, store)
	emitCreatedFromSim(eng, sim)

	// Start sprint
	eng.StartSprint()

	// Get sprint info from projection (not sim directly)
	sprint, _ := eng.Sim().CurrentSprintOption.Get()

	// Advance to sprint end using projection state
	for eng.Sim().CurrentTick < sprint.EndDay {
		eng.Tick()
	}

	// Find SprintEnded event
	evts := store.Replay(eng.Sim().ID)
	var found *events.SprintEnded
	for _, e := range evts {
		if e.EventType() == "SprintEnded" {
			se := e.(events.SprintEnded)
			found = &se
			break
		}
	}

	if found == nil {
		t.Fatal("Expected SprintEnded event not found")
	}

	if found.Number != sprint.Number {
		t.Errorf("SprintEnded.Number = %d, want %d", found.Number, sprint.Number)
	}
}

func TestEngine_SetPolicyEmitsEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := model.NewSimulation(model.PolicyNone, 42)
	sim.ID = "test-sim"
	eng := NewEngineWithStore(sim.Seed, store)
	emitCreatedFromSim(eng, sim)

	// Change policy
	eng.SetPolicy(model.PolicyDORAStrict)

	// Find PolicyChanged event
	evts := store.Replay(sim.ID)
	var found *events.PolicyChanged
	for _, e := range evts {
		if e.EventType() == "PolicyChanged" {
			pc := e.(events.PolicyChanged)
			found = &pc
			break
		}
	}

	if found == nil {
		t.Fatal("Expected PolicyChanged event not found")
	}

	if found.OldPolicy != model.PolicyNone {
		t.Errorf("PolicyChanged.OldPolicy = %v, want PolicyNone", found.OldPolicy)
	}
	if found.NewPolicy != model.PolicyDORAStrict {
		t.Errorf("PolicyChanged.NewPolicy = %v, want PolicyDORAStrict", found.NewPolicy)
	}

	// Verify projection has the new policy
	state := eng.Sim()
	if state.SizingPolicy != model.PolicyDORAStrict {
		t.Errorf("Sim().SizingPolicy = %v, want PolicyDORAStrict", state.SizingPolicy)
	}
}

func TestEngine_ToUIEvent(t *testing.T) {
	store := events.NewMemoryStore()
	sim := createTestSimulation()
	eng := NewEngineWithStore(sim.Seed, store)

	tests := []struct {
		name        string
		event       events.Event
		wantType    model.EventType
		wantMessage string
		wantDay     int
	}{
		{
			name:        "BugDiscovered converts correctly",
			event:       events.NewBugDiscovered("sim-1", 5, "TKT-001", 0.5),
			wantType:    model.EventBugDiscovered,
			wantMessage: "Bug discovered in TKT-001 (+0.5 days)",
			wantDay:     5,
		},
		{
			name:        "ScopeCreepOccurred converts correctly",
			event:       events.NewScopeCreepOccurred("sim-1", 10, "TKT-002", 1.5, 1.5),
			wantType:    model.EventScopeCreep,
			wantMessage: "Scope creep on TKT-002 (+1.5 days)",
			wantDay:     10,
		},
		{
			name:        "IncidentStarted converts correctly",
			event:       events.NewIncidentStarted("sim-1", 15, "INC-001", "DEV-001", "TKT-003", model.SeverityHigh),
			wantType:    model.EventIncident,
			wantMessage: "Incident INC-001: TKT-003 caused production issue",
			wantDay:     15,
		},
		{
			name:        "IncidentResolved converts correctly",
			event:       events.NewIncidentResolved("sim-1", 20, "INC-001", "DEV-001"),
			wantType:    model.EventIncidentResolved,
			wantMessage: "Incident INC-001 resolved",
			wantDay:     20,
		},
		{
			name:        "unknown event returns empty",
			event:       events.NewTicked("sim-1", 25), // Not a generator event
			wantType:    0,                             // Zero value
			wantMessage: "",
			wantDay:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eng.toUIEvent(tt.event)

			if got.Type != tt.wantType {
				t.Errorf("toUIEvent().Type = %v, want %v", got.Type, tt.wantType)
			}
			if got.Message != tt.wantMessage {
				t.Errorf("toUIEvent().Message = %q, want %q", got.Message, tt.wantMessage)
			}
			if got.Day != tt.wantDay {
				t.Errorf("toUIEvent().Day = %d, want %d", got.Day, tt.wantDay)
			}
		})
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

// emitCreatedFromSim emits SimulationCreated event from sim state.
func emitCreatedFromSim(eng *Engine, sim *model.Simulation) {
	eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     len(sim.Developers),
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
	})
}

// TestEngine_DetectsConcurrencyConflict verifies Engine panics when store returns conflict error.
// Per Go Development Guide §6: Uses test double at external boundary (store interface).
func TestEngine_DetectsConcurrencyConflict(t *testing.T) {
	store := &storeWithConflict{
		MemoryStore:    events.NewMemoryStore(),
		conflictOnCall: 2, // Second append will fail
	}
	eng := NewEngineWithStore(42, store)

	// First emit succeeds
	eng.EmitCreated("sim-1", 0, events.SimConfig{})

	// Second emit should detect conflict and panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on concurrency conflict, got none")
		}
	}()

	eng.AddDeveloper("dev-1", "Alice", 1.0)
}

// TestEngine_ConcurrencyConflictPanicMessage verifies panic includes useful debugging info.
func TestEngine_ConcurrencyConflictPanicMessage(t *testing.T) {
	store := &storeWithConflict{
		MemoryStore:    events.NewMemoryStore(),
		conflictOnCall: 1, // First append will fail
	}
	eng := NewEngineWithStore(42, store)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic, got none")
		}

		panicMsg := fmt.Sprintf("%v", r)
		if !containsSubstr(panicMsg, "concurrency conflict") {
			t.Errorf("Panic message should mention 'concurrency conflict', got: %s", panicMsg)
		}
	}()

	eng.EmitCreated("sim-1", 0, events.SimConfig{})
}

// containsSubstr checks if s contains substr.
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// storeWithConflict is a test double that wraps MemoryStore to simulate concurrency conflicts.
//
// This is acceptable per Go Development Guide §6 (Mocks = Design Smell) because:
//   - It's at an external boundary (the Store interface)
//   - It tests Engine's behavior when store fails, not internal implementation details
//   - It simulates a real scenario: another process appending before us
//
// Usage:
//
//	store := &storeWithConflict{
//	    MemoryStore:    events.NewMemoryStore(),
//	    conflictOnCall: 2,  // Fail on second Append call
//	}
type storeWithConflict struct {
	*events.MemoryStore
	conflictOnCall int // Which call number should return an error (1-indexed)
	callCount      int
}

// Append delegates to MemoryStore unless this is the conflictOnCall-th invocation.
func (s *storeWithConflict) Append(simID string, version int, evts ...events.Event) error {
	s.callCount++
	if s.callCount == s.conflictOnCall {
		return fmt.Errorf("concurrency conflict: simulated")
	}
	return s.MemoryStore.Append(simID, version, evts...)
}

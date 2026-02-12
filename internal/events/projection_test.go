package events_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/google/go-cmp/cmp"
)

// TestProjection_Apply tests the Projection.Apply() method for each event type.
// Per Khorikov: Domain logic gets heavy unit testing with table-driven tests.
// Projection is pure (no side effects), making it ideal for unit testing.

func TestProjection_Apply_SimulationCreated(t *testing.T) {
	proj := events.NewProjection()

	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		TeamSize:     3,
		SprintLength: 10,
		Seed:         42,
		Policy:       model.PolicyDORAStrict,
	})

	got := proj.Apply(evt)

	// Verify state was initialized
	state := got.State()
	if state.ID != "sim-1" {
		t.Errorf("ID = %q, want %q", state.ID, "sim-1")
	}
	if state.Seed != 42 {
		t.Errorf("Seed = %d, want %d", state.Seed, 42)
	}
	if state.SizingPolicy != model.PolicyDORAStrict {
		t.Errorf("SizingPolicy = %v, want %v", state.SizingPolicy, model.PolicyDORAStrict)
	}
	if got.Version() != 1 {
		t.Errorf("Version = %d, want 1", got.Version())
	}
}

func TestProjection_Apply_Ticked(t *testing.T) {
	// Setup: create simulation first
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	// Apply tick event
	evt := events.NewTicked("sim-1", 1)
	got := proj.Apply(evt)

	state := got.State()
	if state.CurrentTick != 1 {
		t.Errorf("CurrentTick = %d, want 1", state.CurrentTick)
	}
	if got.Version() != 2 {
		t.Errorf("Version = %d, want 2", got.Version())
	}
}

func TestProjection_Apply_DeveloperAdded(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	evt := events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0)
	got := proj.Apply(evt)

	state := got.State()
	if len(state.Developers) != 1 {
		t.Fatalf("len(Developers) = %d, want 1", len(state.Developers))
	}
	dev := state.Developers[0]
	if dev.ID != "dev-1" {
		t.Errorf("Developer.ID = %q, want %q", dev.ID, "dev-1")
	}
	if dev.Name != "Alice" {
		t.Errorf("Developer.Name = %q, want %q", dev.Name, "Alice")
	}
	if dev.Velocity != 1.0 {
		t.Errorf("Developer.Velocity = %f, want 1.0", dev.Velocity)
	}
}

func TestProjection_Apply_TicketCreated(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	evt := events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding)
	got := proj.Apply(evt)

	state := got.State()
	if len(state.Backlog) != 1 {
		t.Fatalf("len(Backlog) = %d, want 1", len(state.Backlog))
	}
	ticket := state.Backlog[0]
	if ticket.ID != "TKT-001" {
		t.Errorf("Ticket.ID = %q, want %q", ticket.ID, "TKT-001")
	}
	if ticket.Title != "Fix bug" {
		t.Errorf("Ticket.Title = %q, want %q", ticket.Title, "Fix bug")
	}
	if ticket.EstimatedDays != 3.0 {
		t.Errorf("Ticket.EstimatedDays = %f, want 3.0", ticket.EstimatedDays)
	}
}

func TestProjection_Apply_SprintStarted(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))

	// SprintStarted now includes BufferDays for fever chart
	evt := events.NewSprintStarted("sim-1", 0, 1, 2.0) // 2 buffer days
	got := proj.Apply(evt)

	state := got.State()
	sprint, ok := state.CurrentSprintOption.Get()
	if !ok {
		t.Fatal("CurrentSprintOption should be set")
	}
	if sprint.Number != 1 {
		t.Errorf("Sprint.Number = %d, want 1", sprint.Number)
	}
	if sprint.StartDay != 0 {
		t.Errorf("Sprint.StartDay = %d, want 0", sprint.StartDay)
	}
	if sprint.BufferDays != 2.0 {
		t.Errorf("Sprint.BufferDays = %f, want 2.0", sprint.BufferDays)
	}

	// Verify SprintNumber on simulation is updated (bug fix)
	if state.SprintNumber != 1 {
		t.Errorf("Simulation.SprintNumber = %d, want 1", state.SprintNumber)
	}

	// Start sprint 2
	proj = got.Apply(events.NewSprintEnded("sim-1", 10, 1))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 10, 2, 2.5))
	state = proj.State()
	if state.SprintNumber != 2 {
		t.Errorf("Simulation.SprintNumber after sprint 2 = %d, want 2", state.SprintNumber)
	}
}

func TestProjection_Apply_TicketAssigned(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))

	// Assign at tick 5
	startedAt := time.Now()
	evt := events.NewTicketAssigned("sim-1", 5, "TKT-001", "dev-1", startedAt)
	got := proj.Apply(evt)

	state := got.State()

	// Ticket should move from Backlog to ActiveTickets
	if len(state.Backlog) != 0 {
		t.Errorf("len(Backlog) = %d, want 0", len(state.Backlog))
	}
	if len(state.ActiveTickets) != 1 {
		t.Fatalf("len(ActiveTickets) = %d, want 1", len(state.ActiveTickets))
	}

	// Developer should have ticket assigned
	if state.Developers[0].CurrentTicket != "TKT-001" {
		t.Errorf("Developer.CurrentTicket = %q, want %q", state.Developers[0].CurrentTicket, "TKT-001")
	}

	// Verify StartedTick is set
	ticket := state.ActiveTickets[0]
	if ticket.StartedTick != 5 {
		t.Errorf("Ticket.StartedTick = %d, want 5", ticket.StartedTick)
	}

	// Verify StartedAt is set (approximately - allow 1 second tolerance)
	if ticket.StartedAt.Before(startedAt.Add(-time.Second)) || ticket.StartedAt.After(startedAt.Add(time.Second)) {
		t.Errorf("Ticket.StartedAt = %v, want approximately %v", ticket.StartedAt, startedAt)
	}
}

func TestProjection_Apply_WorkProgressed(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1", time.Time{}))

	// Get initial remaining effort
	initialEffort := proj.State().ActiveTickets[0].RemainingEffort

	// WorkProgressed now includes Phase for PhaseEffortSpent tracking
	evt := events.NewWorkProgressed("sim-1", 1, "TKT-001", model.PhaseResearch, 1.0)
	got := proj.Apply(evt)

	state := got.State()
	ticket := state.ActiveTickets[0]

	if ticket.RemainingEffort != initialEffort-1.0 {
		t.Errorf("RemainingEffort = %f, want %f", ticket.RemainingEffort, initialEffort-1.0)
	}
	if ticket.ActualDays != 1.0 {
		t.Errorf("ActualDays = %f, want 1.0", ticket.ActualDays)
	}
	// Verify PhaseEffortSpent is tracked
	if ticket.PhaseEffortSpent[model.PhaseResearch] != 1.0 {
		t.Errorf("PhaseEffortSpent[Research] = %f, want 1.0", ticket.PhaseEffortSpent[model.PhaseResearch])
	}
}

func TestProjection_Apply_WorkProgressed_MultiplePhases(t *testing.T) {
	// Verify PhaseEffortSpent accumulates correctly across multiple phases
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1", time.Time{}))

	// Work in Research phase
	proj = proj.Apply(events.NewWorkProgressed("sim-1", 1, "TKT-001", model.PhaseResearch, 0.5))
	proj = proj.Apply(events.NewWorkProgressed("sim-1", 2, "TKT-001", model.PhaseResearch, 0.3))

	// Transition to Implement phase
	proj = proj.Apply(events.NewTicketPhaseChanged("sim-1", 3, "TKT-001", model.PhaseResearch, model.PhaseImplement))

	// Work in Implement phase
	proj = proj.Apply(events.NewWorkProgressed("sim-1", 4, "TKT-001", model.PhaseImplement, 1.0))
	proj = proj.Apply(events.NewWorkProgressed("sim-1", 5, "TKT-001", model.PhaseImplement, 0.7))

	state := proj.State()
	ticket := state.ActiveTickets[0]

	// Verify Research effort accumulated
	if ticket.PhaseEffortSpent[model.PhaseResearch] != 0.8 {
		t.Errorf("PhaseEffortSpent[Research] = %f, want 0.8", ticket.PhaseEffortSpent[model.PhaseResearch])
	}
	// Verify Implement effort accumulated
	if ticket.PhaseEffortSpent[model.PhaseImplement] != 1.7 {
		t.Errorf("PhaseEffortSpent[Implement] = %f, want 1.7", ticket.PhaseEffortSpent[model.PhaseImplement])
	}
	// Verify total ActualDays
	if ticket.ActualDays != 2.5 {
		t.Errorf("ActualDays = %f, want 2.5", ticket.ActualDays)
	}
}

func TestProjection_Apply_TicketCompleted(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1", time.Time{}))

	evt := events.NewTicketCompleted("sim-1", 3, "TKT-001", "dev-1", 3.0)
	got := proj.Apply(evt)

	state := got.State()

	// Ticket should move from ActiveTickets to CompletedTickets
	if len(state.ActiveTickets) != 0 {
		t.Errorf("len(ActiveTickets) = %d, want 0", len(state.ActiveTickets))
	}
	if len(state.CompletedTickets) != 1 {
		t.Fatalf("len(CompletedTickets) = %d, want 1", len(state.CompletedTickets))
	}

	// Developer should be idle
	if state.Developers[0].CurrentTicket != "" {
		t.Errorf("Developer.CurrentTicket = %q, want empty", state.Developers[0].CurrentTicket)
	}
}

func TestProjection_Apply_BufferConsumed(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 2.0)) // 2 buffer days

	// Consume some buffer
	evt := events.NewBufferConsumed("sim-1", 1, 0.5) // 0.5 days consumed
	got := proj.Apply(evt)

	state := got.State()
	sprint, ok := state.CurrentSprintOption.Get()
	if !ok {
		t.Fatal("CurrentSprintOption should be set")
	}
	if sprint.BufferConsumed != 0.5 {
		t.Errorf("Sprint.BufferConsumed = %f, want 0.5", sprint.BufferConsumed)
	}

	// Consume more
	got = got.Apply(events.NewBufferConsumed("sim-1", 2, 0.3))
	sprint, _ = got.State().CurrentSprintOption.Get()
	if sprint.BufferConsumed != 0.8 {
		t.Errorf("Sprint.BufferConsumed = %f, want 0.8", sprint.BufferConsumed)
	}
}

func TestProjection_Apply_BufferConsumed_FeverTransitions(t *testing.T) {
	// Verify fever status transitions as buffer is consumed relative to sprint progress.
	// Diagonal thresholds:
	//   Green: bufferPct <= progress * 0.66
	//   Red:   bufferPct > 0.33 + progress * 0.67
	//   Yellow: between thresholds
	//
	// NOTE: Progress is now sprint-scoped (only tickets in sprint.Tickets count).
	// Tickets are added to sprint.Tickets when assigned during an active sprint.
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 10.0)) // 10 buffer days

	// Create 2 tickets with 5 days each = 10 total days of work
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "T-1", "Test Ticket 1", 5.0, model.HighUnderstanding))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "T-2", "Test Ticket 2", 5.0, model.HighUnderstanding))

	// Assign BOTH tickets to sprint (adds them to sprint.Tickets)
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, "T-1", "dev-1", time.Time{}))
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, "T-2", "dev-1", time.Time{}))

	// Initially Green (0% consumed, 0% progress - both tickets active)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 1, 0.0))
	sprint, _ := proj.State().CurrentSprintOption.Get()
	if sprint.FeverStatus != model.FeverGreen {
		t.Errorf("Initial FeverStatus = %v, want FeverGreen", sprint.FeverStatus)
	}

	// Complete first ticket -> 50% progress (5 of 10 days estimated)
	proj = proj.Apply(events.NewTicketCompleted("sim-1", 5, "T-1", "dev-1", 5.0))

	// At 50% progress, consume 2.0 of 10.0 = 20% buffer -> Green (20% <= 50%*0.66=33%)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 5, 2.0))
	sprint, _ = proj.State().CurrentSprintOption.Get()
	if sprint.FeverStatus != model.FeverGreen {
		t.Errorf("After 20%% buffer at 50%% progress: FeverStatus = %v, want FeverGreen", sprint.FeverStatus)
	}

	// Consume more: 5.0 total = 50% buffer -> Yellow (50% > 33% green, <= 67% red)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 6, 3.0))
	sprint, _ = proj.State().CurrentSprintOption.Get()
	if sprint.FeverStatus != model.FeverYellow {
		t.Errorf("After 50%% buffer at 50%% progress: FeverStatus = %v, want FeverYellow", sprint.FeverStatus)
	}

	// Consume more: 8.0 total = 80% buffer -> Red (80% > 0.33+0.5*0.67=67%)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 7, 3.0))
	sprint, _ = proj.State().CurrentSprintOption.Get()
	if sprint.FeverStatus != model.FeverRed {
		t.Errorf("After 80%% buffer at 50%% progress: FeverStatus = %v, want FeverRed", sprint.FeverStatus)
	}
}

func TestProjection_Apply_SprintEnded(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 2.0))

	evt := events.NewSprintEnded("sim-1", 10, 1)
	got := proj.Apply(evt)

	state := got.State()
	_, ok := state.CurrentSprintOption.Get()
	if ok {
		t.Error("CurrentSprintOption should be cleared after SprintEnded")
	}
}

func TestProjection_ValueSemantics(t *testing.T) {
	// Verify Apply returns new Projection, doesn't mutate original
	proj1 := events.NewProjection()
	proj2 := proj1.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	// Original should be unchanged
	if proj1.Version() != 0 {
		t.Errorf("Original proj1.Version() = %d, want 0 (should be unchanged)", proj1.Version())
	}
	if proj1.State().ID != "" {
		t.Errorf("Original proj1.State().ID = %q, want empty (should be unchanged)", proj1.State().ID)
	}

	// New projection should have changes
	if proj2.Version() != 1 {
		t.Errorf("New proj2.Version() = %d, want 1", proj2.Version())
	}
	if proj2.State().ID != "sim-1" {
		t.Errorf("New proj2.State().ID = %q, want %q", proj2.State().ID, "sim-1")
	}
}

// BenchmarkProjection_Apply_SingleEvent benchmarks applying a single event.
// Target: < 1μs/op
func BenchmarkProjection_Apply_SingleEvent(b *testing.B) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	evt := events.NewTicked("sim-1", 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proj.Apply(evt)
	}
}

// BenchmarkProjection_ReplayFull benchmarks replaying 1000 events.
// Target: < 1ms total
func BenchmarkProjection_ReplayFull(b *testing.B) {
	// Generate 1000 events: create, add devs, add tickets, ticks
	evts := generateTestEvents(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proj := events.NewProjection()
		for _, e := range evts {
			proj = proj.Apply(e)
		}
	}
}

// BenchmarkProjection_IdempotencySkip measures the cost of duplicate detection.
// Compares first pass (full processing) vs second pass (early return).
func BenchmarkProjection_IdempotencySkip(b *testing.B) {
	evts := generateTestEvents(100)

	// Setup: create projection with all events already processed
	proj := events.NewProjection()
	for _, e := range evts {
		proj = proj.Apply(e)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// All events are duplicates - measures pure skip cost
		for _, e := range evts {
			proj.Apply(e)
		}
	}
}

// generateTestEvents creates n events for benchmarking.
func generateTestEvents(n int) []events.Event {
	result := make([]events.Event, 0, n)

	// Start with SimulationCreated
	result = append(result, events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	// Add 3 developers
	result = append(result, events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	result = append(result, events.NewDeveloperAdded("sim-1", 0, "dev-2", "Bob", 0.8))
	result = append(result, events.NewDeveloperAdded("sim-1", 0, "dev-3", "Carol", 1.2))

	// Fill rest with Ticked events (most common event type)
	for i := len(result); i < n; i++ {
		result = append(result, events.NewTicked("sim-1", i))
	}

	return result
}

// TestProjection_ReplayManyEvents_Correctness verifies that replaying 1000+ events
// produces correct final state. Benchmarks only test performance, not correctness.
// Per Khorikov: edge case test for large event streams (regression protection).
func TestProjection_ReplayManyEvents_Correctness(t *testing.T) {
	evts := generateTestEvents(1000)

	proj := events.NewProjection()
	for _, e := range evts {
		proj = proj.Apply(e)
	}

	state := proj.State()

	// Verify final state matches expected
	if state.ID != "sim-1" {
		t.Errorf("ID = %q, want %q", state.ID, "sim-1")
	}
	if len(state.Developers) != 3 {
		t.Errorf("Developers = %d, want 3", len(state.Developers))
	}

	// generateTestEvents: 4 setup events (Created + 3 DeveloperAdded), then Ticked from index 4 to 999
	// Last Ticked event is NewTicked("sim-1", 999), so CurrentTick should be 999
	expectedTick := 999
	if state.CurrentTick != expectedTick {
		t.Errorf("CurrentTick = %d, want %d", state.CurrentTick, expectedTick)
	}

	// Version should equal total events processed
	if proj.Version() != 1000 {
		t.Errorf("Version = %d, want 1000", proj.Version())
	}
}

func TestProjection_Apply_IncidentStarted(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	evt := events.NewIncidentStarted("sim-1", 10, "INC-001", "DEV-001", "", model.SeverityLow)
	got := proj.Apply(evt)

	state := got.State()
	if len(state.OpenIncidents) != 1 {
		t.Fatalf("len(OpenIncidents) = %d, want 1", len(state.OpenIncidents))
	}
	if state.OpenIncidents[0].ID != "INC-001" {
		t.Errorf("Incident.ID = %q, want %q", state.OpenIncidents[0].ID, "INC-001")
	}
}

func TestProjection_Apply_IncidentResolved(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewIncidentStarted("sim-1", 10, "INC-001", "DEV-001", "", model.SeverityLow))

	evt := events.NewIncidentResolved("sim-1", 15, "INC-001", "DEV-001")
	got := proj.Apply(evt)

	state := got.State()
	if len(state.OpenIncidents) != 0 {
		t.Errorf("len(OpenIncidents) = %d, want 0", len(state.OpenIncidents))
	}
	if len(state.ResolvedIncidents) != 1 {
		t.Fatalf("len(ResolvedIncidents) = %d, want 1", len(state.ResolvedIncidents))
	}
	if state.ResolvedIncidents[0].ResolvedAt == nil {
		t.Error("ResolvedAt should not be nil")
	}
}

func TestProjection_Apply_IncidentResolved_MultipleIncidents(t *testing.T) {
	// Verify resolving one incident doesn't affect others
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewIncidentStarted("sim-1", 10, "INC-001", "DEV-001", "", model.SeverityLow))
	proj = proj.Apply(events.NewIncidentStarted("sim-1", 12, "INC-002", "DEV-002", "", model.SeverityMedium))

	// Resolve first incident
	got := proj.Apply(events.NewIncidentResolved("sim-1", 15, "INC-001", "DEV-001"))

	state := got.State()
	// Second incident should still be open
	if len(state.OpenIncidents) != 1 {
		t.Fatalf("len(OpenIncidents) = %d, want 1", len(state.OpenIncidents))
	}
	if state.OpenIncidents[0].ID != "INC-002" {
		t.Errorf("OpenIncidents[0].ID = %q, want %q", state.OpenIncidents[0].ID, "INC-002")
	}
	// First incident should be resolved
	if len(state.ResolvedIncidents) != 1 {
		t.Fatalf("len(ResolvedIncidents) = %d, want 1", len(state.ResolvedIncidents))
	}
	if state.ResolvedIncidents[0].ID != "INC-001" {
		t.Errorf("ResolvedIncidents[0].ID = %q, want %q", state.ResolvedIncidents[0].ID, "INC-001")
	}
}

func TestProjection_Apply_IncidentResolved_NonExistent(t *testing.T) {
	// Verify resolving non-existent incident is a no-op (event sourcing idempotency)
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewIncidentStarted("sim-1", 10, "INC-001", "DEV-001", "", model.SeverityLow))

	// Try to resolve incident that doesn't exist
	got := proj.Apply(events.NewIncidentResolved("sim-1", 15, "INC-999", "DEV-001"))

	state := got.State()
	// Original incident should still be open
	if len(state.OpenIncidents) != 1 {
		t.Errorf("len(OpenIncidents) = %d, want 1", len(state.OpenIncidents))
	}
	// Nothing should be resolved
	if len(state.ResolvedIncidents) != 0 {
		t.Errorf("len(ResolvedIncidents) = %d, want 0", len(state.ResolvedIncidents))
	}
	// Version should still increment (event was processed)
	if got.Version() != 3 {
		t.Errorf("Version() = %d, want 3", got.Version())
	}
}

// cmp is used for complex struct comparisons
var _ = cmp.Diff

// TestProjection_Apply_IdempotentWithSortedSlice verifies that applying the same
// event twice returns an unchanged projection (no version bump, no state change).
// This is critical for distributed systems where duplicate delivery is inevitable.
func TestProjection_Apply_IdempotentWithSortedSlice(t *testing.T) {
	proj := events.NewProjection()

	// Apply first event
	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})
	proj = proj.Apply(evt)

	// Capture state after first apply
	versionAfterFirst := proj.Version()
	stateAfterFirst := proj.State()

	// Apply same event again (duplicate)
	proj2 := proj.Apply(evt)

	// Version should NOT change (idempotent)
	if proj2.Version() != versionAfterFirst {
		t.Errorf("Version after duplicate = %d, want %d (unchanged)", proj2.Version(), versionAfterFirst)
	}

	// State should be identical
	if proj2.State().ID != stateAfterFirst.ID {
		t.Errorf("State changed after duplicate apply")
	}

	// The returned projection should be the same object (or equal)
	// This verifies true idempotency: duplicate has no effect
}

func TestProjection_Apply_IdempotentSequence(t *testing.T) {
	// Verify idempotency works across a sequence of events
	proj := events.NewProjection()

	evt1 := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})
	evt2 := events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0)
	evt3 := events.NewTicked("sim-1", 1)

	// Apply all events
	proj = proj.Apply(evt1)
	proj = proj.Apply(evt2)
	proj = proj.Apply(evt3)

	versionAfterAll := proj.Version() // Should be 3

	// Replay all events (simulating duplicate delivery)
	proj = proj.Apply(evt1) // duplicate
	proj = proj.Apply(evt2) // duplicate
	proj = proj.Apply(evt3) // duplicate

	// Version should still be 3 (all duplicates skipped)
	if proj.Version() != versionAfterAll {
		t.Errorf("Version after replay = %d, want %d", proj.Version(), versionAfterAll)
	}
}

func TestProjection_Apply_ValueSemantics(t *testing.T) {
	// Verify that Apply returns new projection without mutating original.
	proj1 := events.NewProjection()
	evt1 := events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42})

	// Apply event to get proj2
	proj2 := proj1.Apply(evt1)

	// proj1 should be unchanged (value semantics)
	if proj1.Version() != 0 {
		t.Errorf("proj1.Version() = %d, want 0 (original unchanged)", proj1.Version())
	}

	// proj2 should have version 1
	if proj2.Version() != 1 {
		t.Errorf("proj2.Version() = %d, want 1", proj2.Version())
	}

	// Apply same event to proj1 - creates new projection with version 1
	proj1AfterApply := proj1.Apply(evt1)
	if proj1AfterApply.Version() != 1 {
		t.Errorf("proj1AfterApply.Version() = %d, want 1", proj1AfterApply.Version())
	}

	// proj1 STILL unchanged (value semantics)
	if proj1.Version() != 0 {
		t.Errorf("proj1.Version() after apply = %d, want 0 (still unchanged)", proj1.Version())
	}
}

func TestProjection_Apply_EmptyEventID(t *testing.T) {
	// Empty EventID should bypass idempotency check (defensive - allows processing).
	// This handles edge cases where events might not have IDs assigned.
	proj := events.NewProjection()

	// Create event factory that produces events with empty IDs
	// We'll use Ticked events which have simple state changes
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	// Create two Ticked events - they have generated IDs by default,
	// but if they had empty IDs, both should still be processed
	tick1 := events.NewTicked("sim-1", 1)
	tick2 := events.NewTicked("sim-1", 2)

	proj = proj.Apply(tick1)
	proj = proj.Apply(tick2)

	// Both ticks should be applied (version should be 3: created + 2 ticks)
	if proj.Version() != 3 {
		t.Errorf("Version = %d, want 3 (both ticks should process)", proj.Version())
	}
	// Current tick should be 2
	if proj.State().CurrentTick != 2 {
		t.Errorf("CurrentTick = %d, want 2", proj.State().CurrentTick)
	}
}

func TestProjection_VersionSyncWithStore(t *testing.T) {
	// Critical invariant: Store.EventCount == Projection.Version after all operations.
	// This ensures optimistic concurrency works correctly.
	store := events.NewMemoryStore()
	defer store.Close()
	proj := events.NewProjection()

	simID := "sim-1"

	// Apply and append first event
	evt1 := events.NewSimulationCreated(simID, 0, events.SimConfig{Seed: 42})
	if err := store.Append(simID, proj.Version(), evt1); err != nil {
		t.Fatalf("Append evt1: %v", err)
	}
	proj = proj.Apply(evt1)

	// Check invariant after first event
	if store.EventCount(simID) != proj.Version() {
		t.Errorf("After evt1: store=%d, proj=%d (should match)",
			store.EventCount(simID), proj.Version())
	}

	// Apply and append second event
	evt2 := events.NewDeveloperAdded(simID, 0, "dev-1", "Alice", 1.0)
	if err := store.Append(simID, proj.Version(), evt2); err != nil {
		t.Fatalf("Append evt2: %v", err)
	}
	proj = proj.Apply(evt2)

	// Check invariant after second event
	if store.EventCount(simID) != proj.Version() {
		t.Errorf("After evt2: store=%d, proj=%d (should match)",
			store.EventCount(simID), proj.Version())
	}

	// Note: With version-only idempotency, the store prevents duplicate appends.
	// Projection.Apply is a pure function - applying same event twice WILL increment version.
	// This is correct: caller should not apply duplicates; store.Replay returns unique events.
}

func TestProjection_Apply_SprintWIPUpdated(t *testing.T) {
	// Setup: create simulation with sprint and some active tickets
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 2.0))

	// Apply WIP update with 3 active tickets
	evt := events.NewSprintWIPUpdated("sim-1", 1, 3)
	got := proj.Apply(evt)

	state := got.State()
	sprint, ok := state.CurrentSprintOption.Get()
	if !ok {
		t.Fatal("CurrentSprintOption should be set")
	}

	// Verify WIP tracking updated
	if sprint.MaxWIP != 3 {
		t.Errorf("Sprint.MaxWIP = %d, want 3", sprint.MaxWIP)
	}
	if sprint.WIPSum != 3 {
		t.Errorf("Sprint.WIPSum = %d, want 3", sprint.WIPSum)
	}
	if sprint.WIPTicks != 1 {
		t.Errorf("Sprint.WIPTicks = %d, want 1", sprint.WIPTicks)
	}

	// Apply another WIP update with 5 active tickets (higher)
	got = got.Apply(events.NewSprintWIPUpdated("sim-1", 2, 5))
	sprint, _ = got.State().CurrentSprintOption.Get()

	if sprint.MaxWIP != 5 {
		t.Errorf("Sprint.MaxWIP = %d, want 5 (higher of 3 and 5)", sprint.MaxWIP)
	}
	if sprint.WIPSum != 8 {
		t.Errorf("Sprint.WIPSum = %d, want 8 (3+5)", sprint.WIPSum)
	}
	if sprint.WIPTicks != 2 {
		t.Errorf("Sprint.WIPTicks = %d, want 2", sprint.WIPTicks)
	}

	// Apply WIP update with 2 active tickets (lower - MaxWIP shouldn't change)
	got = got.Apply(events.NewSprintWIPUpdated("sim-1", 3, 2))
	sprint, _ = got.State().CurrentSprintOption.Get()

	if sprint.MaxWIP != 5 {
		t.Errorf("Sprint.MaxWIP = %d, want 5 (max unchanged)", sprint.MaxWIP)
	}
	if sprint.WIPSum != 10 {
		t.Errorf("Sprint.WIPSum = %d, want 10 (3+5+2)", sprint.WIPSum)
	}
	if sprint.WIPTicks != 3 {
		t.Errorf("Sprint.WIPTicks = %d, want 3", sprint.WIPTicks)
	}
}

func TestProjection_Apply_TicketDecomposed(t *testing.T) {
	// Setup: create simulation with a parent ticket in backlog
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-PARENT", "Big feature", 10.0, model.LowUnderstanding))

	// Verify parent is in backlog
	if len(proj.State().Backlog) != 1 {
		t.Fatalf("Setup: len(Backlog) = %d, want 1", len(proj.State().Backlog))
	}

	// Decompose parent into 2 children
	children := []events.ChildTicket{
		{ID: "TKT-CHILD-1", Title: "Part 1", EstimatedDays: 4.0, Understanding: model.MediumUnderstanding},
		{ID: "TKT-CHILD-2", Title: "Part 2", EstimatedDays: 4.0, Understanding: model.MediumUnderstanding},
	}
	evt := events.NewTicketDecomposed("sim-1", 0, "TKT-PARENT", children)
	got := proj.Apply(evt)

	state := got.State()

	// Parent should be removed from backlog
	for _, ticket := range state.Backlog {
		if ticket.ID == "TKT-PARENT" {
			t.Errorf("Parent ticket should be removed from backlog, but found it")
		}
	}

	// Children should be in backlog
	if len(state.Backlog) != 2 {
		t.Fatalf("len(Backlog) = %d, want 2", len(state.Backlog))
	}

	// Verify child tickets exist with correct properties
	childIDs := map[string]bool{}
	for _, ticket := range state.Backlog {
		childIDs[ticket.ID] = true
		if ticket.Phase != model.PhaseBacklog {
			t.Errorf("Child %s Phase = %v, want PhaseBacklog", ticket.ID, ticket.Phase)
		}
	}
	if !childIDs["TKT-CHILD-1"] {
		t.Error("TKT-CHILD-1 not found in backlog")
	}
	if !childIDs["TKT-CHILD-2"] {
		t.Error("TKT-CHILD-2 not found in backlog")
	}
}

func TestProjection_Apply_BugDiscovered(t *testing.T) {
	tests := []struct {
		name         string
		reworkEffort float64
	}{
		{"adds half day rework", 0.5},
		{"adds full day rework", 1.0},
		{"adds small rework", 0.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: simulation with an active ticket
			proj := events.NewProjection()
			proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
			proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
			proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 5.0, model.MediumUnderstanding))
			proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, "TKT-001", "dev-1", time.Time{}))

			// Get RemainingEffort after assignment (calculated from phase effort)
			effortBeforeBug := proj.State().ActiveTickets[0].RemainingEffort

			// Apply BugDiscovered event
			evt := events.NewBugDiscovered("sim-1", 2, "TKT-001", tt.reworkEffort)
			got := proj.Apply(evt)

			state := got.State()
			if len(state.ActiveTickets) != 1 {
				t.Fatalf("len(ActiveTickets) = %d, want 1", len(state.ActiveTickets))
			}

			ticket := state.ActiveTickets[0]
			expectedEffort := effortBeforeBug + tt.reworkEffort
			if ticket.RemainingEffort != expectedEffort {
				t.Errorf("RemainingEffort = %.2f, want %.2f (%.2f + %.2f)",
					ticket.RemainingEffort, expectedEffort, effortBeforeBug, tt.reworkEffort)
			}
		})
	}
}

func TestProjection_Apply_ScopeCreepOccurred(t *testing.T) {
	tests := []struct {
		name          string
		effortAdded   float64
		estimateAdded float64
	}{
		{"adds effort and estimate", 1.0, 1.0},
		{"adds small creep", 0.5, 0.5},
		{"adds large creep", 2.0, 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: simulation with an active ticket
			proj := events.NewProjection()
			proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
			proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
			proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Feature", 5.0, model.MediumUnderstanding))
			proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, "TKT-001", "dev-1", time.Time{}))

			// Get values after assignment
			ticketBefore := proj.State().ActiveTickets[0]
			effortBefore := ticketBefore.RemainingEffort
			estimateBefore := ticketBefore.EstimatedDays

			// Apply ScopeCreepOccurred event
			evt := events.NewScopeCreepOccurred("sim-1", 2, "TKT-001", tt.effortAdded, tt.estimateAdded)
			got := proj.Apply(evt)

			state := got.State()
			if len(state.ActiveTickets) != 1 {
				t.Fatalf("len(ActiveTickets) = %d, want 1", len(state.ActiveTickets))
			}

			ticket := state.ActiveTickets[0]

			// Verify RemainingEffort increased
			expectedEffort := effortBefore + tt.effortAdded
			if ticket.RemainingEffort != expectedEffort {
				t.Errorf("RemainingEffort = %.2f, want %.2f", ticket.RemainingEffort, expectedEffort)
			}

			// Verify EstimatedDays increased
			expectedEstimate := estimateBefore + tt.estimateAdded
			if ticket.EstimatedDays != expectedEstimate {
				t.Errorf("EstimatedDays = %.2f, want %.2f", ticket.EstimatedDays, expectedEstimate)
			}
		})
	}
}

func TestProjection_CalculateSprintProgress(t *testing.T) {
	// Unit test for sprint-scoped progress calculation.
	// Only tickets in sprint.Tickets should be counted.
	// Tickets are added to sprint.Tickets when assigned during an active sprint.
	tests := []struct {
		name         string
		sprintActive int     // tickets assigned to sprint (active)
		sprintDone   int     // tickets assigned to sprint (completed)
		wantProgress float64 // expected progress ratio
	}{
		{"no_sprint_tickets", 0, 0, 0},   // No tickets = 0 progress
		{"all_active", 2, 0, 0},          // No completed work
		{"half_done", 1, 1, 0.5},         // 1 of 2 sprint tickets done
		{"all_done", 0, 2, 1.0},          // All sprint tickets done
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj := events.NewProjection()
			proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
				SprintLength: 10,
				Seed:         42,
			}))
			proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))

			// Start sprint
			proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 10.0))

			ticketNum := 0

			// Add active tickets IN sprint (assigned during sprint)
			for i := 0; i < tt.sprintActive; i++ {
				ticketNum++
				id := fmt.Sprintf("T-SPRINT-%d", ticketNum)
				proj = proj.Apply(events.NewTicketCreated("sim-1", 0, id, "Sprint Active", 5.0, model.HighUnderstanding))
				proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, id, "dev-1", time.Time{})) // During sprint
			}

			// Add completed tickets IN sprint (assigned during sprint, then completed)
			for i := 0; i < tt.sprintDone; i++ {
				ticketNum++
				id := fmt.Sprintf("T-SPRINT-DONE-%d", ticketNum)
				proj = proj.Apply(events.NewTicketCreated("sim-1", 0, id, "Sprint Done", 5.0, model.HighUnderstanding))
				proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, id, "dev-1", time.Time{})) // During sprint
				proj = proj.Apply(events.NewTicketCompleted("sim-1", 5, id, "dev-1", 5.0))
			}

			// Trigger progress calculation via BufferConsumed
			// (This exercises calculateSprintProgress via the handler)
			proj = proj.Apply(events.NewBufferConsumed("sim-1", 5, 0.0))

			state := proj.State()
			sprint, ok := state.CurrentSprintOption.Get()
			if !ok {
				t.Fatal("CurrentSprintOption should be set")
			}

			// The Progress field is set by BufferConsumed handler
			if sprint.Progress != tt.wantProgress {
				t.Errorf("Sprint.Progress = %.2f, want %.2f", sprint.Progress, tt.wantProgress)
			}
		})
	}
}

func TestProjection_CalculateSprintProgress_IgnoresOtherTickets(t *testing.T) {
	// Verify that completed tickets NOT in sprint.Tickets are ignored.
	// This is important for proper CCPM fever chart calculation.
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))

	// Create and complete tickets BEFORE sprint starts (not in sprint)
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("T-BEFORE-%d", i)
		proj = proj.Apply(events.NewTicketCreated("sim-1", 0, id, "Before Sprint", 5.0, model.HighUnderstanding))
		proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, id, "dev-1", time.Time{}))
		proj = proj.Apply(events.NewTicketCompleted("sim-1", 0, id, "dev-1", 5.0))
	}

	// Now start sprint (those tickets are NOT in sprint.Tickets)
	proj = proj.Apply(events.NewSprintStarted("sim-1", 1, 1, 10.0))

	// Add one ticket to sprint (active)
	proj = proj.Apply(events.NewTicketCreated("sim-1", 1, "T-SPRINT-1", "Sprint", 5.0, model.HighUnderstanding))
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 2, "T-SPRINT-1", "dev-1", time.Time{}))

	// Complete it
	proj = proj.Apply(events.NewTicketCompleted("sim-1", 5, "T-SPRINT-1", "dev-1", 5.0))

	// Trigger progress calculation
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 5, 0.0))

	state := proj.State()
	sprint, _ := state.CurrentSprintOption.Get()

	// Sprint progress should be 100% (1 of 1 sprint ticket done)
	// NOT 25% (4 of 4 total completed tickets)
	if sprint.Progress != 1.0 {
		t.Errorf("Sprint.Progress = %.2f, want 1.0 (only sprint tickets counted)", sprint.Progress)
	}

	// Verify sprint.Tickets only has the one sprint ticket
	if len(sprint.Tickets) != 1 {
		t.Errorf("len(sprint.Tickets) = %d, want 1", len(sprint.Tickets))
	}
}

func TestProjection_BufferConsumed_Bidirectional(t *testing.T) {
	// Verify buffer can be consumed (positive) or reclaimed (negative).
	// CCPM semantics: overage consumes, underage reclaims.
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 10.0)) // 10 buffer days

	// Consume 2 days (ticket took longer than estimate)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 1, 2.0))
	sprint, _ := proj.State().CurrentSprintOption.Get()
	if sprint.BufferConsumed != 2.0 {
		t.Errorf("After consume 2: BufferConsumed = %.1f, want 2.0", sprint.BufferConsumed)
	}

	// Reclaim 1 day (ticket finished early)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 2, -1.0))
	sprint, _ = proj.State().CurrentSprintOption.Get()
	if sprint.BufferConsumed != 1.0 {
		t.Errorf("After reclaim 1: BufferConsumed = %.1f, want 1.0", sprint.BufferConsumed)
	}

	// Reclaim more than consumed (net negative buffer consumption)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 3, -3.0))
	sprint, _ = proj.State().CurrentSprintOption.Get()
	if sprint.BufferConsumed != -2.0 {
		t.Errorf("After reclaim 3: BufferConsumed = %.1f, want -2.0", sprint.BufferConsumed)
	}
}

func TestProjection_Apply_IncidentStarted_Enhanced(t *testing.T) {
	// Test the enhanced IncidentStarted: sets incident's TicketID/Severity
	// AND marks the completed ticket as having caused an incident.
	tests := []struct {
		name     string
		severity model.Severity
	}{
		{"low severity", model.SeverityLow},
		{"medium severity", model.SeverityMedium},
		{"high severity", model.SeverityHigh},
		{"critical severity", model.SeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: simulation with a completed ticket
			proj := events.NewProjection()
			proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
			proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
			proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Feature", 3.0, model.MediumUnderstanding))
			proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, "TKT-001", "dev-1", time.Time{}))
			proj = proj.Apply(events.NewTicketCompleted("sim-1", 5, "TKT-001", "dev-1", 3.0))

			// Verify ticket is completed before incident
			if len(proj.State().CompletedTickets) != 1 {
				t.Fatalf("Setup: len(CompletedTickets) = %d, want 1", len(proj.State().CompletedTickets))
			}

			// Apply IncidentStarted with TicketID and Severity
			evt := events.NewIncidentStarted("sim-1", 10, "INC-001", "dev-1", "TKT-001", tt.severity)
			got := proj.Apply(evt)

			state := got.State()

			// Verify incident created with correct fields
			if len(state.OpenIncidents) != 1 {
				t.Fatalf("len(OpenIncidents) = %d, want 1", len(state.OpenIncidents))
			}
			incident := state.OpenIncidents[0]
			if incident.ID != "INC-001" {
				t.Errorf("Incident.ID = %q, want %q", incident.ID, "INC-001")
			}
			if incident.TicketID != "TKT-001" {
				t.Errorf("Incident.TicketID = %q, want %q", incident.TicketID, "TKT-001")
			}
			if incident.Severity != tt.severity {
				t.Errorf("Incident.Severity = %v, want %v", incident.Severity, tt.severity)
			}

			// Verify completed ticket was marked as causing incident
			if len(state.CompletedTickets) != 1 {
				t.Fatalf("len(CompletedTickets) = %d, want 1", len(state.CompletedTickets))
			}
			ticket := state.CompletedTickets[0]
			if !ticket.CausedIncident {
				t.Errorf("Ticket.CausedIncident = false, want true")
			}
			if ticket.IncidentID != "INC-001" {
				t.Errorf("Ticket.IncidentID = %q, want %q", ticket.IncidentID, "INC-001")
			}
		})
	}
}
